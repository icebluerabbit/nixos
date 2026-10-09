// Package policy is the pure decision engine: Layer-1 acceptance (release fields
// only) and Layer-2 grab-gate (account state + in-flight). No I/O — decisions are a
// function of (release, account state, config) over types, so they are unit-testable
// in isolation from HTTP, MAM, and the store.
package policy

import (
	"fmt"
	"regexp"
	"strings"

	"homelab/mousetrap/internal/types"
)

// Engine evaluates acceptance and grab policy.
type Engine struct {
	accept types.AcceptConfig
	grab   types.GrabConfig
	wedge  types.WedgeConfig
	shop   types.ShopConfig

	nameInclude *regexp.Regexp // compiled AcceptConfig.NameIncludeRegex; nil = off
	nameExclude *regexp.Regexp // compiled AcceptConfig.NameExcludeRegex; nil = off
}

// New constructs the policy engine, compiling the accept name regexes (case-
// insensitive) up front so a bad pattern fails at startup, not per announce.
func New(accept types.AcceptConfig, grab types.GrabConfig, wedge types.WedgeConfig, shop types.ShopConfig) (*Engine, error) {
	e := &Engine{accept: accept, grab: grab, wedge: wedge, shop: shop}
	var err error
	if accept.NameIncludeRegex != "" {
		if e.nameInclude, err = regexp.Compile("(?i)" + accept.NameIncludeRegex); err != nil {
			return nil, fmt.Errorf("name-include regex: %w", err)
		}
	}
	if accept.NameExcludeRegex != "" {
		if e.nameExclude, err = regexp.Compile("(?i)" + accept.NameExcludeRegex); err != nil {
			return nil, fmt.Errorf("name-exclude regex: %w", err)
		}
	}
	return e, nil
}

// GrabPlan carries the grab-gate's outputs beyond the decision/reason: whether to
// spend a wedge, and whether the grab costs ratio (drives buffer accounting).
type GrabPlan struct {
	UseWedge   bool
	CostsRatio bool
}

// Accept applies Layer-1 acceptance to a release — release fields only, no account
// state. The first failing rule names the reject reason; if all pass, accept/ok.
// Each rule is inert when its config is empty/0/"". Deny lists are checked before
// allow lists so an explicit deny always wins.
func (e *Engine) Accept(rel types.Release) (types.AcceptDecision, types.AcceptReason) {
	a := e.accept

	if len(a.MediaTypes) > 0 && !containsFold(a.MediaTypes, rel.MediaType) {
		return types.Reject, types.AcceptMediaType
	}
	if anyFold(a.CategoriesDeny, rel.Categories) {
		return types.Reject, types.AcceptCategoryDeny
	}
	if len(a.CategoriesAllow) > 0 && !anyFold(a.CategoriesAllow, rel.Categories) {
		return types.Reject, types.AcceptCategoryAllow
	}
	if anyFold(a.FiletypesDeny, rel.Filetypes) {
		return types.Reject, types.AcceptFiletypeDeny
	}
	if len(a.FiletypesAllow) > 0 && !anyFold(a.FiletypesAllow, rel.Filetypes) {
		return types.Reject, types.AcceptFiletypeAllow
	}
	// MAM category names span both sides of the fiction line (Crime, Historical and
	// Paranormal each appear as fiction AND non-fiction), so this rule is the only
	// thing that separates them. Releases with no flag pass — search results carry
	// none, and rejecting them would silently break the torznab path.
	if len(a.FictionTypes) > 0 && rel.Fiction != "" && !containsFold(a.FictionTypes, rel.Fiction) {
		return types.Reject, types.AcceptFiction
	}
	if len(a.LanguagesAllow) > 0 && !languageAllowed(rel, a.LanguagesAllow) {
		return types.Reject, types.AcceptLanguage
	}
	if a.SizeMinBytes > 0 && rel.Size < a.SizeMinBytes {
		return types.Reject, types.AcceptSizeMin
	}
	if a.SizeMaxBytes > 0 && rel.Size > a.SizeMaxBytes {
		return types.Reject, types.AcceptSizeMax
	}
	if a.FreeleechOnly && !rel.Freeleech {
		return types.Reject, types.AcceptFreeleech
	}
	if a.VIPOnly && !rel.VIP {
		return types.Reject, types.AcceptVIPOnly
	}
	if a.ExcludeVIP && rel.VIP {
		return types.Reject, types.AcceptVIPExclude
	}
	if e.nameInclude != nil && !e.nameInclude.MatchString(rel.Name) {
		return types.Reject, types.AcceptNameInclude
	}
	if e.nameExclude != nil && e.nameExclude.MatchString(rel.Name) {
		return types.Reject, types.AcceptNameExclude
	}
	return types.Accept, types.AcceptOK
}

