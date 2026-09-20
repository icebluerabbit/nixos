package model

import "time"

// Channel kinds. Only announce is ever written today.
const (
	ChannelAnnounce = "announce"
	ChannelBrag     = "brag"
)

// IrcChannel is a channel autobrr has joined.
//
// Kind is this model's concept, not autobrr's: autobrr joins a channel without
// classifying it, so everything it monitors is recorded as an announce channel.
// Brag channels exist on these networks but autobrr does not model them, so
// they stay unrepresented rather than guessed from the channel name.
type IrcChannel struct {
	NetworkHost string `gorm:"primaryKey"`
	Name        string `gorm:"primaryKey"`

	Kind           string `gorm:"not null;default:announce"`
	Enabled        *bool
	Monitoring     *bool
	LastAnnounceAt *time.Time

	Network *IrcNetwork `gorm:"foreignKey:NetworkHost;references:Host;constraint:OnDelete:CASCADE"`
}

func (IrcChannel) TableName() string { return "irc_channels" }
