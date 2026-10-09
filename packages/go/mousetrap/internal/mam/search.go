package mam

import (
	"context"
	"encoding/json"
	"net/http"
)

// SearchParams drives loadSearchJSONbasic. Zero values fall back to MAM defaults.
type SearchParams struct {
	Text        string   // free-text query
	SearchType  string   // all|active|inactive|fl|fl-VIP|VIP|nVIP|nMeta
	SortType    string   // dateDesc|dateAsc|seedersDesc|sizeDesc|snatchedDesc|titleAsc|random|...
	MainCat     []int    // 13 AudioBooks, 14 E-Books, 15 Musicology, 16 Radio
	Cat         []int    // sub-category ids
	BrowseLang  []int    // language ids to include
	SearchIn    []string // srchIn: title|author|narrator|series|tags|filenames|fileTypes|description
	StartDate   string   // YYYY-MM-DD or unix ts; inclusive lower bound
	EndDate     string   // YYYY-MM-DD or unix ts; exclusive upper bound
	Hash        string   // hex torrent hash
	ID          int      // single torrent id
	PerPage     int      // 5..1000
	StartNumber int      // pagination offset
	MySnatched  bool     // limit to torrents you have snatched
	Description bool     // include the description field
	ISBN        bool     // include the isbn field
	MediaInfo   bool     // include key mediaInfo parts
	DLLink      bool     // include the dl download hash
}

// flexString decodes a field MAM sends as either a JSON string or a bare
// number. `tags` is the known offender: normally freeform text, but rows whose
// tags are purely numeric come back unquoted, and a plain `string` field fails
// the decode — which fails the whole page, not just that row, so one such
// torrent 502s an entire search ("cannot unmarshal number into Go struct field
// Torrent.data.tags of type string"). Numbers are kept as their literal digits;
// null becomes "".
type flexString string

func (s *flexString) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] == '"' {
		var str string
		if err := json.Unmarshal(b, &str); err != nil {
			return err
		}
		*s = flexString(str)
		return nil
	}
	if string(b) == "null" {
		*s = ""
		return nil
	}
	*s = flexString(b)
	return nil
}

// Torrent is one loadSearchJSONbasic result — the complete response object. MAM
// encodes the *_info / categories / ownership / mediainfo fields as JSON embedded
// in a string, sends the flag fields as 0/1 numbers, and formats size as a
// human-readable string (e.g. "6.0 MiB"). Fields are kept wire-faithful here;
// normalization (size→bytes, 0/1→bool, decoding the embedded JSON) is the caller's.
type Torrent struct {
	Added             string     `json:"added"`              // upload datetime (UTC)
	AuthorInfo        string     `json:"author_info"`        // JSON object {id:name} as a string
	Bookmarked        string     `json:"bookmarked"`         // bookmark datetime; null when not bookmarked
	BrowseFlags       int        `json:"browseflags"`        // bitfield of tags/flags
	Cat               string     `json:"cat"`                // HTML category display
	Categories        string     `json:"categories"`         // JSON array of ids as a string
	Category          int        `json:"category"`           // specific category id
	CatName           string     `json:"catname"`            // category name
	Comments          int        `json:"comments"`           // comment count
	DL                string     `json:"dl"`                 // per-user download hash; null unless requested
	Filetype          string     `json:"filetype"`           // concatenated file types
	FLVIP             int        `json:"fl_vip"`             // 0/1: freeleech and/or vip
	Free              int        `json:"free"`               // 0/1: freeleech
	ID                int64      `json:"id"`                 // torrent id
	LangCode          string     `json:"lang_code"`          // ISO language code
	Language          int        `json:"language"`           // internal language id
	Leechers          int        `json:"leechers"`           // leecher count
	MainCat           int        `json:"main_cat"`           // main category id
	MainCatAlt        int        `json:"maincat"`            // MAM's second main-category field
	MediaInfo         string     `json:"mediainfo"`          // JSON as a string
	MediaType         int        `json:"mediatype"`          // media type id
	MySnatched        int        `json:"my_snatched"`        // 0/1 (5–20 min lag)
	NarratorInfo      string     `json:"narrator_info"`      // JSON object {id:name} as a string
	NumFiles          int        `json:"numfiles"`           // file count
	Ownership         string     `json:"ownership"`          // JSON array [id, name] as a string
	PersonalFreeleech int        `json:"personal_freeleech"` // 0/1: bought as personal FL (5–20 min lag)
	PosterType        string     `json:"poster_type"`        // cover image mime type
	Seeders           int        `json:"seeders"`            // seeder count
	SeriesInfo        string     `json:"series_info"`        // JSON object as a string
	Size              string     `json:"size"`               // human-formatted size, e.g. "6.0 MiB"
	Tags              flexString `json:"tags"`               // freeform tags; sometimes a bare number
	TimesCompleted    int        `json:"times_completed"`    // full-snatch count
	Title             string     `json:"title"`              // torrent title
	VIP               int        `json:"vip"`                // 0/1: vip
	VIPExpire         int        `json:"vip_expire"`         // vip expiry (0 = none)
	W                 int        `json:"w"`                  // search weight

	// Present only when the matching request flag is set:
	Description string `json:"description,omitempty"` // Description
	ISBN        string `json:"isbn,omitempty"`        // ISBN

	// Documented alternatives to Ownership (rarely returned):
	Owner     int    `json:"owner,omitempty"`      // uploader user id
	OwnerName string `json:"owner_name,omitempty"` // uploader username
}

// SearchResult is one page of results. Total grows as later pages are requested;
// TotalFound is the full match count.
type SearchResult struct {
	Total      int       `json:"total"`
	TotalFound int       `json:"total_found"`
	Data       []Torrent `json:"data"`
}

// Search queries loadSearchJSONbasic.
func (c *Client) Search(ctx context.Context, p SearchParams) (*SearchResult, error) {
	tor := map[string]any{"startNumber": p.StartNumber}
	if p.Text != "" {
		tor["text"] = p.Text
	}
	if p.SearchType != "" {
		tor["searchType"] = p.SearchType
	}
	if p.SortType != "" {
		tor["sortType"] = p.SortType
	}
	if len(p.MainCat) > 0 {
		tor["main_cat"] = p.MainCat
	}
	if len(p.Cat) > 0 {
		tor["cat"] = p.Cat
	}
	if len(p.BrowseLang) > 0 {
		tor["browse_lang"] = p.BrowseLang
	}
	if len(p.SearchIn) > 0 {
		tor["srchIn"] = p.SearchIn
	}
	if p.StartDate != "" {
		tor["startDate"] = p.StartDate
	}
	if p.EndDate != "" {
		tor["endDate"] = p.EndDate
	}
	if p.Hash != "" {
		tor["hash"] = p.Hash
	}
	if p.ID > 0 {
		tor["id"] = p.ID
	}

	body := map[string]any{"tor": tor}
	if p.PerPage > 0 {
		body["perpage"] = p.PerPage
	}
	if p.MySnatched {
		body["my_snatched"] = true
	}
	if p.Description {
		body["description"] = true
	}
	if p.ISBN {
		body["isbn"] = true
	}
	if p.MediaInfo {
		body["mediaInfo"] = true
	}
	if p.DLLink {
		body["dlLink"] = true
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	data, err := c.do(ctx, http.MethodPost, "/tor/js/loadSearchJSONbasic.php", nil, raw)
	if err != nil {
		return nil, err
	}
	var res SearchResult
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, jsonErr("search", err, data)
	}
	return &res, nil
}
