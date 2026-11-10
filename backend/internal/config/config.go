// Package config 读本地配置文件（JSON），放部署期才定的东西：
// 数据库、Redis、登录 token 有效期、Resend 发信参数。
// 文件路径由 UMORE_CONFIG 指定，默认 ./config.json；文件不存在时用默认值继续跑，
// 默认值就是「本地单机」：SQLite + 内存缓存。
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config 整份配置。
type Config struct {
	Port     int      `json:"port"`
	Database Database `json:"database"`
	Redis    Redis    `json:"redis"`
	Auth     Auth     `json:"auth"`
	Resend   Resend   `json:"resend"`
}

// 支持的数据库方言。SQLite 给本地开发与单机自用，Postgres 给正式环境。
const (
	DriverSQLite   = "sqlite"
	DriverPostgres = "postgres"
)

// Database 数据库连接。正式环境用 postgres，本地不填就走 SQLite 文件。
type Database struct {
	// Driver sqlite / postgres
	Driver string `json:"driver"`
	// DSN SQLite 是文件路径（相对 backend 目录），Postgres 是
	// postgres://user:pass@host:5432/umore?sslmode=disable
	DSN string `json:"dsn"`

	MaxOpenConns           int `json:"max_open_conns"`
	MaxIdleConns           int `json:"max_idle_conns"`
	ConnMaxLifetimeMinutes int `json:"conn_max_lifetime_minutes"`
}

// Redis 缓存。验证码、登录 token 只放这里，不落库。
// addr 为空时退化成进程内缓存，本地开发不用起 Redis。
type Redis struct {
	Addr               string `json:"addr"`
	Password           string `json:"password"`
	DB                 int    `json:"db"`
	Prefix             string `json:"prefix"`
	DialTimeoutSeconds int    `json:"dial_timeout_seconds"`
}

// Auth 登录态。
type Auth struct {
	// TokenTTLHours 登录 token 有效期，默认 168 小时（7 天）
	TokenTTLHours int `json:"token_ttl_hours"`
}

// Resend 用 Resend 发注册验证码所需的参数。
// 字段名照着 config.example.json 写，改配置不用翻代码。
type Resend struct {
	APIKey string `json:"api_key"` // resend.com/api-keys 里建
	// From 发件人，形如 "umore <no-reply@your-domain.com>"。
	// 用 onboarding@resend.dev 只能发到 Resend 账号自己的邮箱，正式使用要先验域名。
	From string `json:"from"`
	// LogOnly 只把验证码打到服务端日志，不真的发信，本地开发用
	LogOnly bool `json:"log_only"`

	CodeLength            int `json:"code_length"`             // 验证码位数，默认 6
	CodeTTLMinutes        int `json:"code_ttl_minutes"`        // 有效期，默认 10 分钟
	ResendIntervalSeconds int `json:"resend_interval_seconds"` // 同一邮箱重发间隔，默认 60 秒
	MaxAttempts           int `json:"max_attempts"`            // 单个验证码最多试错次数，默认 5
}

// Load 读配置文件；路径为空或文件不存在都返回默认配置，不当作错误。
func Load(path string) (Config, error) {
	cfg := Default()

	path = strings.TrimSpace(path)
	if path == "" {
		return cfg, nil
	}
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return cfg, fmt.Errorf("read config %s: %w", path, err)
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return cfg, fmt.Errorf("parse config %s: %w", path, err)
	}
	cfg.applyDefaults()
	return cfg, nil
}

// Default 默认配置：本地单机可用，正式环境要显式填 database / redis。
func Default() Config {
	return Config{
		Database: Database{
			Driver:                 DriverSQLite,
			MaxOpenConns:           10,
			MaxIdleConns:           5,
			ConnMaxLifetimeMinutes: 30,
		},
		Redis: Redis{Prefix: "umore", DialTimeoutSeconds: 3},
		Auth:  Auth{TokenTTLHours: 168},
		Resend: Resend{
			CodeLength:            6,
			CodeTTLMinutes:        10,
			ResendIntervalSeconds: 60,
			MaxAttempts:           5,
		},
	}
}

