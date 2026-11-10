package repository

import "gorm.io/gorm/schema"

// gormNaming 表名保持「结构体名小写复数」，与老库的表名一致（tasks / projects / comments）。
func gormNaming() schema.NamingStrategy {
	return schema.NamingStrategy{SingularTable: false}
}
