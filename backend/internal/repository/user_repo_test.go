package repository

import (
	"testing"

	"github.com/imloama/umore/internal/model"
)

// 账号：建、按邮箱查大小写不敏感、重复邮箱被挡
func TestUserRepo(t *testing.T) {
	repo := newTestRepo(t)

	user, err := repo.CreateUser("Max@Example.com", "hash", "salt", true)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if user.Email != "max@example.com" {
		t.Fatalf("邮箱应归一化为小写: %q", user.Email)
	}
	if !user.EmailVerified {
		t.Fatal("注册即已验证，落库就该是 true")
	}

	got, err := repo.GetUserByEmail("  MAX@example.COM ")
	if err != nil || got == nil || got.ID != user.ID {
		t.Fatalf("按邮箱查不到: %v", err)
	}

	byID, err := repo.GetUser(user.ID)
	if err != nil || byID == nil || byID.PasswordHash != "hash" {
		t.Fatalf("按 id 查不到: %v", err)
	}

	if _, err := repo.GetUserByEmail("nobody@example.com"); err != nil {
		t.Fatalf("查不存在的邮箱不该报错: %v", err)
	}
}

// 迁移补时区：直接写空的进去，BackfillTimezones 之后回到默认值
func TestBackfillTimezones(t *testing.T) {
	repo := newTestRepo(t)
	uid := newUser(t, repo, "tz-backfill@example.com")
	if err := repo.db.Model(&model.User{}).Where("id = ?", uid).Update("timezone", "").Error; err != nil {
		t.Fatalf("clear timezone: %v", err)
	}
	if err := repo.BackfillTimezones(); err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if got := repo.TimezoneFor(uid); got != model.DefaultTimezone {
		t.Fatalf("补完之后应为 %s，实际 %s", model.DefaultTimezone, got)
	}
}
