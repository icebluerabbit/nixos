// Package types holds mousetrap's pure domain declarations — config shape, domain
// entities, and enums. No logic, no I/O, no pflag: it is a leaf that everything
// imports and that imports nothing. Keep it that way — behavior belongs elsewhere.
package types

import "time"

// Config is the fully-resolved runtime configuration. Package config populates it
// from command-line flags. Two axes: integrations (external systems mousetrap
// talks to — MAM, the download client, the webhook sink) and policy (the decision
// rules — accept, grab, wedge). Policy is client-agnostic; it never names qbt.
//
// The json tags are the configuration keys lib/config resolves, and each one is
// its flag's name with the dashes turned into underscores: --accept-size-min
// addresses accept_size_min. The domain groupings below are squashed rather than
// nested, because the flag surface is flat and predates the structs — a nested
// key would rename every flag the container passes.
type Config struct {
	// process
	Listen   string `json:"listen"`
	Port     string `json:"port"`
	StateDir string `json:"state_dir"`

	// integrations
	MAM     MAMConfig     `json:",squash"`
	Client  ClientConfig  `json:",squash"`
	Webhook WebhookConfig `json:",squash"`
	Torznab TorznabConfig `json:",squash"`

	// policy
	Accept AcceptConfig `json:",squash"`
	Grab   GrabConfig   `json:",squash"`
	Wedge  WedgeConfig  `json:",squash"`
	Shop   ShopConfig   `json:",squash"`
}

// Addr is the HTTP bind address.
func (c Config) Addr() string { return c.Listen + ":" + c.Port }

// MAMConfig is the MyAnonaMouse source client.
type MAMConfig struct {
	APIBase  string `json:"mam_api_base"`
	SeedPath string `json:"mam_seed_path"` // path to the mam_id seed secret (the path, never the value)
	// MediaTypeNames maps MAM's numeric search-API mediatype id to the string its
	// IRC announce uses, so both sources key policy on one vocabulary. Config, not
	// code: MAM ships ids we have no name for until one is announced, and an
	// unmapped id falls through as its own raw value — it matches no allowlist and
	// shows up untranslated in the metrics rather than being silently mislabeled.
	MediaTypeNames map[string]string `json:"mam_media_type_name"`
}

// ClientConfig is the download client (qBittorrent today). The domain is kept
// client-agnostic so a second client is a new Type + settings here, not a change
// to grab policy.
type ClientConfig struct {
	URL        string            `json:"client_url"`
	Categories map[string]string `json:"client_category"` // MAM media type -> client category
	AddPaused  bool              `json:"client_paused"`
}

// WebhookConfig configures an outbound notification POST. When URL is set, the
// rendered Template is POSTed on the triggering event with the given Headers.
// Template is a Go text/template evaluated over the event's data.
type WebhookConfig struct {
	URL      string            `json:"webhook_url"`
	Template string            `json:"webhook_template"`
	Headers  map[string]string `json:"webhook_header"`
	Events   []string          `json:"webhook_event"` // event types that fire; empty = none
}

// TorznabConfig is the Torznab indexer surface mousetrap exposes to library
// managers (Prowlarr/Readarr/LazyLibrarian). Search results carry download links
// pointing back here, so every grab still runs the full policy — mousetrap stays
// the sole MAM grab authority. Link metadata is carried plainly in the URL (trusted
// LAN, no signing); LinkTTL bounds how stale that metadata (freeleech/vip flags,
// size) may be by grab time.
type TorznabConfig struct {
	APIKey  string        `json:"torznab_apikey"`   // required apikey on requests; "" = no auth
	BaseURL string        `json:"torznab_base_url"` // absolute base for minted download links, e.g. http://mousetrap:8080
	LinkTTL time.Duration `json:"torznab_link_ttl"` // reject download links older than this; 0 = no expiry
}

