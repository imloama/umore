package repository

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

)

// newTestRepo 本地 SQLite 库，每个用例一个临时文件，建表走 AutoMigrate。
func newTestRepo(t *testing.T) *Repository {
	t.Helper()
	repo, err := NewRepository(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { repo.Close() })
	if err := repo.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return repo
}

// newPostgresRepo PG 集成测试：给了 UMORE_TEST_POSTGRES_DSN 才跑。
// 每次用独立 schema，跑完 drop，互不干扰，也不碰手头的库。
//
//	UMORE_TEST_POSTGRES_DSN='postgres://postgres:pw@127.0.0.1:5432/umore?sslmode=disable' go test ./internal/repository/
func newPostgresRepo(t *testing.T) *Repository {
	t.Helper()
	dsn := os.Getenv("UMORE_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("UMORE_TEST_POSTGRES_DSN 未设置，跳过 PostgreSQL 集成测试")
	}

	schema := fmt.Sprintf("umore_test_%d", time.Now().UnixNano())

	// 先建 schema，再用带 search_path 的连接串连库：
	// SET search_path 只对单个连接生效，连接池换连接就丢了，写进 DSN 才稳。
	admin, err := Open(Options{Driver: Postgres, DSN: dsn})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	if err := admin.DB().Exec("CREATE SCHEMA " + schema).Error; err != nil {
		admin.Close()
		t.Fatalf("create schema: %v", err)
	}
	admin.Close()

	repo, err := Open(Options{Driver: Postgres, DSN: dsn + "&search_path=" + schema})
	if err != nil {
		t.Fatalf("open postgres schema: %v", err)
	}
	t.Cleanup(func() {
		repo.Close()
		cleaner, err := Open(Options{Driver: Postgres, DSN: dsn})
		if err != nil {
			return
		}
		defer cleaner.Close()
		cleaner.DB().Exec("DROP SCHEMA IF EXISTS " + schema + " CASCADE")
	})

	if err := repo.Migrate(); err != nil {
		t.Fatalf("migrate postgres: %v", err)
	}
	return repo
}

// newUser 造一个账号，返回它的 id —— 业务表的 user_id 就是它
func newUser(t *testing.T, repo *Repository, email string) int {
	t.Helper()
	user, err := repo.CreateUser(email, "hash", "salt", true)
	if err != nil {
		t.Fatalf("create user %s: %v", email, err)
	}
	return user.ID
}

// 每种数据库都要跑一遍的场景，用同一份用例验证「换库不改行为」
func eachRepo(t *testing.T, run func(t *testing.T, repo *Repository)) {
	t.Run("sqlite", func(t *testing.T) { run(t, newTestRepo(t)) })
	t.Run("postgres", func(t *testing.T) { run(t, newPostgresRepo(t)) })
}
