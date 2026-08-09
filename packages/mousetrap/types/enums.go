package types

// Decision, reason, and result enums returned by the policy engine and used as
// metric label values. The string values are a stable contract with the Grafana
// dashboard — do not change them.
type (
	AcceptDecision string
	AcceptReason   string
	GrabDecision   string
	GrabReason     string
	GrabResult     string
	ShopAction     string
	Kind           string // media kind derived from a release's category
	Source         string // where a grab originated
	SpendType      string // what an outgoing spend consumed
	SpendReason    string // why an outgoing spend happened
	BonusType      string // a MAM userBonusHistory row type
)

// Outgoing spend types (our own optimistic spend records).
const (
	SpendWedge  SpendType = "wedge"
	SpendPoints SpendType = "points"
)

// Outgoing spend reasons.
const (
	SpendReasonGrab       SpendReason = "grab"
	SpendReasonShopVIP    SpendReason = "shop_vip"
	SpendReasonShopUpload SpendReason = "shop_upload"
)

// Bonus types, mirrored from MAM userBonusHistory (income and MAM-recorded spends).
const (
	BonusGiftPoints    BonusType = "giftPoints"
	BonusGiftWedge     BonusType = "giftWedge"
	BonusWedgePF       BonusType = "wedgePF"
	BonusWedgeGFL      BonusType = "wedgeGFL"
	BonusTorrentThanks BonusType = "torrentThanks"
	BonusMillionaires  BonusType = "millionaires"
)

// Media kinds.
const (
	KindEbook     Kind = "ebook"
	KindAudiobook Kind = "audiobook"
	KindOther     Kind = "other"
)

// Grab origins.
const (
	SourceAnnounce Source = "announce"
	SourceTorznab  Source = "torznab"
)

// Layer-1 accept decisions.
const (
	Accept AcceptDecision = "accept"
	Reject AcceptDecision = "reject"
)

// Layer-1 reject reasons (Accept uses AcceptOK).
const (
	AcceptOK            AcceptReason = "ok"
	AcceptKind          AcceptReason = "kind"
	AcceptCategoryAllow AcceptReason = "category_allow"
	AcceptCategoryDeny  AcceptReason = "category_deny"
	AcceptFiletypeAllow AcceptReason = "filetype_allow"
	AcceptFiletypeDeny  AcceptReason = "filetype_deny"
	AcceptLanguage      AcceptReason = "language"
	AcceptSizeMin       AcceptReason = "size_min"
	AcceptSizeMax       AcceptReason = "size_max"
	AcceptFreeleech     AcceptReason = "freeleech"
	AcceptVIPOnly       AcceptReason = "vip_only"
	AcceptVIPExclude    AcceptReason = "vip_exclude"
	AcceptNameInclude   AcceptReason = "name_include"
	AcceptNameExclude   AcceptReason = "name_exclude"
	AcceptDuplicate     AcceptReason = "duplicate" // already grabbed this tid (dedup)
)

// Layer-2 grab-gate decisions.
const (
	Grab GrabDecision = "grab"
	Hold GrabDecision = "hold"
	Skip GrabDecision = "skip"
)

// Layer-2 grab-gate reasons.
const (
	GrabOK           GrabReason = "ok"
	GrabOKWedge      GrabReason = "ok_wedge"
	GrabNoAccount    GrabReason = "no_account"
	GrabUnsatCeiling GrabReason = "unsat_ceiling"
	GrabBuffer       GrabReason = "buffer"
	GrabVIPRequired  GrabReason = "vip_required"
	GrabLayer1Reject GrabReason = "layer1_reject"
)

// Webhook event types (the {{.Type}} a notification template branches on).
type WebhookEvent string

const (
	WebhookAnnounce      WebhookEvent = "announce"       // every release received
	WebhookGrab          WebhookEvent = "grab"           // accepted + grab-gate said grab
	WebhookReject        WebhookEvent = "reject"         // Layer-1 reject
	WebhookHold          WebhookEvent = "hold"           // accepted but held/skipped (unsat/buffer/vip)
	WebhookEnqueue       WebhookEvent = "enqueue"        // successfully added to the download client
	WebhookBonusReceived WebhookEvent = "bonus_received" // new MAM bonus ledger row
	WebhookSpend         WebhookEvent = "spend"          // points/wedge spent
)

// Grab action results.
const (
	ResultDryRun      GrabResult = "dryrun"
	ResultNoURL       GrabResult = "no_url"
	ResultFetchFailed GrabResult = "fetch_failed"
	ResultAddFailed   GrabResult = "add_failed"
	ResultAddRejected GrabResult = "add_rejected"
	ResultAdded       GrabResult = "added"
	ResultAddedWedge  GrabResult = "added_wedge"
	ResultNone        GrabResult = "none"
)

// Bonus-point shop actions.
const (
	ShopHold      ShopAction = "hold"
	ShopBuyVIP    ShopAction = "buy_vip"
	ShopBuyUpload ShopAction = "buy_upload"
)
