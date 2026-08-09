package actions

import (
	"context"
	"encoding/xml"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"mousetrap/app"
	"mousetrap/mam"
	"mousetrap/policy"
	"mousetrap/store"
	"mousetrap/types"
)

// Torznab serves the indexer interface at /torznab/api: t=caps returns the static
// capability document, t=search/book run a MAM search filtered by Layer-1 accept and
// return an RSS/torznab feed. Each result's download link points back at
// /torznab/download so the actual grab still runs the full policy (mousetrap stays
// the sole MAM grab authority).
func Torznab(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if !torznabAuthorized(a, q) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch q.Get("t") {
		case "caps":
			writeXML(w, "application/xml", capsDoc())
		case "search", "book", "":
			torznabSearch(a, w, r, q)
		default:
			http.Error(w, "unsupported function", http.StatusBadRequest)
		}
	}
}

// TorznabDownload serves the download link minted into each search item. It hydrates
// the release from the URL params (no MAM re-fetch — the metadata is carried in the
// link), re-checks Layer-1 accept, runs the Layer-2 grab-gate under the shared grab
// lock, fetches the .torrent from MAM, records the grab (source=torznab), and returns
// the bytes. The downstream client adds it to its own client; mousetrap never adds it
// here. A held grab is a 404 so the consumer marks the release failed and moves on.
func TorznabDownload(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if !torznabAuthorized(a, q) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		tid := parseInt64(q.Get("id"))
		if tid == 0 {
			http.Error(w, "missing id", http.StatusBadRequest)
			return
		}
		ctx := r.Context()

		// Already grabbed → serve the .torrent again idempotently, no new ledger row.
		if seen, err := a.Store.GrabSeen(ctx, tid); err == nil && seen {
			data, err := a.MAM.Download(ctx, tid, false)
			if err != nil {
				a.Metrics.MAMError("download")
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}
			writeTorrent(w, data)
			return
		}

		// Reject stale links: their freeleech/vip/size metadata may no longer hold.
		if a.Cfg.Torznab.LinkTTL > 0 {
			if ts := parseInt64(q.Get("ts")); ts > 0 &&
				time.Since(time.Unix(ts, 0)) > a.Cfg.Torznab.LinkTTL {
				http.Error(w, "link expired", http.StatusGone)
				return
			}
		}

		rel := releaseFromQuery(q)

		// Layer-1 (defense in depth — search already filtered, but the link is trusted).
		if d, _ := a.Policy.Accept(rel); d != types.Accept {
			http.Error(w, "rejected", http.StatusNotFound)
			return
		}

		gctx, cancel := context.WithTimeout(ctx, grabTimeout)
		defer cancel()

		// Refresh before the lock so the slow poll doesn't serialize behind it.
		if err := a.State.RefreshAccount(gctx); err != nil {
			a.Metrics.MAMError("loaduser")
		}

		// Serialized grab: in-flight read, gate, and grab record are atomic.
		var (
			result types.GrabResult
			data   []byte
		)
		func() {
			a.GrabMu.Lock()
			defer a.GrabMu.Unlock()

			acct := a.State.Account()
			since := time.Now().Add(-a.Cfg.Grab.InFlightWindow).Unix()
			inflight, _ := a.Store.InFlightSince(gctx, since)

			gd, _, plan := a.Policy.Grab(rel, acct, inflight)
			if gd != types.Grab {
				result = types.ResultNone
				return
			}
			result, data = execTorznabGrab(gctx, a, rel, plan)
		}()

		switch result {
		case types.ResultAdded, types.ResultAddedWedge:
			writeTorrent(w, data)
		case types.ResultFetchFailed:
			http.Error(w, "fetch failed", http.StatusBadGateway)
		default: // held by the gate
			http.Error(w, "held", http.StatusNotFound)
		}
	}
}

