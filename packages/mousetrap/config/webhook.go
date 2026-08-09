package config

import (
	flag "github.com/spf13/pflag"

	"mousetrap/types"
)

// registerWebhook binds the outbound-notification flags. Empty URL disables it.
func registerWebhook(fs *flag.FlagSet, c *types.WebhookConfig) {
	fs.StringVar(&c.URL, "webhook-url", "", "POST notifications to this URL; empty = disabled")
	fs.StringVar(&c.Template, "webhook-template", "", "Go text/template rendered as the POST body ({{.Type}} + {{.Payload}}; toJson/toRawJson available)")
	fs.StringToStringVar(&c.Headers, "webhook-header", nil, "request header key=value (repeatable)")
	fs.StringArrayVar(&c.Events, "webhook-event", nil, "event type to fire (repeatable): announce|grab|reject|hold|enqueue|bonus_received|spend; empty = none")
}
