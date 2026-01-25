package session

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/narasux/goblog/pkg/envs"
	"github.com/narasux/goblog/pkg/infras/database"
	"github.com/narasux/goblog/pkg/model"
	"github.com/narasux/goblog/pkg/utils/uuid"
)

var (
	// ErrSessionNotFound 会话不存在
	ErrSessionNotFound = errors.New("session not found")
	// ErrSessionExpired 会话已过期
	ErrSessionExpired = errors.New("session expired")
	// ErrUserNotFound 用户不存在
	ErrUserNotFound = errors.New("user not found")
	// ErrInvalidSessionSignature Session 签名无效
	ErrInvalidSessionSignature = errors.New("invalid session signature")
)

// GenerateSessionCookie 生成带签名的 Session Cookie 值
// 格式: sessionID|signature
func GenerateSessionCookie(sessionID string) string {
	signature := signSessionID(sessionID, envs.SessionSecret)
	return sessionID + "|" + signature
}

// ParseSessionCookie 解析并验证带签名的 Session Cookie 值
// 返回原始 Session ID，如果签名无效则返回错误
func ParseSessionCookie(cookieValue string) (string, error) {
	if cookieValue == "" {
		return "", ErrSessionNotFound
	}

	parts := strings.Split(cookieValue, "|")
	if len(parts) != 2 {
		return "", ErrInvalidSessionSignature
	}

	sessionID := parts[0]
	signature := parts[1]

	// 验证签名
	if !verifySessionID(sessionID, signature, envs.SessionSecret) {
		return "", ErrInvalidSessionSignature
	}

	return sessionID, nil
}

// signSessionID 使用 HMAC-SHA256 对 Session ID 进行签名
func signSessionID(sessionID, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(sessionID))
	return hex.EncodeToString(h.Sum(nil))
}

// verifySessionID 验证 Session ID 的签名
func verifySessionID(sessionID, signature, secret string) bool {
	expectedSignature := signSessionID(sessionID, secret)
	return hmac.Equal([]byte(signature), []byte(expectedSignature))
}

// Create 创建新会话
func Create(ctx context.Context, userID int64, ip, userAgent string) (*model.Session, error) {
	db := database.Client(ctx)

	session := &model.Session{
		ID:        uuid.GenUUID4(),
		UserID:    userID,
		ExpiresAt: time.Now().Add(time.Duration(envs.SessionMaxAge) * time.Second),
		IP:        ip,
		UserAgent: userAgent,
	}

	if err := db.Create(session).Error; err != nil {
		return nil, err
	}

	return session, nil
}

// Validate 验证会话有效性，返回用户信息
func Validate(ctx context.Context, sessionID string) (*model.User, error) {
	if sessionID == "" {
		return nil, ErrSessionNotFound
	}

	db := database.Client(ctx)

	var session model.Session
	if err := db.Where("id = ?", sessionID).First(&session).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}

	if session.IsExpired() {
		// 删除过期会话
		_ = db.Delete(&session).Error
		return nil, ErrSessionExpired
	}

	var user model.User
	if err := db.Where("id = ?", session.UserID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return &user, nil
}

// Delete 删除会话（登出）
func Delete(ctx context.Context, sessionID string) error {
	db := database.Client(ctx)
	return db.Where("id = ?", sessionID).Delete(&model.Session{}).Error
}