// execTorznabGrab fetches the .torrent from MAM and records the grab (and wedge spend,
// if any) as source=torznab. It returns the bytes rather than adding to a client — the
// downstream consumer owns delivery. Runs under the grab lock. Mirrors announce's
// execGrab minus the client add and the announce foreign key.
func execTorznabGrab(ctx context.Context, a *app.App, rel types.Release, plan policy.GrabPlan) (types.GrabResult, []byte) {
	defer func(start time.Time) { a.Metrics.ObserveGrab(time.Since(start)) }(time.Now())

	data, err := a.MAM.Download(ctx, rel.TID, plan.UseWedge)
	if err != nil {
		a.Metrics.MAMError("download")
		logGrabError(rel, err)
		recordTorznabGrab(ctx, a, rel, plan, types.ResultFetchFailed, nil)
		return types.ResultFetchFailed, nil
	}

	// Download succeeded → the wedge (if used) is now spent; record it.
	var spendID *int64
	if plan.UseWedge {
		if sid, err := a.Store.InsertSpend(ctx, store.Spend{
			Type: types.SpendWedge, TID: &rel.TID, Reason: types.SpendReasonGrab,
		}); err == nil {
			spendID = &sid
		}
	}

	result := types.ResultAdded
	if plan.UseWedge {
		result = types.ResultAddedWedge
	}
	recordTorznabGrab(ctx, a, rel, plan, result, spendID)
	return result, data
}

// recordTorznabGrab persists a torznab-sourced grab (no announce foreign key).
func recordTorznabGrab(ctx context.Context, a *app.App, rel types.Release, plan policy.GrabPlan, result types.GrabResult, spendID *int64) {
	_, _ = a.Store.InsertGrab(ctx, store.Grab{
		SpendID:    spendID,
		Source:     types.SourceTorznab,
		TID:        rel.TID,
		Size:       rel.Size,
		CostsRatio: plan.CostsRatio,
		Result:     string(result),
	})
}

