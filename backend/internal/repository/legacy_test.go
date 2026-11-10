package repository

import (
	"path/filepath"
	"testing"

	"gorm.io/gorm"
)

// 老库里有 accounts 表、projects 还带 env_config/credentials：
// 这两块已经下线，Migrate 顺手把它们清掉，免得残留脏数据被误读。
func TestMigrateDropsLegacyTablesAndColumns(t *testing.T) {
	repo, err := NewRepository(filepath.Join(t.TempDir(), "legacy.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { repo.Close() })

	// 手工造一份旧结构
	stmts := []string{
		`CREATE TABLE accounts (id integer primary key, user_id integer, platform text, username text, password text, url text, notes text, created_at datetime, updated_at datetime)`,
		`CREATE TABLE projects (id integer primary key, user_id integer, name text, description text, git_repo text, customer text, env_config text, credentials text, status text, amount numeric(18,2), created_at datetime, updated_at datetime)`,
	}
	for _, s := range stmts {
		if err := repo.DB().Exec(s).Error; err != nil {
			t.Fatalf("prepare legacy schema: %v", err)
		}
	}
	if err := repo.DB().Exec(`INSERT INTO projects (user_id, name, env_config, credentials, status, amount) VALUES (1, '老项目', '{"a":1}', 'u:p', 'active', 1000)`).Error; err != nil {
		t.Fatalf("seed legacy row: %v", err)
	}

	if err := repo.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	m := repo.DB().Migrator()
	if m.HasTable("accounts") {
		t.Fatal("accounts 表应该被删掉")
	}
	for _, col := range []string{"env_config", "credentials"} {
		var count int64
		if err := repo.DB().Raw(
			`SELECT COUNT(*) FROM pragma_table_info('projects') WHERE name = ?`, col).Scan(&count).Error; err != nil {
			t.Fatalf("inspect columns: %v", err)
		}
		if count != 0 {
			t.Fatalf("projects.%s 应该被删掉", col)
		}
	}

	// 老数据本身要留住（只是敏感列没了）
	var name string
	if err := repo.DB().Raw(`SELECT name FROM projects LIMIT 1`).Scan(&name).Error; err != nil && err != gorm.ErrRecordNotFound {
		t.Fatalf("query project: %v", err)
	}
	if name != "老项目" {
		t.Fatalf("旧项目应保留，got %q", name)
	}
}
