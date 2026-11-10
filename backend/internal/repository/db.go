package repository

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Options 打开数据库所需的全部参数，由 config 翻译过来，repository 不依赖 config。
type Options struct {
	Driver string // sqlite / postgres
	DSN    string // sqlite 是文件路径，postgres 是连接串

	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

const (
	SQLite   = "sqlite"
	Postgres = "postgres"
)

// Repository 数据访问层。
// 换数据库靠 gorm 的方言实现，业务代码里没有一行方言判断：
// 建表用 AutoMigrate，SQL 一律写 `?` 占位符，分页、排序、聚合都走 gorm。
type Repository struct {
	db *gorm.DB
}

// DB 暴露底层连接，给迁移 CLI 和测试用；业务代码不要绕过仓储层直接写表
func (r *Repository) DB() *gorm.DB { return r.db }

// NewRepository 打开一个 SQLite 库（本地开发与单机自用），也是测试的默认入口。
func NewRepository(dbPath string) (*Repository, error) {
	return Open(Options{Driver: SQLite, DSN: dbPath})
}

// Open 按方言打开数据库连接。正式环境走 Postgres，本地默认 SQLite 文件。
func Open(opt Options) (*Repository, error) {
	driver := strings.ToLower(strings.TrimSpace(opt.Driver))
	if driver == "" {
		driver = SQLite
	}

	var dialector gorm.Dialector
	switch driver {
	case SQLite:
		dsn := opt.DSN
		if err := os.MkdirAll(filepath.Dir(dsn), 0o755); err != nil {
			return nil, fmt.Errorf("failed to create db directory: %w", err)
		}
		// 纯 Go 驱动，免 cgo；顺带打开外键与忙等超时（并发写时别直接报 database is locked）
		dialector = sqlite.Open(withPragmas(dsn))
	case Postgres:
		if strings.TrimSpace(opt.DSN) == "" {
			return nil, fmt.Errorf("postgres dsn is required")
		}
		dialector = postgres.Open(opt.DSN)
	default:
		return nil, fmt.Errorf("unsupported database driver %q (want sqlite or postgres)", opt.Driver)
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		// 只记慢查询与错误；「查不到」是正常业务路径（越权访问走的就是这条），不当错误刷屏
		Logger: logger.New(log.New(os.Stderr, "", log.LstdFlags), logger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		}),
		// 表名用结构体名复数，跟老库的表名对得上（tasks / projects ...）
		NamingStrategy: gormNaming(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database handle: %w", err)
	}
	if opt.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(opt.MaxOpenConns)
	}
	if opt.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(opt.MaxIdleConns)
	}
	if opt.ConnMaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(opt.ConnMaxLifetime)
	}
	if err := sqlDB.Ping(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &Repository{db: db}, nil
}

// withPragmas SQLite 的连接参数：外键约束 + 5 秒忙等。
// 用户已经自己带参数时原样返回，不覆盖他的设置。
func withPragmas(dsn string) string {
	if strings.Contains(dsn, "_pragma=") {
		return dsn
	}
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	return dsn + sep + "_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
}

// Tables 当前库里有哪些表，迁移 CLI 的 status 用它看结果
func (r *Repository) Tables() ([]string, error) {
	migrator := r.db.Migrator()
	tables, err := migrator.GetTables()
	if err != nil {
		return nil, err
	}
	return tables, nil
}

// HasTable 某张表是否已建好
func (r *Repository) HasTable(name string) bool {
	return r.db.Migrator().HasTable(name)
}

func (r *Repository) Close() error {
	sqlDB, err := r.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
