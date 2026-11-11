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

// UpdateReceivable 只更新「记一笔」里的那几个字段，状态另有专门入口。
// 已收的那笔改了金额/项目，跟着的流水也要改，不然两边对不上。
func (r *Repository) UpdateReceivable(userID int, item *model.Receivable) error {
	if err := r.checkProjectOwnership(userID, item.ProjectID); err != nil {
		return err
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.Receivable{}).Where("user_id = ? AND id = ?", userID, item.ID).
			Select("project_id", "title", "amount", "due_date", "note").Updates(item)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return tx.Model(&model.Finance{}).Where("user_id = ? AND receivable_id = ?", userID, item.ID).
			Updates(map[string]any{"amount": item.Amount, "category": item.Title, "project_id": item.ProjectID}).Error
	})
}

// SetReceivableStatus 标记已收 / 撤回，done_at 跟着走。
//
// 「已收」不只是改个状态：标记已收同时落一笔收入流水（project 跟着收款计划走），
// 撤回就把它删掉。项目的「已收款」、财务页的「总收入」都是按 income 流水汇总的，
// 只改状态不动流水，界面上就永远是 0（见 issue #28）。
func (r *Repository) SetReceivableStatus(userID, id int, done bool) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var item model.Receivable
		err := tx.Where("user_id = ? AND id = ?", userID, id).First(&item).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return gorm.ErrRecordNotFound
		}
		if err != nil {
			return err
		}

		updates := map[string]any{"status": "pending", "done_at": nil}
		if done {
			now := time.Now()
			updates["status"] = "done"
			updates["done_at"] = &now
		}
		if err := tx.Model(&model.Receivable{}).Where("user_id = ? AND id = ?", userID, id).
			Updates(updates).Error; err != nil {
			return err
		}

		if done {
			if item.Status == "done" {
				// 重复点「已收」：流水已经在，别再加一笔
				return nil
			}
			return ensureReceivableIncome(tx, userID, item)
		}
		// 撤回：只删这笔收款计划记的那条流水，手记的收入不动
		return tx.Where("user_id = ? AND receivable_id = ?", userID, id).
			Delete(&model.Finance{}).Error
	})
}

// ensureReceivableIncome 给「已收」的收款计划补一条收入流水。
// 收款计划本身就是「哪笔钱到账了」，收入流水的日期取收到那天（本次 done_at）。
func ensureReceivableIncome(tx *gorm.DB, userID int, item model.Receivable) error {
	var existing int64
	if err := tx.Model(&model.Finance{}).
		Where("user_id = ? AND receivable_id = ?", userID, item.ID).
		Count(&existing).Error; err != nil {
		return err
	}
	if existing > 0 {
		return nil
	}
	date := time.Now().Format("2006-01-02")
	if item.DoneAt != nil {
		date = item.DoneAt.Format("2006-01-02")
	}
	income := &model.Finance{
		UserID:       userID,
		Type:         "income",
		Amount:       item.Amount,
		Category:     item.Title,
		Description:  "收款计划到账",
		Date:         date,
		ProjectID:    item.ProjectID,
		ReceivableID: &item.ID,
	}
	return tx.Create(income).Error
}

// DeleteReceivable 删一笔待收款；不是自己的当作不存在，连带删掉它记的收入流水
func (r *Repository) DeleteReceivable(userID, id int) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		res := tx.Where("user_id = ? AND id = ?", userID, id).Delete(&model.Receivable{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return tx.Where("user_id = ? AND receivable_id = ?", userID, id).Delete(&model.Finance{}).Error
	})
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
