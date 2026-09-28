package database

import "time"

type User struct {
	ID          uint   `gorm:"primarykey"`
	Username    string `gorm:"uniqueIndex;not null"`
	Password    string `gorm:"not null"`
	Role        string `gorm:"default:user"`
	Status      int    `gorm:"default:1"`
	APIKey      string
	AuthVersion uint `gorm:"not null;default:1"`
	// FnOSUserID is the NAS-local UID from the fnOS unified gateway. It is a
	// binding identifier, never an API token or credential.
	FnOSUserID   *uint `gorm:"uniqueIndex"`
	FnOSUsername string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// UserSession 记录「用户在应用里主动退出登录」这件事，而不是记录登录本身。
//
// 为什么需要它：飞牛网关域上服务端每个请求都能从 X-Trim-Userid 认出 NAS 用户
// 对应的应用账号（隐式登录态）。如果用户点了「退出登录」，光清前端的令牌没有
// 用——下一个请求服务端又把他认回来，用户看到的就是「飞牛登录的应用退不出去」。
// Suppressed 为 true 表示该用户的网关隐式登录被抑制，直到他显式登录。
type UserSession struct {
	UserID uint `gorm:"primarykey"`
	// Suppressed 为 true 表示该用户的网关隐式登录被抑制，直到他显式登录。
	Suppressed bool
	// SuppressedAt 是抑制发生的时间，仅用于排查。
	SuppressedAt time.Time
	UpdatedAt    time.Time
}

type SystemConfig struct {
	ID        uint   `gorm:"primarykey"`
	UserID    uint   `gorm:"default:0;uniqueIndex:idx_user_key"`
	Key       string `gorm:"not null;uniqueIndex:idx_user_key"`
	Value     string
	Public    bool `gorm:"default:false"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

type UpgradeRecord struct {
	ID         uint   `gorm:"primarykey"`
	Version    string `gorm:"not null;index"`
	Name       string
	UpgradedAt time.Time
}

// AuditLog records a security- or admin-relevant action for later review.
type AuditLog struct {
	ID         uint   `gorm:"primarykey"`
	UserID     uint   `gorm:"index"`
	Username   string `gorm:"index"`
	Action     string `gorm:"index;not null"`
	TargetType string `gorm:"index"`
	TargetID   uint
	Detail     string
	IP         string
	CreatedAt  time.Time `gorm:"index"`
}

type SecurityQuestion struct {
	ID        uint `gorm:"primarykey"`
	UserID    uint `gorm:"not null;uniqueIndex"`
	Question1 string
	Answer1   string
	Question2 string
	Answer2   string
	Question3 string
	Answer3   string
	CreatedAt time.Time
	UpdatedAt time.Time
}
