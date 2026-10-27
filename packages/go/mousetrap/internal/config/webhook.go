package config

import (
	flag "github.com/spf13/pflag"
)

// registerWebhook declares the outbound-notification flags. Empty URL disables it.
func registerWebhook(fs *flag.FlagSet) {
	fs.String("webhook-url", "", "POST notifications to this URL; empty = disabled")
	fs.String("webhook-template", "", "Go text/template rendered as the POST body ({{.Type}} + {{.Payload}}; toJson/toRawJson available)")
	fs.StringToString("webhook-header", nil, "request header key=value (repeatable)")
	fs.StringArray("webhook-event", nil, "event type to fire (repeatable): announce|grab|reject|hold|enqueue|bonus_received|spend; empty = none")
}
