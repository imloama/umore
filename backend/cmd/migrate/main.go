// Command migrate 是数据库迁移的独立入口，正式环境发布前单独跑一次。
//
//	go run ./cmd/migrate up       # 建表 / 补字段（服务启动时也会自动跑，这里用于发布前置）
//	go run ./cmd/migrate status   # 看当前库有哪些表、各多少行
//
// 配置与服务共用 UMORE_CONFIG，所以不用重复填连接串。
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/imloama/umore/internal/config"
	"github.com/imloama/umore/internal/repository"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	dataDir := envOr("UMORE_DATA_DIR", filepath.Join(".", "data"))
	cfgPath := envOr("UMORE_CONFIG", filepath.Join(".", "config.json"))
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("加载配置 %s 失败: %v", cfgPath, err)
	}

	driver := cfg.Database.DriverValue()
	dsn := cfg.Database.DSNValue()
	if driver == config.DriverSQLite && strings.TrimSpace(dsn) == "" {
		dsn = filepath.Join(dataDir, "umore.db")
	}

	repo, err := repository.Open(repository.Options{
		Driver:       driver,
		DSN:          dsn,
		MaxOpenConns: cfg.Database.MaxOpenConns,
		MaxIdleConns: cfg.Database.MaxIdleConns,
	})
	if err != nil {
		log.Fatalf("连接数据库失败: %v", err)
	}
	defer repo.Close()

	switch os.Args[1] {
	case "up":
		if err := repo.Migrate(); err != nil {
			log.Fatalf("迁移失败: %v", err)
		}
		tables, err := repo.Tables()
		if err != nil {
			log.Fatalf("读取表列表失败: %v", err)
		}
		fmt.Printf("迁移完成，共 %d 张表\n", len(tables))
	case "status":
		tables, err := repo.Tables()
		if err != nil {
			log.Fatalf("读取表列表失败: %v", err)
		}
		fmt.Printf("数据库: %s\n", driver)
		for _, table := range tables {
			count, err := repo.CountRows(table)
			if err != nil {
				fmt.Printf("  %-24s ?\n", table)
				continue
			}
			fmt.Printf("  %-24s %d 行\n", table, count)
		}
		fmt.Printf("共 %d 张表\n", len(tables))
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "用法: migrate <up|status>")
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
