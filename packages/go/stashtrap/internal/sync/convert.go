package sync

import (
	"net/url"
	"slices"
	"strings"

	"homelab/clients/stashbox"
	"homelab/stashtrap/internal/store"
)

// The stash-box types and the store types are deliberately separate: the wire
// shape is the service's to change, the stored shape is ours. These functions
// are the one place the two meet.

func performerFrom(p stashbox.Performer) store.Performer {
	return store.Performer{
		ID:             p.ID,
		Name:           p.Name,
		Disambiguation: p.Disambiguation,
		Gender:         p.Gender,
		Aliases:        p.Aliases,
		URLs:           urlsFrom(p.URLs),
		IsFavourite:    p.IsFavorite,
		Deleted:        p.Deleted,
		MergedIntoID:   p.MergedIntoID,
		UpdatedAt:      p.Updated,
	}
}

func studioFrom(s stashbox.Studio) store.Studio {
	out := store.Studio{
		ID:          s.ID,
		Name:        s.Name,
		Aliases:     append(append([]string{}, s.Aliases...), domainAliases(s.URLs)...),
		URLs:        urlsFrom(s.URLs),
		IsFavourite: s.IsFavorite,
		Deleted:     s.Deleted,
		UpdatedAt:   s.Updated,
	}
	if s.Parent != nil {
		out.ParentID = s.Parent.ID
	}
	return out
}

// homeSite is the site name stash-box gives a studio's own website. The others
// — IAFD, Data18, Twitter — are directories, and their hosts would alias every
// studio that lists one to the directory itself.
const homeSite = "Home"

// minDomainAlias is the shortest derived alias worth storing. Below it the
// label is a country or second-level suffix rather than a name.
const minDomainAlias = 3

// domainAliases derives the names a release writes a studio under from the
// studio's own website.
//
// Neither the canonical name nor the alias list carries the domain: StashDB
// calls it "1000 Facials" while the release calls it "1000Facials.com". The
// URL is the only place the two forms meet.
func domainAliases(urls []stashbox.URL) []string {
	var out []string
	for _, u := range urls {
		if u.Site.Name != homeSite {
			continue
		}
		out = append(out, hostAliases(u.URL)...)
	}
	return out
}

// hostAliases strips a URL down to the name in its host: no scheme, no www, no
// TLD. A host with a subdomain yields both readings — "tour.example.com" is
// announced as either — since an alias matching nothing costs a row.
func hostAliases(raw string) []string {
	u, err := url.Parse(raw)
	if err != nil {
		return nil
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	labels := strings.Split(host, ".")
	if len(labels) < 2 {
		return nil
	}
	labels = labels[:len(labels)-1]

	var out []string
	for _, alias := range []string{strings.Join(labels, ""), labels[len(labels)-1]} {
		if len(alias) >= minDomainAlias && !slices.Contains(out, alias) {
			out = append(out, alias)
		}
	}
	return out
}

func sceneFrom(s stashbox.Scene) store.Scene {
	out := store.Scene{
		ID:          s.ID,
		Title:       s.Title,
		ReleaseDate: s.ReleaseDate,
		Code:        s.Code,
		Duration:    s.Duration,
		Deleted:     s.Deleted,
		UpdatedAt:   s.Updated,
		URLs:        urlsFrom(s.URLs),
	}
	if s.Studio != nil {
		out.StudioID = s.Studio.ID
	}
	for _, a := range s.Performers {
		// Fall back to the canonical name when a scene credits no alias:
		// an empty normalised column would index every such appearance
		// under the same empty key.
		as := a.As
		if as == "" {
			as = a.Performer.Name
		}
		out.Performers = append(out.Performers, store.ScenePerformer{
			PerformerID: a.Performer.ID,
			As:          as,
		})
	}
	for _, t := range s.Tags {
		out.Tags = append(out.Tags, store.Tag{ID: t.ID, Name: t.Name})
	}
	for _, f := range s.Fingerprints {
		out.Fingerprints = append(out.Fingerprints, store.Fingerprint{
			Algorithm:   f.Algorithm,
			Hash:        f.Hash,
			Duration:    f.Duration,
			Submissions: f.Submissions,
		})
	}
	return out
}

func urlsFrom(in []stashbox.URL) []store.CatalogueURL {
	if len(in) == 0 {
		return nil
	}
	out := make([]store.CatalogueURL, 0, len(in))
	for _, u := range in {
		out = append(out, store.CatalogueURL{URL: u.URL, Site: u.Site.Name})
	}
	return out
}
