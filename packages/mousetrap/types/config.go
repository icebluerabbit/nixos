// Package types holds mousetrap's pure domain declarations — config shape, domain
// entities, and enums. No logic, no I/O, no pflag: it is a leaf that everything
// imports and that imports nothing. Keep it that way — behavior belongs elsewhere.
package types

import "time"

// Config is the fully-resolved runtime configuration. Package config populates it
// from command-line flags. Two axes: integrations (external systems mousetrap
// talks to — MAM, the download client, the webhook sink) and policy (the decision
// rules — accept, grab, wedge). Policy is client-agnostic; it never names qbt.
type Config struct {
	// process
	Listen   string
	Port     string
	StateDir string

	// integrations
	MAM     MAMConfig
	Client  ClientConfig
	Webhook WebhookConfig
	Torznab TorznabConfig

	// policy
	Accept AcceptConfig
	Grab   GrabConfig
	Wedge  WedgeConfig
	Shop   ShopConfig
}

// Addr is the HTTP bind address.
func (c Config) Addr() string { return c.Listen + ":" + c.Port }

// MAMConfig is the MyAnonaMouse source client.
type MAMConfig struct {
	APIBase  string
	SeedPath string // path to the mam_id seed secret (the path, never the value)
}

// ClientConfig is the download client (qBittorrent today). The domain is kept
// client-agnostic so a second client is a new Type + settings here, not a change
// to grab policy.
type ClientConfig struct {
	URL        string
	Categories map[string]string // kind -> client category
	AddPaused  bool
}

// WebhookConfig configures an outbound notification POST. When URL is set, the
// rendered Template is POSTed on the triggering event with the given Headers.
// Template is a Go text/template evaluated over the event's data.
type WebhookConfig struct {
	URL      string
	Template string
	Headers  map[string]string
	Events   []string // event types that fire; empty = none
}

// TorznabConfig is the Torznab indexer surface mousetrap exposes to library
// managers (Prowlarr/Readarr/LazyLibrarian). Search results carry download links
// pointing back here, so every grab still runs the full policy — mousetrap stays
// the sole MAM grab authority. Link metadata is carried plainly in the URL (trusted
// LAN, no signing); LinkTTL bounds how stale that metadata (freeleech/vip flags,
// size) may be by grab time.
type TorznabConfig struct {
	APIKey  string        // required apikey on requests; "" = no auth
	BaseURL string        // absolute base for minted download links, e.g. http://mousetrap:8080
	LinkTTL time.Duration // reject download links older than this; 0 = no expiry
}

// AcceptConfig is the Layer-1 acceptance policy — release-field filters only, no
// account state. Empty list / 0 / "" means the rule is off.
type AcceptConfig struct {
	Kinds            []string // allowed media kinds; empty = any
	CategoriesAllow  []string // exact category allowlist; empty = any
	CategoriesDeny   []string // exact category denylist; takes precedence
	FiletypesAllow   []string // filetype allowlist; empty = any
	FiletypesDeny    []string // filetype denylist; takes precedence
	LanguagesAllow   []string // language substrings, case-insensitive; empty = any
	SizeMinBytes     int64    // reject below; 0 = no floor
	SizeMaxBytes     int64    // reject above; 0 = no ceiling
	FreeleechOnly    bool     // reject non-freeleech releases
	VIPOnly          bool     // reject non-VIP releases (raw release flag)
	ExcludeVIP       bool     // reject VIP releases (raw release flag)
	NameIncludeRegex string   // require name to match (case-insensitive); "" = off
	NameExcludeRegex string   // reject name matching (case-insensitive); "" = off
}

// GrabConfig is the Layer-2 grab-gate policy — pure account-state thresholds
// applied to Layer-1 survivors. Where/how a grab is delivered is ClientConfig.
type GrabConfig struct {
	UnsatMargin    int           // hold this many below the unsatisfied limit
	InFlightWindow time.Duration // how long a grab counts as in-flight
	BufferMinBytes int64         // non-freeleech grabs require buffer >= this; 0 = off
	DryRun         bool          // decide and log only; grab nothing
}

// WedgeConfig is the Personal Freeleech wedge strategy. A top-level policy: wedges
// are a finite, purchasable account resource, so their spend rules stand alongside
// accept and grab rather than nested inside either.
type WedgeConfig struct {
	Enable                bool
	Reserve               int      // keep at least this many wedges unspent
	ProactiveKinds        []string // always wedge these kinds above the proactive size
	ProactiveMinSizeBytes int64    // proactive wedge size threshold
	RescueEnable          bool     // wedge a grab that would otherwise be a buffer hold
	RescueMinSizeBytes    int64    // rescue wedge size threshold
}

// ShopConfig is the bonus-point spend policy: renew VIP or buy upload credit from
// surplus points above a reserve.
type ShopConfig struct {
	DryRun             bool     // decide + log only; buy nothing
	Reserved           int64    // absolute points floor, never spent
	VIPEnable          bool     // allow buying/renewing VIP
	VIPCost            int64    // points per VIP purchase
	VIPReserveMultiple int64    // hold this × VIPCost for renewals before buying upload
	VIPBuyableClasses  []string // account classes permitted to buy VIP; empty = never
	UploadEnable       bool     // allow buying upload credit
	UploadChunkGiB     int64    // GiB per upload purchase
	UploadPointsPerGiB int64    // points per GiB
}
