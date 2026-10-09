package actions

import (
	"encoding/json"
	"net/http"

	"homelab/mousetrap/internal/app"
	"homelab/mousetrap/internal/mam"
)

// searchRequest is the /search body — a subset of MAM search params for ad-hoc
// queries. All fields optional; zero values fall back to MAM defaults.
type searchRequest struct {
	Text        string `json:"text"`
	SearchType  string `json:"searchType"`
	SortType    string `json:"sortType"`
	MainCat     []int  `json:"mainCat"`
	Cat         []int  `json:"cat"`
	PerPage     int    `json:"perPage"`
	StartNumber int    `json:"startNumber"`
	MySnatched  bool   `json:"mySnatched"`
}

// Search runs an ad-hoc MAM search and returns the results verbatim.
func Search(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req searchRequest
		_ = json.NewDecoder(r.Body).Decode(&req) // empty body → all defaults

		res, err := a.MAM.Search(r.Context(), mam.SearchParams{
			Text:        req.Text,
			SearchType:  req.SearchType,
			SortType:    req.SortType,
			MainCat:     req.MainCat,
			Cat:         req.Cat,
			PerPage:     req.PerPage,
			StartNumber: req.StartNumber,
			MySnatched:  req.MySnatched,
		})
		if err != nil {
			a.Metrics.MAMError("search")
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(res)
	}
}