// applyDefaults 配置文件里漏写的字段（或写了 0）回到默认值，
// 免得少一行 code_ttl_minutes 就发出「立即过期」的验证码。
func (c *Config) applyDefaults() {
	d := Default()

	c.Database.Driver = normalizeDriver(c.Database.Driver)
	if c.Database.Driver == "" {
		c.Database.Driver = d.Database.Driver
	}
	if c.Database.MaxOpenConns <= 0 {
		c.Database.MaxOpenConns = d.Database.MaxOpenConns
	}
	if c.Database.MaxIdleConns <= 0 {
		c.Database.MaxIdleConns = d.Database.MaxIdleConns
	}
	if c.Database.ConnMaxLifetimeMinutes <= 0 {
		c.Database.ConnMaxLifetimeMinutes = d.Database.ConnMaxLifetimeMinutes
	}

	if strings.TrimSpace(c.Redis.Prefix) == "" {
		c.Redis.Prefix = d.Redis.Prefix
	}
	if c.Redis.DialTimeoutSeconds <= 0 {
		c.Redis.DialTimeoutSeconds = d.Redis.DialTimeoutSeconds
	}
	if c.Auth.TokenTTLHours <= 0 {
		c.Auth.TokenTTLHours = d.Auth.TokenTTLHours
	}

	r := &c.Resend
	dr := d.Resend
	if r.CodeLength <= 0 {
		r.CodeLength = dr.CodeLength
	}
	if r.CodeTTLMinutes <= 0 {
		r.CodeTTLMinutes = dr.CodeTTLMinutes
	}
	if r.ResendIntervalSeconds <= 0 {
		r.ResendIntervalSeconds = dr.ResendIntervalSeconds
	}
	if r.MaxAttempts <= 0 {
		r.MaxAttempts = dr.MaxAttempts
	}
}

// DriverValue 环境变量优先，容器里注入不用改文件
func (d Database) DriverValue() string {
	if v := normalizeDriver(os.Getenv("UMORE_DB_DRIVER")); v != "" {
		return v
	}
	if v := normalizeDriver(d.Driver); v != "" {
		return v
	}
	return DriverSQLite
}

// normalizeDriver 把常见写法收敛成两个值：sqlite / postgres。
// postgresql、pg 都当 postgres，省得因为写全称就启动失败。
func normalizeDriver(driver string) string {
	driver = strings.ToLower(strings.TrimSpace(driver))
	switch driver {
	case "":
		return ""
	case "sqlite", "sqlite3":
		return DriverSQLite
	case "postgres", "postgresql", "pg", "pgx":
		return DriverPostgres
	default:
		return driver
	}
}

// DSNValue 同上；SQLite 为空时由调用方拼默认路径
func (d Database) DSNValue() string {
	if v := strings.TrimSpace(os.Getenv("UMORE_DB_DSN")); v != "" {
		return v
	}
	return strings.TrimSpace(d.DSN)
}

// AddrValue Redis 地址，环境变量优先
func (r Redis) AddrValue() string {
	if v := strings.TrimSpace(os.Getenv("UMORE_REDIS_ADDR")); v != "" {
		return v
	}
	return strings.TrimSpace(r.Addr)
}

func (r Redis) PasswordValue() string {
	if v, ok := os.LookupEnv("UMORE_REDIS_PASSWORD"); ok {
		return v
	}
	return r.Password
}

func (r Redis) PrefixValue() string {
	if v := strings.TrimSpace(os.Getenv("UMORE_REDIS_PREFIX")); v != "" {
		return v
	}
	if strings.TrimSpace(r.Prefix) == "" {
		return "umore"
	}
	return strings.TrimSpace(r.Prefix)
}

func (r Redis) DBValue() int {
	if v := strings.TrimSpace(os.Getenv("UMORE_REDIS_DB")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return r.DB
}

// Configured 没配地址就退化成进程内缓存
func (r Redis) Configured() bool { return r.AddrValue() != "" }

// TokenTTL 登录 token 有效期
func (a Auth) TokenTTL() time.Duration {
	hours := a.TokenTTLHours
	if v := strings.TrimSpace(os.Getenv("UMORE_TOKEN_TTL_HOURS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			hours = n
		}
	}
	if hours <= 0 {
		hours = 168
	}
	return time.Duration(hours) * time.Hour
}

// APIKeyValue 环境变量优先，方便容器里注入而不用把 key 落盘
func (r Resend) APIKeyValue() string {
	if v := strings.TrimSpace(os.Getenv("RESEND_API_KEY")); v != "" {
		return v
	}
	return strings.TrimSpace(r.APIKey)
}

// FromValue 发件人同理，容器里只挂环境变量就够了，不用再维护一份配置文件
func (r Resend) FromValue() string {
	if v := strings.TrimSpace(os.Getenv("RESEND_FROM")); v != "" {
		return v
	}
	return strings.TrimSpace(r.From)
}

func (r Resend) CodeTTL() time.Duration {
	return time.Duration(r.CodeTTLMinutes) * time.Minute
}

func (r Resend) ResendInterval() time.Duration {
	return time.Duration(r.ResendIntervalSeconds) * time.Second
}

// Configured 判断能不能真的发信：LogOnly 视为可用（本地调试）
func (r Resend) Configured() bool {
	return r.LogOnly || (r.APIKeyValue() != "" && r.FromValue() != "")
}
