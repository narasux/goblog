package model

import "time"

// User 用户信息（通过 GitHub OAuth 登录）
type User struct {
	BaseModel
	ID          int64  `json:"id" gorm:"primaryKey"`
	GithubID    int64  `json:"githubID" gorm:"uniqueIndex;not null"`      // GitHub 用户 ID
	Username    string `json:"username" gorm:"type:varchar(64);not null"` // GitHub 用户名
	Email       string `json:"email" gorm:"type:varchar(128)"`            // GitHub 邮箱
	AvatarURL   string `json:"avatarURL" gorm:"type:varchar(256)"`        // GitHub 头像
	AccessToken string `json:"-" gorm:"type:varchar(256)"`                // GitHub Access Token（不暴露给前端）
}

// Session 用户会话
type Session struct {
	BaseModel
	ID        string    `json:"id" gorm:"primaryKey;type:varchar(64)"` // Session ID (UUID)
	UserID    int64     `json:"userID" gorm:"index;not null"`          // 关联用户
	ExpiresAt time.Time `json:"expiresAt" gorm:"not null"`             // 过期时间
	IP        string    `json:"ip" gorm:"type:varchar(64)"`            // 登录 IP
	UserAgent string    `json:"userAgent" gorm:"type:varchar(512)"`    // 浏览器 UA
}

// IsExpired 检查会话是否过期
func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}
