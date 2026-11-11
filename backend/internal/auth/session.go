// Package auth 管登录态：签发 token、校验 token、退出登录。
// token 是随机会话号，对应的会话只放缓存（正式环境 Redis），不落数据库：
// 它本来就是「登录了多久」这种临时状态，过期由 TTL 收走，也不用拿表来堆垃圾。
package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"github.com/imloama/umore/internal/cache"
)

// ErrInvalidToken token 不认（不存在、已过期、已退出）
// 错误文本只用英文：面向用户的提示由前端按 handler 返回的错误码渲染，后端不做多语。
var ErrInvalidToken = errors.New("invalid or expired session")

// Session 一次登录会话。只存身份与签发时间，
// 加密密钥仍然由密码派生、只在前端手上一份，后端不留。
type Session struct {
	UserID    int    `json:"user_id"`
	Email     string `json:"email"`
	IssuedAt  int64  `json:"issued_at"`
	ExpiresAt int64  `json:"expires_at"`
}

// Manager 签发与校验 token
type Manager struct {
	Cache cache.Store
	TTL   time.Duration
	Now   func() time.Time
}

func NewManager(store cache.Store, ttl time.Duration) *Manager {
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	return &Manager{Cache: store, TTL: ttl, Now: time.Now}
}

func (m *Manager) now() time.Time {
	if m.Now != nil {
		return m.Now()
	}
	return time.Now()
}

// Issue 登录成功后签发 token。random 32 字节，库里/缓存里都不需要它的明文以外的信息。
func (m *Manager) Issue(ctx context.Context, userID int, email string) (string, time.Time, error) {
	token, err := randomToken()
	if err != nil {
		return "", time.Time{}, err
	}
	now := m.now()
	expiresAt := now.Add(m.TTL)
	sess := Session{
		UserID:    userID,
		Email:     email,
		IssuedAt:  now.Unix(),
		ExpiresAt: expiresAt.Unix(),
	}
	payload, err := json.Marshal(sess)
	if err != nil {
		return "", time.Time{}, err
	}
	if err := m.Cache.Set(ctx, sessionKey(token), payload, m.TTL); err != nil {
		return "", time.Time{}, err
	}
	return token, expiresAt, nil
}

// Lookup 校验 token 并取回会话
func (m *Manager) Lookup(ctx context.Context, token string) (*Session, error) {
	if token == "" {
		return nil, ErrInvalidToken
	}
	payload, ok, err := m.Cache.Get(ctx, sessionKey(token))
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrInvalidToken
	}
	var sess Session
	if err := json.Unmarshal(payload, &sess); err != nil {
		return nil, ErrInvalidToken
	}
	return &sess, nil
}

// Revoke 退出登录：删掉这条会话，token 立刻失效
func (m *Manager) Revoke(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return m.Cache.Del(ctx, sessionKey(token))
}

// sessionKey 前缀区分用途；token 自己是 32 字节随机串，撞车概率可忽略
func sessionKey(token string) string {
	return "session:" + token
}

func randomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
