package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// 配置文件不存在也不该当错误，用默认值继续跑
func TestLoadMissingFile(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if cfg.Resend.CodeLength != 6 || cfg.Resend.CodeTTL() != 10*time.Minute {
		t.Fatalf("默认值不对: %+v", cfg.Resend)
	}
	if cfg.Resend.Configured() {
		t.Fatal("没有 key/from 时不该算已配置")
	}
}

// 漏写的数字字段回到默认值，避免 code_ttl_minutes 写 0 导致码立即过期
func TestLoadAppliesDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"resend":{"api_key":"re_x","from":"a <a@b.com>"}}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Resend.MaxAttempts != 5 || cfg.Resend.ResendIntervalSeconds != 60 {
		t.Fatalf("默认值没补上: %+v", cfg.Resend)
	}
	if !cfg.Resend.Configured() {
		t.Fatal("填了 key 和 from 应算已配置")
	}
}

// 环境变量优先于配置文件，容器里注入 key 不用改文件
func TestAPIKeyFromEnv(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"resend":{"api_key":"re_file","from":"a <a@b.com>"}}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	t.Setenv("RESEND_API_KEY", "re_env")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Resend.APIKeyValue() != "re_env" {
		t.Fatalf("环境变量应优先, got %q", cfg.Resend.APIKeyValue())
	}
}

// 数据库与 Redis 的默认值：本地就是 SQLite + 内存缓存
func TestDatabaseAndRedisDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Database.DriverValue() != DriverSQLite {
		t.Fatalf("默认方言应为 sqlite, got %q", cfg.Database.DriverValue())
	}
	if cfg.Redis.Configured() {
		t.Fatal("没配地址时不该算已配置（会走进程内缓存）")
	}
	if cfg.Auth.TokenTTL() != 168*time.Hour {
		t.Fatalf("token 默认有效期应为 7 天, got %v", cfg.Auth.TokenTTL())
	}
	if cfg.Database.MaxOpenConns == 0 || cfg.Redis.PrefixValue() != "umore" {
		t.Fatalf("连接池与前缀应有默认值: %+v %+v", cfg.Database, cfg.Redis)
	}
}

// 正式环境的配置能读进来，环境变量优先
func TestDatabaseAndRedisFromConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	body := `{
		"database": {"driver": "PostgreSQL", "dsn": "postgres://u:p@h:5432/db"},
		"redis": {"addr": "h:6379", "prefix": "prod"},
		"auth": {"token_ttl_hours": 2}
	}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Database.DriverValue() != DriverPostgres {
		t.Fatalf("driver 应归一化成小写, got %q", cfg.Database.DriverValue())
	}
	if !cfg.Redis.Configured() || cfg.Redis.PrefixValue() != "prod" {
		t.Fatalf("redis 配置没读到: %+v", cfg.Redis)
	}
	if cfg.Auth.TokenTTL() != 2*time.Hour {
		t.Fatalf("token 有效期应为 2 小时, got %v", cfg.Auth.TokenTTL())
	}

	t.Setenv("UMORE_DB_DRIVER", "sqlite")
	t.Setenv("UMORE_DB_DSN", "/tmp/x.db")
	t.Setenv("UMORE_REDIS_ADDR", "env:6380")
	if cfg.Database.DriverValue() != DriverSQLite || cfg.Database.DSNValue() != "/tmp/x.db" {
		t.Fatal("环境变量应优先于配置文件")
	}
	if cfg.Redis.AddrValue() != "env:6380" {
		t.Fatalf("redis 地址应取环境变量, got %q", cfg.Redis.AddrValue())
	}
}

// log_only 让本地开发不发真邮件
func TestLogOnlyConfigured(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	cfg.Resend.LogOnly = true
	if !cfg.Resend.Configured() {
		t.Fatal("log_only 时应视为可用")
	}
}
