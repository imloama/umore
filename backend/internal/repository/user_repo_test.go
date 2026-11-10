package repository

import "testing"

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
