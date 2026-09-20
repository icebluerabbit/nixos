package model

// SoulseekPeer is another Soulseek user.
type SoulseekPeer struct {
	Username string `gorm:"primaryKey"`
}

func (SoulseekPeer) TableName() string { return "soulseek_peers" }
