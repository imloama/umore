// Command migrate 是数据库迁移的独立入口，正式环境发布前单独跑一次。
//
//	go run ./cmd/migrate up       # 建表 / 补字段（服务启动时也会自动跑，这里用于发布前置）
//	go run ./cmd/migrate status   # 看当前库有哪些表、各多少行
//	go run ./cmd/migrate attachments --user 1           # 把旧的上传文件搬进按人隔离的目录
//	go run ./cmd/migrate attachments --user 1 --dry-run # 只报告，不动文件
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
	"github.com/imloama/umore/internal/migrate"
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

	uploadDir := envOr("UMORE_UPLOAD_DIR", filepath.Join(dataDir, "uploads"))

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
	case "attachments":
		// 老版本的上传是公开目录 + 没有归属人。新版要求按人隔离，
		// 所以得指定一个 user：这些历史文件算谁的（单机自用就是你自己的账号）。
		userID := intFlag(os.Args[2:], "--user", 0)
		if userID <= 0 {
			log.Fatal("必须带 --user <id>：老上传没有归属人，需要指明这些文件算谁的")
		}
		report, err := migrate.MigrateAttachments(repo, migrate.AttachmentOptions{
			UploadDir: uploadDir,
			UserID:    userID,
			DryRun:    hasFlag(os.Args[2:], "--dry-run"),
		})
		if err != nil {
			log.Fatalf("附件迁移失败: %v", err)
		}
		printAttachmentReport(report)
	default:
		usage()
		os.Exit(2)
	}
}

func printAttachmentReport(r *migrate.AttachmentReport) {
	prefix := ""
	if r.DryRun {
		prefix = "[dry-run] "
	}
	fmt.Printf("%s待处理旧文件 %d，已搬 %d，磁盘缺失 %d，正文改写 %d 处\n",
		prefix, r.Scanned, r.Moved, r.Missing, r.Rewritten)
	for _, d := range r.Details {
		fmt.Println("  " + d)
	}
	if len(r.Problems) > 0 {
		fmt.Printf("需要看一眼 %d 条：\n", len(r.Problems))
		for _, p := range r.Problems {
			fmt.Println("  ! " + p)
		}
	}
	if r.DryRun {
		fmt.Println("这是预演，没有改动任何文件。去掉 --dry-run 再跑一次就会真搬。")
	}
}

func hasFlag(args []string, name string) bool {
	for _, a := range args {
		if a == name {
			return true
		}
	}
	return false
}

// intFlag 读 --user 1 或 --user=1 两种写法
func intFlag(args []string, name string, fallback int) int {
	for i, a := range args {
		var raw string
		if a == name && i+1 < len(args) {
			raw = args[i+1]
		} else if strings.HasPrefix(a, name+"=") {
			raw = strings.TrimPrefix(a, name+"=")
		}
		if raw == "" {
			continue
		}
		var v int
		if _, err := fmt.Sscanf(raw, "%d", &v); err == nil {
			return v
		}
	}
	return fallback
}

func usage() {
	fmt.Fprintln(os.Stderr, `用法:
  migrate up                         建表 / 补字段
  migrate status                     看表与行数
  migrate attachments --user <id> [--dry-run]
                                     把旧版公开目录里的上传文件搬进按人隔离目录，
                                     并把正文里的 /uploads/ 地址换成 /api/attachments/<id>`)
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
