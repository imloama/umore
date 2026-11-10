package repository

import "github.com/imloama/umore/internal/model"

// CreateAttachment 记一条附件元信息，文件本体在 data/uploads 下
func (r *Repository) CreateAttachment(userID int, a *model.Attachment) error {
	a.UserID = userID
	a.ID = 0
	return r.db.Create(a).Error
}