// torznabSearch runs the MAM search, filters survivors through Layer-1 accept, and
// writes the RSS/torznab feed.
func torznabSearch(a *app.App, w http.ResponseWriter, r *http.Request, q url.Values) {
	text := strings.TrimSpace(strings.Join(nonEmpty(q.Get("q"), q.Get("author"), q.Get("title")), " "))
	limit := atoiDefault(q.Get("limit"), 100)
	if limit > 100 {
		limit = 100
	}

	res, err := a.MAM.Search(r.Context(), searchParamsFor(text, q.Get("cat"), limit, atoiDefault(q.Get("offset"), 0)))
	if err != nil {
		a.Metrics.MAMError("search")
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	feed := rssFeed{Version: "2.0", XMLNSTorznab: torznabNS}
	feed.Channel.Title = "mousetrap"
	for _, t := range res.Data {
		rel := releaseFromTorrent(t)
		if d, _ := a.Policy.Accept(rel); d != types.Accept {
			continue
		}
		feed.Channel.Items = append(feed.Channel.Items, buildItem(a, rel, t))
	}
	writeXML(w, "application/rss+xml", feed)
}

// ── Torznab XML: capabilities ──

const torznabNS = "http://torznab.com/schemas/2015/feed"

type capsResponse struct {
	XMLName    xml.Name       `xml:"caps"`
	Server     capsServer     `xml:"server"`
	Limits     capsLimits     `xml:"limits"`
	Searching  capsSearching  `xml:"searching"`
	Categories capsCategories `xml:"categories"`
}

type capsServer struct {
	Title string `xml:"title,attr"`
}
type capsLimits struct {
	Max     int `xml:"max,attr"`
	Default int `xml:"default,attr"`
}
type capsSearching struct {
	Search      capsMode `xml:"search"`
	BookSearch  capsMode `xml:"book-search"`
	TVSearch    capsMode `xml:"tv-search"`
	MovieSearch capsMode `xml:"movie-search"`
}
type capsMode struct {
	Available       string `xml:"available,attr"`
	SupportedParams string `xml:"supportedParams,attr"`
}
type capsCategories struct {
	Categories []capsCategory `xml:"category"`
}
type capsCategory struct {
	ID      int          `xml:"id,attr"`
	Name    string       `xml:"name,attr"`
	Subcats []capsSubcat `xml:"subcat"`
}
type capsSubcat struct {
	ID   int    `xml:"id,attr"`
	Name string `xml:"name,attr"`
}

// capsDoc is the static capability document. Books-only domain: two Newznab-standard
// parent trees — Audio(3000)/Audiobook(3030) and Books(7000)/Ebook(7020).
func capsDoc() capsResponse {
	return capsResponse{
		Server: capsServer{Title: "mousetrap"},
		Limits: capsLimits{Max: 100, Default: 100},
		Searching: capsSearching{
			Search:      capsMode{Available: "yes", SupportedParams: "q"},
			BookSearch:  capsMode{Available: "yes", SupportedParams: "q,author,title"},
			TVSearch:    capsMode{Available: "no", SupportedParams: "q"},
			MovieSearch: capsMode{Available: "no", SupportedParams: "q"},
		},
		Categories: capsCategories{Categories: []capsCategory{
			{ID: 3000, Name: "Audio", Subcats: []capsSubcat{{ID: 3030, Name: "Audiobook"}}},
			{ID: 7000, Name: "Books", Subcats: []capsSubcat{{ID: 7020, Name: "Ebook"}}},
		}},
	}
}

// ── Torznab XML: search feed ──

type rssFeed struct {
	XMLName      xml.Name   `xml:"rss"`
	Version      string     `xml:"version,attr"`
	XMLNSTorznab string     `xml:"xmlns:torznab,attr"`
	Channel      rssChannel `xml:"channel"`
}
type rssChannel struct {
	Title string    `xml:"title"`
	Items []rssItem `xml:"item"`
}
type rssItem struct {
	Title     string        `xml:"title"`
	GUID      string        `xml:"guid"`
	Link      string        `xml:"link"`
	Size      int64         `xml:"size"`
	Enclosure rssEnclosure  `xml:"enclosure"`
	Attrs     []torznabAttr `xml:"torznab:attr"`
}
type rssEnclosure struct {
	URL    string `xml:"url,attr"`
	Length int64  `xml:"length,attr"`
	Type   string `xml:"type,attr"`
}
type torznabAttr struct {
	Name  string `xml:"name,attr"`
	Value string `xml:"value,attr"`
}

// buildItem renders one search result: the download enclosure points back at
// /torznab/download, and torznab attrs carry the category/peer/size metadata.
func buildItem(a *app.App, rel types.Release, t mam.Torrent) rssItem {
	link := mintDownloadURL(a, rel)
	return rssItem{
		Title:     rel.Name,
		GUID:      strconv.FormatInt(rel.TID, 10),
		Link:      link,
		Size:      rel.Size,
		Enclosure: rssEnclosure{URL: link, Length: rel.Size, Type: "application/x-bittorrent"},
		Attrs: []torznabAttr{
			{Name: "category", Value: torznabCatForKind(rel.Kind)},
			{Name: "seeders", Value: strconv.Itoa(t.Seeders)},
			{Name: "peers", Value: strconv.Itoa(t.Seeders + t.Leechers)},
			{Name: "size", Value: strconv.FormatInt(rel.Size, 10)},
			{Name: "files", Value: strconv.Itoa(t.NumFiles)},
			{Name: "grabs", Value: strconv.Itoa(t.TimesCompleted)},
		},
	}
}

// mintDownloadURL builds the self-referential download link carrying every field the
// grab-gate needs (metadata packed plainly; trusted LAN). ts stamps it for the TTL.
func mintDownloadURL(a *app.App, rel types.Release) string {
	v := url.Values{}
	v.Set("id", strconv.FormatInt(rel.TID, 10))
	v.Set("name", rel.Name)
	v.Set("cat", rel.Category)
	v.Set("ft", rel.Filetype)
	v.Set("lang", strings.Join(rel.Languages, ","))
	v.Set("size", strconv.FormatInt(rel.Size, 10))
	v.Set("vip", bit(rel.VIP))
	v.Set("fl", bit(rel.Freeleech))
	if a.Cfg.Torznab.LinkTTL > 0 {
		v.Set("ts", strconv.FormatInt(time.Now().Unix(), 10))
	}
	if a.Cfg.Torznab.APIKey != "" {
		v.Set("apikey", a.Cfg.Torznab.APIKey)
	}
	return strings.TrimRight(a.Cfg.Torznab.BaseURL, "/") + "/torznab/download?" + v.Encode()
}

// ── helpers ──

func torznabAuthorized(a *app.App, q url.Values) bool {
	key := a.Cfg.Torznab.APIKey
	return key == "" || q.Get("apikey") == key
}

// searchParamsFor maps torznab query inputs onto MAM search params.
func searchParamsFor(text, cat string, limit, offset int) mam.SearchParams {
	return mam.SearchParams{
		Text:        text,
		MainCat:     mainCatsFor(cat),
		SortType:    "dateDesc",
		PerPage:     limit,
		StartNumber: offset,
	}
}

// mainCatsFor maps a torznab cat list (comma-separated ids) to MAM main categories.
// Empty/unrecognized → both books main-cats (13 AudioBooks, 14 E-Books).
func mainCatsFor(cat string) []int {
	seen := map[int]bool{}
	for _, part := range strings.Split(cat, ",") {
		switch strings.TrimSpace(part) {
		case "3000", "3030":
			seen[13] = true
		case "7000", "7020":
			seen[14] = true
		}
	}
	if len(seen) == 0 {
		return []int{13, 14}
	}
	out := make([]int, 0, 2)
	for _, mc := range []int{13, 14} {
		if seen[mc] {
			out = append(out, mc)
		}
	}
	return out
}

// torznabCatForKind maps our media kind to the Newznab-standard category id.
func torznabCatForKind(k types.Kind) string {
	switch k {
	case types.KindAudiobook:
		return "3030"
	case types.KindEbook:
		return "7020"
	default:
		return "7000"
	}
}

// kindFromMainCat derives media kind from a MAM main-category id.
func kindFromMainCat(mainCat int) types.Kind {
	switch mainCat {
	case 13:
		return types.KindAudiobook
	case 14:
		return types.KindEbook
	default:
		return types.KindOther
	}
}

// releaseFromTorrent maps a MAM search result to the policy's domain release.
func releaseFromTorrent(t mam.Torrent) types.Release {
	return types.Release{
		TID:       t.ID,
		Name:      t.Title,
		Category:  t.CatName,
		Kind:      kindFromMainCat(t.MainCat),
		Filetype:  t.Filetype,
		Languages: langSliceCSV(t.LangCode),
		Size:      parseFormattedSize(t.Size),
		VIP:       t.VIP == 1,
		Freeleech: t.Free == 1,
	}
}

// releaseFromQuery reconstructs the release from a minted download link's params.
func releaseFromQuery(q url.Values) types.Release {
	return types.Release{
		TID:       parseInt64(q.Get("id")),
		Name:      q.Get("name"),
		Category:  q.Get("cat"),
		Kind:      kindOf(q.Get("cat")),
		Filetype:  q.Get("ft"),
		Languages: langSliceCSV(q.Get("lang")),
		Size:      parseInt64(q.Get("size")),
		VIP:       truthy(q.Get("vip")),
		Freeleech: truthy(q.Get("fl")),
	}
}

func writeXML(w http.ResponseWriter, contentType string, doc any) {
	w.Header().Set("Content-Type", contentType+"; charset=utf-8")
	_, _ = w.Write([]byte(xml.Header))
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	_ = enc.Encode(doc)
}

func writeTorrent(w http.ResponseWriter, data []byte) {
	w.Header().Set("Content-Type", "application/x-bittorrent")
	_, _ = w.Write(data)
}

func bit(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

func nonEmpty(vals ...string) []string {
	out := vals[:0]
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			out = append(out, v)
		}
	}
	return out
}

