package repository

import (
	"errors"

	"github.com/imloama/umore/internal/model"

	"gorm.io/gorm"
)

// CreateAttachment 记一条附件元信息，文件本体在 data/uploads 下
func (r *Repository) CreateAttachment(userID int, a *model.Attachment) error {
	a.UserID = userID
	a.ID = 0
	return r.db.Create(a).Error
}

// GetAttachment 按 id 取附件。附件只属于上传它的人：别人的附件一律当作不存在，
// 路由层据此回 404，而不是把图发给对方。
func (r *Repository) GetAttachment(userID, id int) (*model.Attachment, error) {
	var a model.Attachment
	err := r.db.Where("user_id = ? AND id = ?", userID, id).First(&a).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}
