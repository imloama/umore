package repository

import (
	"time"

	"github.com/imloama/umore/internal/model"

	"gorm.io/gorm"
)

// GetLabels 标签字典，带该用户下的任务计数
func (r *Repository) GetLabels(userID int) ([]model.Label, error) {
	labels := []model.Label{}
	if err := r.db.Where("user_id = ?", userID).Order("name").Find(&labels).Error; err != nil {
		return nil, err
	}
	if len(labels) == 0 {
		return labels, nil
	}

	type countRow struct {
		LabelID int
		Total   int
	}
	var counts []countRow
	if err := r.db.Table("task_labels").
		Joins("JOIN tasks ON tasks.id = task_labels.task_id").
		Where("tasks.user_id = ?", userID).
		Select("task_labels.label_id, COUNT(*) AS total").
		Group("task_labels.label_id").Scan(&counts).Error; err != nil {
		return nil, err
	}
	byLabel := map[int]int{}
	for _, c := range counts {
		byLabel[c.LabelID] = c.Total
	}
	for i := range labels {
		labels[i].TaskCount = byLabel[labels[i].ID]
	}
	return labels, nil
}

// CreateLabel 建标签；同名已存在时返回已有的那条，不报错
func (r *Repository) CreateLabel(userID int, l *model.Label) error {
	l.UserID = userID
	l.ID = 0
	if l.Color == "" {
		l.Color = defaultLabelColor("")
	}
	existing, err := r.findLabelByName(userID, l.Name)
	if err != nil {
		return err
	}
	if existing != nil {
		*l = *existing
		return nil
	}
	return r.db.Create(l).Error
}

func (r *Repository) findLabelByName(userID int, name string) (*model.Label, error) {
	var label model.Label
	err := r.db.Where("user_id = ? AND name = ?", userID, name).First(&label).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &label, nil
}

// UpdateLabel 改标签名与颜色
func (r *Repository) UpdateLabel(userID int, l *model.Label) error {
	res := r.db.Model(&model.Label{}).Where("user_id = ? AND id = ?", userID, l.ID).
		Select("name", "color").Updates(l)
	if res.Error != nil {
		if isDuplicate(res.Error) {
			return ErrLabelNameTaken
		}
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// DeleteLabel 删标签，关联表跟着清；不是自己的当作不存在
func (r *Repository) DeleteLabel(userID, id int) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// 先清关联再删标签，理由同 DeleteTask：task_labels 上有外键
		if err := tx.Where("label_id IN (?)",
			tx.Model(&model.Label{}).Select("id").Where("user_id = ? AND id = ?", userID, id)).
			Delete(&taskLabel{}).Error; err != nil {
			return err
		}
		res := tx.Where("user_id = ? AND id = ?", userID, id).Delete(&model.Label{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

// GetComments 回复列表
func (r *Repository) GetComments(userID int, targetType string, targetID int) ([]model.Comment, error) {
	comments := []model.Comment{}
	err := r.db.Where("user_id = ? AND target_type = ? AND target_id = ?", userID, targetType, targetID).
		Order("created_at, id").Find(&comments).Error
	if err != nil {
		return nil, err
	}
	for i := range comments {
		comments[i].ContentHTML = model.RenderMarkdown(comments[i].Content)
	}
	return comments, nil
}

// CreateComment 写一条回复
func (r *Repository) CreateComment(userID int, c *model.Comment) error {
	c.UserID = userID
	c.ID = 0
	if err := r.db.Create(c).Error; err != nil {
		return err
	}
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now()
	}
	c.ContentHTML = model.RenderMarkdown(c.Content)
	return nil
}

// DeleteComment 删一条回复，只能删自己数据下的
func (r *Repository) DeleteComment(userID, id int) error {
	res := r.db.Where("user_id = ? AND id = ?", userID, id).Delete(&model.Comment{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