func atoiDefault(s string, def int) int {
	if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil && n > 0 {
		return n
	}
	return def
}

// langSliceCSV splits a comma-separated language value and normalizes each element
// (lowercase + trim), dropping empties; "" → nil.
func langSliceCSV(s string) []string {
	return normLangs(strings.Split(s, ","))
}

// parseFormattedSize converts a MAM display size like "6.0 MiB" to bytes. Duplicated
// from state's parser (sharing isn't necessary; the actions layer stays self-contained).
func parseFormattedSize(s string) int64 {
	s = strings.ReplaceAll(strings.TrimSpace(s), ",", "")
	if s == "" {
		return 0
	}
	i := 0
	for i < len(s) && (s[i] == '.' || (s[i] >= '0' && s[i] <= '9')) {
		i++
	}
	num, err := strconv.ParseFloat(strings.TrimSpace(s[:i]), 64)
	if err != nil {
		return 0
	}
	factor := 1.0
	if f, ok := sizeUnits[strings.ToUpper(strings.TrimSpace(s[i:]))]; ok {
		factor = f
	}
	return int64(num * factor)
}

var sizeUnits = map[string]float64{
	"B":   1,
	"KIB": 1 << 10, "MIB": 1 << 20, "GIB": 1 << 30, "TIB": 1 << 40, "PIB": 1 << 50,
	"KB": 1e3, "MB": 1e6, "GB": 1e9, "TB": 1e12, "PB": 1e15,
}
