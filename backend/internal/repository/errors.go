package repository

import (
	"errors"
	"strings"

	"gorm.io/gorm"
)

// isDuplicate 判断是不是唯一约束冲突。
// gorm 的 ErrDuplicatedKey 需要驱动支持翻译，SQLite 这边只能看错误文本，
// 两种都认，省得为了「邮箱已注册」这种提示写方言分支。
func isDuplicate(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint") ||
		strings.Contains(msg, "duplicate key") ||
		strings.Contains(msg, "unique failed")
}

// ErrProjectNameTaken 同一用户名下项目重名，上层据此返回 409
var ErrProjectNameTaken = errors.New("project name already exists")

// ErrLabelNameTaken 同一用户名下标签重名
var ErrLabelNameTaken = errors.New("label name already exists")

// ErrProjectNotFound 关联的项目不属于当前用户，或者根本不存在
var ErrProjectNotFound = errors.New("project not found")
