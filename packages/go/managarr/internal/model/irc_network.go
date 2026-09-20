package model

// IrcNetwork is an IRC network autobrr connects to for announces.
type IrcNetwork struct {
	Host string `gorm:"primaryKey"`

	Name    *string
	Port    *int
	TLS     *bool
	Nick    *string
	Enabled *bool
}

func (IrcNetwork) TableName() string { return "irc_networks" }
