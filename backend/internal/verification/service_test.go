package verification

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/imloama/umore/internal/cache"
	"github.com/imloama/umore/internal/config"
	"github.com/imloama/umore/internal/repository"
)

type fakeSender struct {
	code string
	to   string
	fail bool
}

func (f *fakeSender) SendCode(_ context.Context, to, code string) error {
	if f.fail {
		return errors.New("boom")
	}
	f.to, f.code = to, code
	return nil
}

func newService(t *testing.T) (*Service, *fakeSender, *repository.Repository, cache.Store) {
	t.Helper()
	repo, err := repository.NewRepository(filepath.Join(t.TempDir(), "v.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { repo.Close() })
	if err := repo.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	store := cache.NewMemory()
	t.Cleanup(func() { store.Close() })

	sender := &fakeSender{}
	cfg := config.Resend{APIKey: "re_test", From: "umore <no-reply@example.com>",
		CodeLength: 6, CodeTTLMinutes: 10, ResendIntervalSeconds: 60, MaxAttempts: 3}
	return New(repo, store, cfg, sender), sender, repo, store
}

func TestSendAndVerify(t *testing.T) {
	s, sender, _, _ := newService(t)

	if err := s.SendRegisterCode(context.Background(), " Max@Example.com "); err != nil {
		t.Fatalf("send: %v", err)
	}
	if sender.to != "max@example.com" {
		t.Fatalf("收件人应归一化为小写, got %q", sender.to)
	}
	if len(sender.code) != 6 {
		t.Fatalf("验证码应为 6 位, got %q", sender.code)
	}
	if err := s.VerifyRegisterCode(context.Background(), "max@example.com", sender.code); err != nil {
		t.Fatalf("verify: %v", err)
	}
	// 用过即废
	if err := s.VerifyRegisterCode(context.Background(), "max@example.com", sender.code); !errors.Is(err, ErrNoCode) {
		t.Fatalf("重放应报 ErrNoCode, got %v", err)
	}
}

// 验证码只进缓存，库里不该有任何痕迹
func TestCodeNotStoredInDatabase(t *testing.T) {
	s, sender, repo, store := newService(t)
	if err := s.SendRegisterCode(context.Background(), "max@example.com"); err != nil {
		t.Fatalf("send: %v", err)
	}

	payload, ok, err := store.Get(context.Background(), codeKey("max@example.com", PurposeRegister))
	if err != nil || !ok {
		t.Fatalf("缓存里应该有这条码: %v ok=%v", err, ok)
	}
	if string(payload) == sender.code {
		t.Fatal("缓存里不该出现明文验证码")
	}

	// 库里也不该有验证码表
	if repo.HasTable("email_verification_codes") {
		t.Fatal("验证码不该再占数据库表")
	}
}

func TestVerifyExpired(t *testing.T) {
	s, sender, _, _ := newService(t)
	now := time.Now()
	s.Now = func() time.Time { return now }

	if err := s.SendRegisterCode(context.Background(), "max@example.com"); err != nil {
		t.Fatalf("send: %v", err)
	}
	s.Now = func() time.Time { return now.Add(11 * time.Minute) }
	if err := s.VerifyRegisterCode(context.Background(), "max@example.com", sender.code); !errors.Is(err, ErrExpired) && !errors.Is(err, ErrNoCode) {
		t.Fatalf("过期应报 ErrExpired, got %v", err)
	}
}

func TestVerifyTooManyAttempts(t *testing.T) {
	s, _, _, _ := newService(t)
	if err := s.SendRegisterCode(context.Background(), "max@example.com"); err != nil {
		t.Fatalf("send: %v", err)
	}
	for i := 0; i < 2; i++ {
		if err := s.VerifyRegisterCode(context.Background(), "max@example.com", "000000"); !errors.Is(err, ErrMismatch) {
			t.Fatalf("第 %d 次错码应报 ErrMismatch, got %v", i+1, err)
		}
	}
	if err := s.VerifyRegisterCode(context.Background(), "max@example.com", "000000"); !errors.Is(err, ErrTooManyAttempts) {
		t.Fatalf("超上限应报 ErrTooManyAttempts, got %v", err)
	}
}

func TestResendInterval(t *testing.T) {
	s, _, _, _ := newService(t)
	if err := s.SendRegisterCode(context.Background(), "max@example.com"); err != nil {
		t.Fatalf("send: %v", err)
	}
	if err := s.SendRegisterCode(context.Background(), "max@example.com"); !errors.Is(err, ErrTooFrequent) {
		t.Fatalf("间隔内重发应报 ErrTooFrequent, got %v", err)
	}
}

// 新码覆盖旧码：只有最新那封邮件里的码有效
func TestNewCodeInvalidatesOld(t *testing.T) {
	s, sender, _, _ := newService(t)
	now := time.Now()
	s.Now = func() time.Time { return now }

	if err := s.SendRegisterCode(context.Background(), "max@example.com"); err != nil {
		t.Fatalf("send: %v", err)
	}
	old := sender.code
	s.Now = func() time.Time { return now.Add(2 * time.Minute) }
	if err := s.SendRegisterCode(context.Background(), "max@example.com"); err != nil {
		t.Fatalf("resend: %v", err)
	}
	if err := s.VerifyRegisterCode(context.Background(), "max@example.com", old); err == nil {
		t.Fatal("旧码应失效")
	}
	if err := s.VerifyRegisterCode(context.Background(), "max@example.com", sender.code); err != nil {
		t.Fatalf("新码应可用: %v", err)
	}
}

// 发信失败就把码清掉，别留下用户拿不到的码；冷却也一起清，允许立刻重发
func TestSendFailureDropsCode(t *testing.T) {
	s, sender, _, store := newService(t)
	sender.fail = true

	if err := s.SendRegisterCode(context.Background(), "max@example.com"); err == nil {
		t.Fatal("发信失败应返回错误")
	}
	if _, ok, _ := store.Get(context.Background(), codeKey("max@example.com", PurposeRegister)); ok {
		t.Fatal("发信失败应删掉这条码")
	}
	if _, ok, _ := store.Get(context.Background(), cooldownKey("max@example.com", PurposeRegister)); ok {
		t.Fatal("发信失败应清掉冷却，允许用户立刻重发")
	}
}

// 已注册的邮箱不发码
func TestSendRejectsExistingEmail(t *testing.T) {
	s, _, repo, _ := newService(t)
	if _, err := repo.CreateUser("max@example.com", "h", "s", true); err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := s.SendRegisterCode(context.Background(), "max@example.com"); !errors.Is(err, repository.ErrEmailTaken) {
		t.Fatalf("已注册邮箱应报 ErrEmailTaken, got %v", err)
	}
}

// 未配置发信时一律拒绝
func TestNotConfigured(t *testing.T) {
	repo, err := repository.NewRepository(filepath.Join(t.TempDir(), "n.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer repo.Close()
	s := New(repo, cache.NewMemory(), config.Resend{}, &fakeSender{})
	if s.Available() {
		t.Fatal("没配 key/from 时不应可用")
	}
	if err := s.SendRegisterCode(context.Background(), "max@example.com"); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("want ErrNotConfigured, got %v", err)
	}
}