// AcceptConfig is the Layer-1 acceptance policy — release-field filters only, no
// account state. Empty list / 0 / "" means the rule is off.
type AcceptConfig struct {
	MediaTypes       []string `json:"accept_media_type"`     // allowed MAM media types; empty = any
	CategoriesAllow  []string `json:"accept_category_allow"` // category allowlist, any-of; empty = any
	CategoriesDeny   []string `json:"accept_category_deny"`  // category denylist, any-of; takes precedence
	FiletypesAllow   []string `json:"accept_filetype_allow"` // filetype allowlist, any-of; empty = any
	FiletypesDeny    []string `json:"accept_filetype_deny"`  // filetype denylist, any-of; takes precedence
	FictionTypes     []string `json:"accept_fiction_type"`   // allowed fiction flags ("Fiction"/"Non-Fiction"); empty = any
	LanguagesAllow   []string `json:"accept_language"`       // language substrings, case-insensitive; empty = any
	SizeMinBytes     int64    `json:"accept_size_min"`       // reject below; 0 = no floor
	SizeMaxBytes     int64    `json:"accept_size_max"`       // reject above; 0 = no ceiling
	FreeleechOnly    bool     `json:"accept_freeleech_only"` // reject non-freeleech releases
	VIPOnly          bool     `json:"accept_vip_only"`       // reject non-VIP releases (raw release flag)
	ExcludeVIP       bool     `json:"accept_exclude_vip"`    // reject VIP releases (raw release flag)
	NameIncludeRegex string   `json:"accept_name_include"`   // require name to match (case-insensitive); "" = off
	NameExcludeRegex string   `json:"accept_name_exclude"`   // reject name matching (case-insensitive); "" = off
}

// GrabConfig is the Layer-2 grab-gate policy — pure account-state thresholds
// applied to Layer-1 survivors. Where/how a grab is delivered is ClientConfig.
type GrabConfig struct {
	UnsatMargin    int           `json:"grab_unsat_margin"`    // hold this many below the unsatisfied limit
	InFlightWindow time.Duration `json:"grab_inflight_window"` // how long a grab counts as in-flight
	BufferMinBytes int64         `json:"grab_buffer_min"`      // non-freeleech grabs require buffer >= this; 0 = off
	DryRun         bool          `json:"grab_dry_run"`         // decide and log only; grab nothing
}

// WedgeConfig is the Personal Freeleech wedge strategy. A top-level policy: wedges
// are a finite, purchasable account resource, so their spend rules stand alongside
// accept and grab rather than nested inside either.
type WedgeConfig struct {
	Enable                bool     `json:"wedge_enable"`
	Reserve               int      `json:"wedge_reserve"`              // keep at least this many wedges unspent
	ProactiveMediaTypes   []string `json:"wedge_proactive_media_type"` // always wedge these media types above the proactive size
	ProactiveMinSizeBytes int64    `json:"wedge_proactive_min_size"`   // proactive wedge size threshold
	// RequireWedge makes the proactive rule strict: a release big enough to deserve
	// a wedge is skipped outright when none is spendable, rather than falling through
	// to a full-ratio grab. Releases under the proactive threshold are unaffected and
	// still grab normally.
	RequireWedge       bool  `json:"wedge_require"`
	RescueEnable       bool  `json:"wedge_rescue"`          // wedge a grab that would otherwise be a buffer hold
	RescueMinSizeBytes int64 `json:"wedge_rescue_min_size"` // rescue wedge size threshold
}

// ShopConfig is the bonus-point spend policy: renew VIP or buy upload credit from
// surplus points above a reserve.
type ShopConfig struct {
	DryRun             bool     `json:"shop_dry_run"`               // decide + log only; buy nothing
	Reserved           int64    `json:"shop_reserved"`              // absolute points floor, never spent
	VIPEnable          bool     `json:"shop_vip_enable"`            // allow buying/renewing VIP
	VIPCost            int64    `json:"shop_vip_cost"`              // points per VIP purchase
	VIPReserveMultiple int64    `json:"shop_vip_reserve_multiple"`  // hold this × VIPCost for renewals before buying upload
	VIPBuyableClasses  []string `json:"shop_vip_buyable_class"`     // account classes permitted to buy VIP; empty = never
	UploadEnable       bool     `json:"shop_upload_enable"`         // allow buying upload credit
	UploadChunkGiB     int64    `json:"shop_upload_chunk_gib"`      // GiB per upload purchase
	UploadPointsPerGiB int64    `json:"shop_upload_points_per_gib"` // points per GiB
}