// Grab applies the Layer-2 grab-gate to a Layer-1 survivor, given the current
// account snapshot and optimistic in-flight tally. Order: no-account → unsat ceiling
// → freeleech-for-us (VIP-aware) → wedge (proactive/rescue) → wedge-required skip →
// buffer floor → grab.
func (e *Engine) Grab(rel types.Release, acct types.AccountState, inflight types.InFlight) (types.GrabDecision, types.GrabReason, GrabPlan) {
	// No snapshot yet (zero-value account) → don't act until the first refresh.
	if acct.UnsatLimit <= 0 {
		return types.Hold, types.GrabNoAccount, GrabPlan{}
	}
	// Stay a margin below the unsatisfied limit, counting optimistic in-flight grabs.
	if acct.UnsatCount+inflight.Count >= acct.UnsatLimit-e.grab.UnsatMargin {
		return types.Hold, types.GrabUnsatCeiling, GrabPlan{}
	}
	// A VIP-locked release is undownloadable unless the account is VIP — MAM answers
	// the fetch with 406. Skip it rather than burn a grab (or a wedge) on a fetch that
	// can only fail.
	if rel.VIP && !acct.VIP() {
		return types.Skip, types.GrabVIPRequired, GrabPlan{}
	}
	// Freeleech for us: a VIP release is free only if the account is VIP; otherwise
	// the raw fl flag (which autobrr sets on every VIP release) is trusted only for
	// non-VIP releases.
	free := rel.Freeleech
	if rel.VIP {
		free = acct.VIP()
	}
	if free {
		return types.Grab, types.GrabOK, GrabPlan{}
	}

	// Non-free: would this ratio-costing grab drop the buffer below the floor?
	bufferHold := e.grab.BufferMinBytes > 0 &&
		acct.Buffer()-inflight.RatioBytes-rel.Size < e.grab.BufferMinBytes

	// Wedge decision: proactive (a configured kind above a size) or rescue (a grab
	// that would otherwise be a buffer hold), gated on the wedge reserve.
	w := e.wedge
	proactive := containsFold(w.ProactiveMediaTypes, rel.MediaType) && rel.Size >= w.ProactiveMinSizeBytes
	rescue := bufferHold && w.RescueEnable && rel.Size >= w.RescueMinSizeBytes
	if w.Enable && (proactive || rescue) && acct.Wedges-inflight.Wedges > w.Reserve {
		return types.Grab, types.GrabOKWedge, GrabPlan{UseWedge: true}
	}

	// Big enough to deserve a wedge, but none spendable. Paying full ratio here
	// contradicts the rule that flagged it, so skip instead. Wedge income is
	// periodic, so this is a drought brake, not a permanent exclusion. Scoped to the
	// proactive arm: a rescue candidate still falls through to the buffer hold below,
	// and anything under the proactive size threshold grabs normally, wedges or not.
	if w.Enable && w.RequireWedge && proactive {
		return types.Skip, types.GrabWedgeUnavailable, GrabPlan{}
	}

	if bufferHold {
		return types.Hold, types.GrabBuffer, GrabPlan{}
	}
	return types.Grab, types.GrabOK, GrabPlan{CostsRatio: true}
}

// ShopPlan carries a shop decision's spend details.
type ShopPlan struct {
	Cost   int64 // bonus points to be spent
	Amount int64 // upload GiB (buy_upload only)
}

// Shop decides how to spend surplus bonus points: renew VIP (if permitted and
// affordable above the reserve), else buy an upload chunk from surplus above the
// reserve plus the VIP renewal reserve, else hold. Pure over (account, in-flight,
// config).
//
// Note what inflight is NOT used for: the VIP branch still tests acct.VIP(), which
// reads MAM's own ClassName. A purchase we believe succeeded never marks us VIP —
// that would turn every VIP-flagged release freeleech-for-us in the grab gate and
// grab the stream at full ratio cost if the buy had actually failed.
func (e *Engine) Shop(acct types.AccountState, inflight types.InFlight) (types.ShopAction, ShopPlan) {
	p := e.shop
	// Subtract spends MAM's seedbonus may not have caught up to yet.
	points := acct.SeedBonus - inflight.Points

	if p.VIPEnable && !acct.VIP() && contains(p.VIPBuyableClasses, acct.ClassName) &&
		points-p.VIPCost >= p.Reserved {
		return types.ShopBuyVIP, ShopPlan{Cost: p.VIPCost}
	}

	vipReserve := p.VIPReserveMultiple * p.VIPCost
	uploadCost := p.UploadChunkGiB * p.UploadPointsPerGiB
	if p.UploadEnable && uploadCost > 0 && points-p.Reserved-vipReserve >= uploadCost {
		return types.ShopBuyUpload, ShopPlan{Cost: uploadCost, Amount: p.UploadChunkGiB}
	}

	return types.ShopHold, ShopPlan{}
}

// contains reports whether s exactly equals any element of list.
func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// anyFold reports whether any of vals case-insensitively equals any element of list.
// This is how every multi-valued MAM field (categories, filetypes) is matched: a
// release satisfies an allowlist by carrying ANY listed value, and trips a denylist
// the same way, so deny still wins when a release carries both.
func anyFold(list, vals []string) bool {
	for _, v := range vals {
		if containsFold(list, v) {
			return true
		}
	}
	return false
}

// containsFold reports whether s case-insensitively equals any element of list.
func containsFold(list []string, s string) bool {
	for _, v := range list {
		if strings.EqualFold(v, s) {
			return true
		}
	}
	return false
}

// anySubstringFold reports whether any needle is a case-insensitive substring of hay.
func anySubstringFold(hay string, needles []string) bool {
	h := strings.ToLower(hay)
	for _, n := range needles {
		if strings.Contains(h, strings.ToLower(n)) {
			return true
		}
	}
	return false
}

// languageAllowed reports whether any of the release's (already reduced) languages
// matches the allowlist.
func languageAllowed(rel types.Release, allow []string) bool {
	for _, c := range rel.Languages {
		if c != "" && anySubstringFold(c, allow) {
			return true
		}
	}
	return false
}
