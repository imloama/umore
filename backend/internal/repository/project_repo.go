package repository

import (
	"errors"
	"time"

	"github.com/imloama/umore/internal/model"

	"gorm.io/gorm"
)

// todayString 逾期判断用的「今天」，按本地时区取日期
func todayString() string {
	return time.Now().Format("2006-01-02")
}

// GetProjects 项目列表，带上任务工时、收款、金额变更、待收款四组汇总量。
//
// 汇总不用一条巨型 JOIN：先查项目，再按 project_id 分组各查一次，
// 有 N 个项目也只是固定几条 SQL，且不用为 SQLite / Postgres 的聚合类型差异操心。
func (r *Repository) GetProjects(userID int) ([]model.Project, error) {
	var projects []model.Project
	if err := r.db.Where("user_id = ?", userID).Order("updated_at DESC").Find(&projects).Error; err != nil {
		return nil, err
	}
	if err := r.fillProjectSummaries(userID, projects); err != nil {
		return nil, err
	}
	return projects, nil
}

// GetProject 取单个项目
func (r *Repository) GetProject(userID, id int) (*model.Project, error) {
	var project model.Project
	err := r.db.Where("user_id = ? AND id = ?", userID, id).First(&project).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	list := []model.Project{project}
	if err := r.fillProjectSummaries(userID, list); err != nil {
		return nil, err
	}
	return &list[0], nil
}

