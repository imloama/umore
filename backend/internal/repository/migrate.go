package repository

import (
	"fmt"
	"log"

	"github.com/imloama/umore/internal/model"
)

// Migrate 建表 / 补字段，服务启动与 migrate CLI 共用。
//
// 走 gorm 的 AutoMigrate：表定义只有 model 包一份，换库不需要另一套 DDL。
// 这个项目不做历史数据兼容——新库直接建全，业务表都带 user_id。
func (r *Repository) Migrate() error {
	if err := r.db.AutoMigrate(model.All()...); err != nil {
		return fmt.Errorf("auto migrate: %w", err)
	}
	// 昵称是后加的字段：老账号按 MORE+ID 补一条，界面不出现空名字
	if err := r.BackfillNicknames(); err != nil {
		return fmt.Errorf("backfill nickname: %w", err)
	}
	r.dropLegacy()
	return nil
}

// dropLegacy 清掉已经下线的表与列：账户模块没了，项目也不再存环境/凭证，知识库换成了灵感。
// AutoMigrate 只加不删，所以这步得自己来。删不掉只记日志，不拦着服务启动。
func (r *Repository) dropLegacy() {
	m := r.db.Migrator()
	// 知识库下线：链接/文本收藏这套不放着了，换成灵感（ideas）卡片。
	// 老数据直接删——表结构跟灵感对不上，留着也读不回来。
	for _, table := range []string{"accounts", "knowledge"} {
		if !m.HasTable(table) {
			continue
		}
		if err := m.DropTable(table); err != nil {
			log.Printf("WARN: 删除旧表 %s 失败: %v", table, err)
		}
	}
	// HasColumn 是按结构体字段找列的，这两个字段已经从模型里删了，所以直接查库里的实际列
	cols, err := m.ColumnTypes(&model.Project{})
	if err != nil {
		log.Printf("WARN: 读取 projects 列信息失败: %v", err)
		return
	}
	existing := map[string]bool{}
	for _, c := range cols {
		existing[c.Name()] = true
	}
	// 直接发 DDL：gorm 的 DropColumn 要拿结构体字段来重建表，字段已经删了就无从下手
	for _, col := range []string{"env_config", "credentials"} {
		if !existing[col] {
			continue
		}
		if err := r.db.Exec("ALTER TABLE projects DROP COLUMN " + col).Error; err != nil {
			log.Printf("WARN: 删除旧列 projects.%s 失败: %v", col, err)
		}
	}
}

// CountRows 迁移 CLI 的 status 用来展示每张表当前多少行
func (r *Repository) CountRows(table string) (int64, error) {
	var count int64
	if err := r.db.Table(table).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}
