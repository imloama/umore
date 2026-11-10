package repository

import (
	"errors"
	"time"

	"github.com/imloama/umore/internal/model"

	"gorm.io/gorm"
)

// 待收款要算「还剩几天 / 是否逾期」，只在读取时算，不落库，
// 以免日期一过就要跑一次批量更新。
func fillReceivableTiming(item *model.Receivable) {
	if item.DueDate == nil || *item.DueDate == "" {
		item.DaysLeft = 0
		item.Overdue = false
		return
	}
	// 按本地时区解析，否则 UTC 差 8 小时会把「今天到期」算成昨天
	due, err := time.ParseInLocation("2006-01-02", *item.DueDate, time.Local)
	if err != nil {
		item.DaysLeft = 0
		item.Overdue = false
		return
	}
	today := time.Now()
	todayDate := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.Local)
	item.DaysLeft = int(due.Sub(todayDate).Hours() / 24)
	item.Overdue = item.Status == "pending" && item.DaysLeft < 0
}

// GetReceivables 收款计划列表。status 为空时返回全部；pending 单按到期日升序（没填到期日的垫后）。
func (r *Repository) GetReceivables(userID int, status string, projectID int) ([]model.Receivable, error) {
	query := r.db.Where("user_id = ?", userID)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if projectID > 0 {
		query = query.Where("project_id = ?", projectID)
	}
	// 排序：先「未收」，未收里把没填到期日的垫后，其余按到期日升序
	query = query.Order("CASE WHEN status = 'pending' THEN 0 ELSE 1 END").
		Order("CASE WHEN COALESCE(due_date, '') = '' THEN 1 ELSE 0 END").
		Order("due_date").Order("id DESC")

	list := []model.Receivable{}
	if err := query.Find(&list).Error; err != nil {
		return nil, err
	}
	if err := r.attachReceivableProjectNames(userID, list); err != nil {
		return nil, err
	}
	for i := range list {
		fillReceivableTiming(&list[i])
	}
	return list, nil
}

// attachReceivableProjectNames 列表要显示项目名
func (r *Repository) attachReceivableProjectNames(userID int, list []model.Receivable) error {
	ids := map[int]bool{}
	for _, item := range list {
		if item.ProjectID != nil {
			ids[*item.ProjectID] = true
		}
	}
	if len(ids) == 0 {
		return nil
	}
	list2 := make([]int, 0, len(ids))
	for id := range ids {
		list2 = append(list2, id)
	}
	var projects []model.Project
	if err := r.db.Select("id", "name").Where("user_id = ? AND id IN ?", userID, list2).
		Find(&projects).Error; err != nil {
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

// GetReceivable 取单笔；不是自己的就当作不存在
func (r *Repository) GetReceivable(userID, id int) (*model.Receivable, error) {
	var item model.Receivable
	err := r.db.Where("user_id = ? AND id = ?", userID, id).First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	list := []model.Receivable{item}
	if err := r.attachReceivableProjectNames(userID, list); err != nil {
		return nil, err
	}
	fillReceivableTiming(&list[0])
	return &list[0], nil
}

// CreateReceivable 记一笔待收款
func (r *Repository) CreateReceivable(userID int, item *model.Receivable) error {
	if err := r.checkProjectOwnership(userID, item.ProjectID); err != nil {
		return err
	}
	if item.Status == "" {
		item.Status = "pending"
	}
	item.UserID = userID
	item.ID = 0
	return r.db.Create(item).Error
}

// UpdateReceivable 只更新「记一笔」里的那几个字段，状态另有专门入口
func (r *Repository) UpdateReceivable(userID int, item *model.Receivable) error {
	if err := r.checkProjectOwnership(userID, item.ProjectID); err != nil {
		return err
	}
	res := r.db.Model(&model.Receivable{}).Where("user_id = ? AND id = ?", userID, item.ID).
		Select("project_id", "title", "amount", "due_date", "note").Updates(item)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// SetReceivableStatus 标记已收 / 撤回，done_at 跟着走
func (r *Repository) SetReceivableStatus(userID, id int, done bool) error {
	updates := map[string]any{"status": "pending", "done_at": nil}
	if done {
		now := time.Now()
		updates["status"] = "done"
		updates["done_at"] = &now
	}
	res := r.db.Model(&model.Receivable{}).Where("user_id = ? AND id = ?", userID, id).Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// DeleteReceivable 删一笔待收款；不是自己的当作不存在
func (r *Repository) DeleteReceivable(userID, id int) error {
	res := r.db.Where("user_id = ? AND id = ?", userID, id).Delete(&model.Receivable{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// GetReceivableSummary 顶部三格：未收 / 逾期 / 7 天内到期，外加已收合计。
// 逾期与剩余天数是 Go 侧算的（见 fillReceivableTiming），这里只做朴素累加。
func (r *Repository) GetReceivableSummary(userID int) (*model.ReceivableSummary, error) {
	list, err := r.GetReceivables(userID, "", 0)
	if err != nil {
		return nil, err
	}

	sum := &model.ReceivableSummary{}
	for _, item := range list {
		if item.Status == "done" {
			sum.DoneCount++
			sum.DoneAmount = sum.DoneAmount.Add(item.Amount)
			continue
		}
		sum.PendingCount++
		sum.PendingAmount = sum.PendingAmount.Add(item.Amount)
		if item.Overdue {
			sum.OverdueCount++
			sum.OverdueAmount = sum.OverdueAmount.Add(item.Amount)
		} else if item.DueDate != nil && *item.DueDate != "" && item.DaysLeft <= 7 {
			sum.DueSoonCount++
			sum.DueSoonAmount = sum.DueSoonAmount.Add(item.Amount)
		}
	}
	return sum, nil
}