// fillProjectSummaries 一次把四组汇总填进项目列表
func (r *Repository) fillProjectSummaries(userID int, projects []model.Project) error {
	if len(projects) == 0 {
		return nil
	}
	ids := make([]int, 0, len(projects))
	for _, p := range projects {
		ids = append(ids, p.ID)
	}

	// 工时：任务上的两个手填数字，按项目 SUM 出来
	type hoursRow struct {
		ProjectID int
		TaskCount int
		Estimated float64
		Actual    float64
	}
	var hours []hoursRow
	if err := r.db.Model(&model.Task{}).
		Where("user_id = ? AND project_id IN ?", userID, ids).
		Select(`project_id, COUNT(*) AS task_count,
			COALESCE(SUM(estimated_hours), 0) AS estimated,
			COALESCE(SUM(actual_hours), 0) AS actual`).
		Group("project_id").Scan(&hours).Error; err != nil {
		return err
	}

	// 已收款：只看 income 流水
	type receivedRow struct {
		ProjectID int
		Total     model.Amount
	}
	var received []receivedRow
	if err := r.db.Model(&model.Finance{}).
		Where("user_id = ? AND project_id IN ? AND type = ?", userID, ids, "income").
		Select("project_id, COALESCE(SUM(amount), 0) AS total").
		Group("project_id").Scan(&received).Error; err != nil {
		return err
	}

	// 金额变更：次数 + 最近一次时间。
	// 不直接 MAX(created_at)：SQLite 里时间落的是字符串，聚合出来的列 gorm
	// 认不出类型，干脆取 MAX(id)，再把那条记录整行读回来（时间是模型字段，能正常解析）。
	type changeRow struct {
		ProjectID int
		Count     int
		LatestID  int
	}
	var changes []changeRow
	if err := r.db.Model(&model.ProjectAmountChange{}).
		Where("user_id = ? AND project_id IN ?", userID, ids).
		Select("project_id, COUNT(*) AS count, MAX(id) AS latest_id").
		Group("project_id").Scan(&changes).Error; err != nil {
		return err
	}
	latestIDs := make([]int, 0, len(changes))
	for _, c := range changes {
		latestIDs = append(latestIDs, c.LatestID)
	}
	latestAt := map[int]time.Time{}
	if len(latestIDs) > 0 {
		var latest []model.ProjectAmountChange
		if err := r.db.Select("id", "created_at").Where("id IN ?", latestIDs).
			Find(&latest).Error; err != nil {
			return err
		}
		for _, c := range latest {
			latestAt[c.ID] = c.CreatedAt
		}
	}

	// 待收款：只算 pending，逾期按到期日 < 今天（今天由 Go 传进去，不用方言函数）
	today := todayString()
	type receivableRow struct {
		ProjectID int
		Count     int
		Total     model.Amount
		Overdue   model.Amount
		NextDue   *string
	}
	var receivables []receivableRow
	if err := r.db.Model(&model.Receivable{}).
		Where("user_id = ? AND project_id IN ? AND status = ?", userID, ids, "pending").
		Select(`project_id, COUNT(*) AS count,
			COALESCE(SUM(amount), 0) AS total,
			COALESCE(SUM(CASE WHEN COALESCE(due_date, '') <> '' AND due_date < ? THEN amount ELSE 0 END), 0) AS overdue,
			MIN(CASE WHEN COALESCE(due_date, '') <> '' THEN due_date END) AS next_due`, today).
		Group("project_id").Scan(&receivables).Error; err != nil {
		return err
	}

	hoursBy := map[int]hoursRow{}
	for _, h := range hours {
		hoursBy[h.ProjectID] = h
	}
	receivedBy := map[int]model.Amount{}
	for _, x := range received {
		receivedBy[x.ProjectID] = x.Total.Normalize()
	}
	changesBy := map[int]changeRow{}
	for _, c := range changes {
		changesBy[c.ProjectID] = c
	}
	receivableBy := map[int]receivableRow{}
	for _, x := range receivables {
		receivableBy[x.ProjectID] = x
	}

	for i := range projects {
		p := &projects[i]
		if h, ok := hoursBy[p.ID]; ok {
			p.TaskCount = h.TaskCount
			p.EstimatedHours = h.Estimated
			p.ActualHours = h.Actual
		}
		if v, ok := receivedBy[p.ID]; ok {
			p.Received = v
		}
		if c, ok := changesBy[p.ID]; ok {
			p.AmountChanges = c.Count
			if at, ok := latestAt[c.LatestID]; ok {
				// RFC3339，前端 new Date() 直接能解析
				p.ChangedAt = at.Format(time.RFC3339)
			}
		}
		if x, ok := receivableBy[p.ID]; ok {
			p.ReceivableCount = x.Count
			p.ReceivableAmount = x.Total.Normalize()
			p.ReceivableOverdue = x.Overdue.Normalize()
			if x.NextDue != nil {
				p.NextDueDate = *x.NextDue
			}
		}
	}
	return nil
}

// CreateProject 建项目
func (r *Repository) CreateProject(userID int, p *model.Project) error {
	if p.Status == "" {
		p.Status = "active"
	}
	p.UserID = userID
	p.ID = 0
	if err := r.db.Create(p).Error; err != nil {
		if isDuplicate(err) {
			return ErrProjectNameTaken
		}
		return err
	}
	return nil
}

// UpdateProject 更新项目；金额变了就顺手记一条变更记录，reason 为空则记「手动调整」
func (r *Repository) UpdateProject(userID int, p *model.Project, reason string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var existing model.Project
		err := tx.Where("user_id = ? AND id = ?", userID, p.ID).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return gorm.ErrRecordNotFound
		}
		if err != nil {
			return err
		}

		if p.Status == "" {
			p.Status = "active"
		}
		if err := tx.Model(&model.Project{}).Where("user_id = ? AND id = ?", userID, p.ID).
			Select("name", "description", "git_repo", "customer", "status", "amount").
			Updates(p).Error; err != nil {
			if isDuplicate(err) {
				return ErrProjectNameTaken
			}
			return err
		}

		if !existing.Amount.Equal(p.Amount.Decimal) {
			return insertAmountChange(tx, userID, p.ID, existing.Amount, p.Amount, reason)
		}
		return nil
	})
}

