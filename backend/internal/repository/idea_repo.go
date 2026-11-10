package repository

import (
	"strings"

	"github.com/imloama/umore/internal/model"

	"gorm.io/gorm"
)

// 灵感的状态：待整理 → 已成项目 / 已归档
const (
	IdeaStatusInbox    = "inbox"
	IdeaStatusProject  = "project"
	IdeaStatusArchived = "archived"
)

// GetIdeas 灵感列表，新的在前；status 为空时不筛状态（归档 tab 之外的「全部」）
func (r *Repository) GetIdeas(userID int, status string) ([]model.Idea, error) {
	query := r.db.Where("user_id = ?", userID)
	if s := strings.TrimSpace(status); s != "" {
		query = query.Where("status = ?", s)
	}
	list := []model.Idea{}
	if err := query.Order("updated_at DESC, id DESC").Find(&list).Error; err != nil {
		return nil, err
	}
	if err := r.attachIdeaProjectNames(userID, list); err != nil {
		return nil, err
	}
	return list, nil
}

// GetIdea 取单条；不是本人的当作不存在
func (r *Repository) GetIdea(userID, id int) (*model.Idea, error) {
	var idea model.Idea
	err := r.db.Where("user_id = ? AND id = ?", userID, id).First(&idea).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	list := []model.Idea{idea}
	if err := r.attachIdeaProjectNames(userID, list); err != nil {
		return nil, err
	}
	return &list[0], nil
}

// CreateIdea 记一条灵感，状态默认待整理
func (r *Repository) CreateIdea(userID int, idea *model.Idea) error {
	normalizeIdea(idea)
	idea.UserID = userID
	idea.ID = 0
	return r.db.Create(idea).Error
}

// UpdateIdea 改内容或状态；ProjectID 单独走 SetIdeaProject，免得随手编辑把关联抹了
func (r *Repository) UpdateIdea(userID int, idea *model.Idea) error {
	normalizeIdea(idea)
	res := r.db.Model(&model.Idea{}).
		Where("user_id = ? AND id = ?", userID, idea.ID).
		Select("content", "status").Updates(idea)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// SetIdeaProject 把灵感挂到某个项目上（转项目成功时用）；projectID 传 nil 即解除关联
func (r *Repository) SetIdeaProject(userID, id int, projectID *int) error {
	if projectID != nil {
		if err := r.checkProjectOwnership(userID, projectID); err != nil {
			return err
		}
	}
	res := r.db.Model(&model.Idea{}).
		Where("user_id = ? AND id = ?", userID, id).
		Update("project_id", projectID)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// DeleteIdea 彻底删掉；不想再看到但不删的走归档
func (r *Repository) DeleteIdea(userID, id int) error {
	res := r.db.Where("user_id = ? AND id = ?", userID, id).Delete(&model.Idea{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// attachIdeaProjectNames 卡片上要显示「已成项目 · xxx」，名字一次查出来
func (r *Repository) attachIdeaProjectNames(userID int, list []model.Idea) error {
	ids := map[int]bool{}
	for _, idea := range list {
		if idea.ProjectID != nil {
			ids[*idea.ProjectID] = true
		}
	}
	if len(ids) == 0 {
		return nil
	}
	projectIDs := make([]int, 0, len(ids))
	for id := range ids {
		projectIDs = append(projectIDs, id)
	}
	var projects []model.Project
	if err := r.db.Select("id", "name").
		Where("user_id = ? AND id IN ?", userID, projectIDs).Find(&projects).Error; err != nil {
		return err
	}
	names := map[int]string{}
	for _, p := range projects {
		names[p.ID] = p.Name
	}
	for i := range list {
		if list[i].ProjectID != nil {
			list[i].ProjectName = names[*list[i].ProjectID]
		}
	}
	return nil
}

// normalizeIdea 补齐默认值：空状态算待整理，非法状态当待整理处理
func normalizeIdea(idea *model.Idea) {
	idea.Content = strings.TrimSpace(idea.Content)
	switch idea.Status {
	case IdeaStatusInbox, IdeaStatusProject, IdeaStatusArchived:
	default:
		idea.Status = IdeaStatusInbox
	}
}
