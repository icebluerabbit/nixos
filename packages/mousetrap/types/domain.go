package types

import "strings"

// Release is a torrent candidate the policy evaluates — normalized from an autobrr
// announce or a MAM search result into the fields the decision engine needs.
type Release struct {
	TID         int64
	Name        string
	Category    string   // exact MAM category, e.g. "Ebooks - Fantasy"
	Kind        Kind     // derived from Category
	Filetype    string   // e.g. epub, m4b, mp3
	Languages   []string // reduced language set (autobrr list, or name-parsed fallback)
	Size        int64    // bytes
	VIP         bool     // raw release VIP flag
	Freeleech   bool     // raw release freeleech flag
	DownloadURL string
}

// AccountState is the latest polled MAM account snapshot consumed by the grab-gate.
type AccountState struct {
	Ratio           float64
	UploadedBytes   int64
	DownloadedBytes int64
	UnsatCount      int
	UnsatLimit      int
	SeedingCount    int // sSat + seedHnr + seedUnsat
	SatisfiedCount  int // sSat
	SeedUnsatCount  int // seedUnsat
	LeechingCount   int
	HnrCount        int // seedHnr + inactHnr
	SeedHnrCount    int // seedHnr (seeding but hit-and-run)
	InactHnrCount   int // inactHnr (inactive hit-and-run)
	Wedges          int
	SeedBonus       int64 // bonus points (for the shop)
	ClassName       string
}

// Buffer is uploaded minus downloaded — the ratio buffer the grab-gate floors on.
func (a AccountState) Buffer() int64 { return a.UploadedBytes - a.DownloadedBytes }

// VIP reports whether the account itself is VIP — a VIP release is freeleech for us
// only when this is true.
func (a AccountState) VIP() bool {
	return strings.Contains(strings.ToLower(a.ClassName), "vip")
}

// InFlight is the optimistic aggregate of recent grabs within the reconciliation
// window: how many count against the unsat ceiling, how many ratio-costing bytes
// against the buffer, and how many wedges spent.
type InFlight struct {
	Count      int
	RatioBytes int64
	Wedges     int
}
