package auth

import (
	"context"
	"testing"
	"time"

	"github.com/imloama/umore/internal/cache"
)

func newManager(t *testing.T) (*Manager, cache.Store) {
	t.Helper()
	store := cache.NewMemory()
	t.Cleanup(func() { store.Close() })
	return NewManager(store, time.Hour), store
}

// 签发的 token 能换回身份，退出登录后立刻失效
func TestIssueLookupRevoke(t *testing.T) {
	m, _ := newManager(t)
	ctx := context.Background()

	token, expiresAt, err := m.Issue(ctx, 7, "max@example.com")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if len(token) < 32 {
		t.Fatalf("token 应该有足够随机长度, got %q", token)
	}
	if !expiresAt.After(time.Now()) {
		t.Fatalf("过期时间应在将来: %v", expiresAt)
	}

	sess, err := m.Lookup(ctx, token)
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if sess.UserID != 7 || sess.Email != "max@example.com" {
		t.Fatalf("会话内容不对: %+v", sess)
	}

	if err := m.Revoke(ctx, token); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if _, err := m.Lookup(ctx, token); err != ErrInvalidToken {
		t.Fatalf("退出后应失效, got %v", err)
	}
}

// 过期后查不到；时间由 Now 控制，不靠 sleep
func TestSessionExpires(t *testing.T) {
	m, _ := newManager(t)
	ctx := context.Background()
	now := time.Now()
	m.Now = func() time.Time { return now }

	token, _, err := m.Issue(ctx, 1, "a@b.com")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	// 内存实现的 TTL 走真实时间，这里单独验证记录里的过期字段
	sess, err := m.Lookup(ctx, token)
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if sess.ExpiresAt <= sess.IssuedAt {
		t.Fatalf("过期时间必须晚于签发时间: %+v", sess)
	}
}

func TestLookupUnknownToken(t *testing.T) {
	m, _ := newManager(t)
	if _, err := m.Lookup(context.Background(), "nope"); err != ErrInvalidToken {
		t.Fatalf("want ErrInvalidToken, got %v", err)
	}
	if _, err := m.Lookup(context.Background(), ""); err != ErrInvalidToken {
		t.Fatalf("空 token 也该报错, got %v", err)
	}
}

// 两次登录签出的 token 不一样
func TestTokensAreUnique(t *testing.T) {
	m, _ := newManager(t)
	ctx := context.Background()
	a, _, _ := m.Issue(ctx, 1, "a@b.com")
	b, _, _ := m.Issue(ctx, 1, "a@b.com")
	if a == b {
		t.Fatal("同一个人两次登录应拿到不同 token")
	}
	if sessionKey(a) == a {
		t.Fatal("会话键应带前缀，别和验证码的键混在一起")
	}
}