// DeleteProject 删项目；关联的任务/收款保留，只把 project_id 置空。
// 不是自己的项目一律当作不存在，上层回 404。
func (r *Repository) DeleteProject(userID, id int) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		res := tx.Where("user_id = ? AND id = ?", userID, id).Delete(&model.Project{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		for _, m := range []any{&model.Task{}, &model.Finance{}, &model.Receivable{}} {
			if err := tx.Model(m).Where("user_id = ? AND project_id = ?", userID, id).
				Update("project_id", nil).Error; err != nil {
				return err
			}
		}
		return tx.Where("user_id = ? AND project_id = ?", userID, id).
			Delete(&model.ProjectAmountChange{}).Error
	})
}

// GetProjectAmountChanges 金额变更记录，新的在前
func (r *Repository) GetProjectAmountChanges(userID, projectID int) ([]model.ProjectAmountChange, error) {
	changes := []model.ProjectAmountChange{}
	err := r.db.Where("user_id = ? AND project_id = ?", userID, projectID).
		Order("created_at DESC, id DESC").Find(&changes).Error
	if err != nil {
		return nil, err
	}
	return changes, nil
}

// AddProjectAmountChange 记一条金额变更，同时把项目总金额改成新值
// 这样「加钱」不用先改项目再补记录，一个动作两步落库。
//
// 新金额跟当前金额一样就直接返回：不写金额变更流水，仍回读一条「虚拟记录」
// 给接口回包（调用方拿得到 new_amount/delta=0），免得用「相等也算变更」把库写脏。
func (r *Repository) AddProjectAmountChange(userID, projectID int, newAmount model.Amount, reason string) (*model.ProjectAmountChange, error) {
	var change *model.ProjectAmountChange
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var project model.Project
		err := tx.Where("user_id = ? AND id = ?", userID, projectID).First(&project).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return gorm.ErrRecordNotFound
		}
		if err != nil {
			return err
		}

		if project.Amount.Equal(newAmount.Decimal) {
			record := model.ProjectAmountChange{
				ProjectID: projectID,
				OldAmount: project.Amount,
				NewAmount: newAmount,
				Reason:    normalizeReason(reason),
			}
			change = &record
			return nil
		}

		// 金额变了：金额和 updated_at 一起刷，别让列表里的「最近更新」还停在改名那天。
		// updated_at 显式写：Update 单列时 gorm 的自动时间戳不参与，不写就还是老值。
		if err := tx.Model(&model.Project{}).Where("user_id = ? AND id = ?", userID, projectID).
			Updates(map[string]any{"amount": newAmount, "updated_at": time.Now()}).Error; err != nil {
			return err
		}

		record := model.ProjectAmountChange{
			UserID:    userID,
			ProjectID: projectID,
			OldAmount: project.Amount,
			NewAmount: newAmount,
			Delta:     newAmount.Sub(project.Amount),
			Reason:    normalizeReason(reason),
		}
		if err := tx.Create(&record).Error; err != nil {
			return err
		}
		change = &record
		return nil
	})
	if err != nil {
		return nil, err
	}
	return change, nil
}

// DeleteProjectAmountChange 删一条变更记录，不回滚项目当前金额——历史是历史，金额以最新一条为准
func (r *Repository) DeleteProjectAmountChange(userID, id int) error {
	res := r.db.Where("user_id = ? AND id = ?", userID, id).Delete(&model.ProjectAmountChange{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func insertAmountChange(tx *gorm.DB, userID, projectID int, oldAmount, newAmount model.Amount, reason string) error {
	record := model.ProjectAmountChange{
		UserID:    userID,
		ProjectID: projectID,
		OldAmount: oldAmount,
		NewAmount: newAmount,
		Delta:     newAmount.Sub(oldAmount),
		Reason:    normalizeReason(reason),
	}
	return tx.Create(&record).Error
}

func normalizeReason(reason string) string {
	if reason == "" {
		return "手动调整总金额"
	}
	return reason
}
