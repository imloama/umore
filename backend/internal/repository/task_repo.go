package repository

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/imloama/umore/internal/model"

	"gorm.io/gorm"
)

// GetTasks 按条件查询任务。所有查询都带 user_id，别人的任务查不出来。
func (r *Repository) GetTasks(userID int, f model.TaskFilter) ([]model.Task, error) {
	query := r.db.Model(&model.Task{}).Where("tasks.user_id = ?", userID)

	if f.Category != "" {
		query = query.Where("tasks.category = ?", f.Category)
	}
	if f.Status != "" {
		query = query.Where("tasks.status = ?", f.Status)
	}
	if f.Priority != "" {
		query = query.Where("tasks.priority = ?", f.Priority)
	}
	if f.ProjectID != nil {
		query = query.Where("tasks.project_id = ?", *f.ProjectID)
	}
	// 归档项目下的任务默认不出现：项目归档的意思就是「这条线的数据别再烦我」。
	// 按项目筛或显式带上 IncludeArchived 时才查（看单个项目的历史用得上）。
	if !f.IncludeArchived && f.ProjectID == nil {
		cond, args := r.notArchivedProjectID(userID, "tasks")
		query = query.Where(cond, args...)
	}
	if f.Scheduled != "" {
		query = query.Where("tasks.scheduled_date = ?", f.Scheduled)
	}
	// 日历区间：优先看计划日期，没排期的用截止日期兜底
	if f.StartDate != "" && f.EndDate != "" {
		query = query.Where("COALESCE(NULLIF(tasks.scheduled_date, ''), tasks.due_date) BETWEEN ? AND ?",
			f.StartDate, f.EndDate)
	}
	if f.TodayOnly {
		if f.Today == "" {
			f.Today = model.TodayIn(r.timezoneOf(userID))
		}
		// "!" 打头表示「今天不算」。「今天该做」那半截的取反，加上「今天完成」那半截不动：
		// 今天刚勾掉的既不是今天的活，也不该掉进「已过期 / 未开始」。
		today := strings.TrimPrefix(f.Today, "!")
		negate := strings.HasPrefix(f.Today, "!")
		if !negate {
			cond, args := r.todayCond(r.db.Dialector.Name(), r.timezoneOf(userID), today)
			query = query.Where(cond, args...)
		}
	}
	if f.Keyword != "" {
		kw := "%" + f.Keyword + "%"
		query = query.Where("tasks.title LIKE ? OR tasks.content LIKE ?", kw, kw)
	}
	if f.Label != "" {
		query = query.Where(`EXISTS (
			SELECT 1 FROM task_labels tl JOIN labels l ON l.id = tl.label_id
			WHERE tl.task_id = tasks.id AND l.name = ? AND l.user_id = ?)`, f.Label, userID)
	}

	// 排序：今日在前的按手动顺序，其余按状态、优先级、更新时间
	query = query.Order("CASE WHEN tasks.is_today THEN 0 ELSE 1 END").
		Order("COALESCE(tasks.today_order, 999999)").
		Order("CASE tasks.status WHEN 'DOING' THEN 0 WHEN 'BLOCKED' THEN 1 WHEN 'TODO' THEN 2 ELSE 3 END").
		Order("CASE tasks.priority WHEN 'URGENT' THEN 0 WHEN 'HIGH' THEN 1 WHEN 'MEDIUM' THEN 2 ELSE 3 END").
		Order("tasks.updated_at DESC")

	var tasks []model.Task
	if err := query.Find(&tasks).Error; err != nil {
		return nil, err
	}
	// 没排期的任务「开始时间」看创建日期，这一步只有 Go 时间能算（见 model.TodayCondition 的注释）。
	// 正向这边 SQL 已经把排期、截止、状态筛过一遍，这里只补这一个条件；
	// 取反那边 SQL 什么都没筛，全靠这儿挑。
	if f.TodayOnly {
		timezone := r.timezoneOf(userID)
		today := strings.TrimPrefix(f.Today, "!")
		negate := strings.HasPrefix(f.Today, "!")
		kept := make([]model.Task, 0, len(tasks))
		for _, t := range tasks {
			// 「今天完成的」只算今天的活，不落进「已过期 / 未开始」那一栏：
			// 取反那半边写严格点——不在今天，才进那一栏
			isToday := model.FinishedOn(t, timezone, today) ||
				(model.NotDoneAndOpen(t, today) && model.HasWindow(t) && model.IsStarted(t, today))
			if isToday != negate {
				kept = append(kept, t)
			}
		}
		tasks = kept
	}
	if err := r.attachLabels(userID, tasks); err != nil {
		return nil, err
	}
	if err := r.attachProjectNames(userID, tasks); err != nil {
		return nil, err
	}
	// 顺手把「算不算今天」标上：列表里一行就能看出今天该做哪些，前端不用自己再算一遍日期
	r.markToday(userID, tasks)
	return tasks, nil
}

// GetTask 取单条任务；不是这个用户的就当作不存在
func (r *Repository) GetTask(userID, id int) (*model.Task, error) {
	var task model.Task
	err := r.db.Where("user_id = ? AND id = ?", userID, id).First(&task).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	tasks := []model.Task{task}
	if err := r.attachLabels(userID, tasks); err != nil {
		return nil, err
	}
	if err := r.attachProjectNames(userID, tasks); err != nil {
		return nil, err
	}
	r.markToday(userID, tasks)
	return &tasks[0], nil
}

// markToday 给查出来的任务打上「算不算今天的今日任务」。
//
// 判定还是那一条 SQL（model.TodayCondition），不在这儿另写一份规则 ——
// 两处各写一份迟早对不上，列表和统计就又开始打架了。
// 唯一的例外是「没排期就看创建日期」：日期列的方言差异只能交给 Go 补，
// 所以 SQL 筛过之后再拿 model.IsStarted 收一遍。
func (r *Repository) markToday(userID int, tasks []model.Task) {
	if len(tasks) == 0 {
		return
	}
	ids := make([]int, 0, len(tasks))
	for _, t := range tasks {
		ids = append(ids, t.ID)
	}
	timezone := r.timezoneOf(userID)
	todayDate := model.TodayIn(timezone)
	cond, args := r.todayCond(r.db.Dialector.Name(), timezone, todayDate)
	var candidates []model.Task
	if err := r.db.Model(&model.Task{}).
		Where("user_id = ? AND id IN ?", userID, ids).
		Where(cond, args...).
		Select("id", "status", "scheduled_date", "due_date", "created_at").
		Find(&candidates).Error; err != nil {
		return
	}
	set := make(map[int]bool, len(candidates))
	for _, t := range candidates {
		// 没排期的要看创建日期，这一步 SQL 算不了，交给 IsStarted；
		// 今天完成的不受这条约束
		if t.Status != "DONE" && (!model.HasWindow(t) || !model.IsStarted(t, todayDate)) {
			continue
		}
		set[t.ID] = true
	}
	for i := range tasks {
		today := set[tasks[i].ID]
		if tasks[i].Status != "DONE" {
			today = today && model.HasWindow(tasks[i]) && model.IsStarted(tasks[i], todayDate)
		}
		tasks[i].TodayFlag = today
	}
}

// attachLabels 一次查出这批任务的标签，别在循环里逐条查。
// 会先清空 Labels：调用方可能带着刚提交的壳子标签进来，别叠加成两份。
func (r *Repository) attachLabels(userID int, tasks []model.Task) error {
	for i := range tasks {
		tasks[i].Labels = []model.Label{}
	}
	ids := make([]int, 0, len(tasks))
	for _, t := range tasks {
		ids = append(ids, t.ID)
	}
	if len(ids) == 0 {
		return nil
	}
	var labels []model.Label
	if err := r.db.Model(&model.Label{}).
		Joins("JOIN task_labels tl ON tl.label_id = labels.id").
		Where("tl.task_id IN ? AND labels.user_id = ?", ids, userID).
		Order("labels.name").
		Find(&labels).Error; err != nil {
		return err
	}
	if len(labels) == 0 {
		return nil
	}

	type link struct {
		TaskID  int
		LabelID int
	}
	var links []link
	if err := r.db.Table("task_labels").Where("task_id IN ?", ids).Find(&links).Error; err != nil {
		return err
	}
	byID := map[int]model.Label{}
	for _, l := range labels {
		byID[l.ID] = l
	}
	for _, l := range links {
		label, ok := byID[l.LabelID]
		if !ok {
			continue
		}
		for i := range tasks {
			if tasks[i].ID == l.TaskID {
				tasks[i].Labels = append(tasks[i].Labels, label)
			}
		}
	}
	return nil
}

// attachProjectNames 列表要显示项目名，顺带把名字查出来
func (r *Repository) attachProjectNames(userID int, tasks []model.Task) error {
	ids := map[int]bool{}
	for _, t := range tasks {
		if t.ProjectID != nil {
			ids[*t.ProjectID] = true
		}
	}
	if len(ids) == 0 {
		return nil
	}
	list := make([]int, 0, len(ids))
	for id := range ids {
		list = append(list, id)
	}
	var projects []model.Project
	if err := r.db.Select("id", "name").Where("user_id = ? AND id IN ?", userID, list).Find(&projects).Error; err != nil {
		return err
	}
	names := map[int]string{}
	for _, p := range projects {
		names[p.ID] = p.Name
	}
	for i := range tasks {
		if tasks[i].ProjectID != nil {
			tasks[i].ProjectName = names[*tasks[i].ProjectID]
		}
	}
	return nil
}

// CreateTask 建任务；编号等于 id，所以先落库再回填
func (r *Repository) CreateTask(userID int, t *model.Task) error {
	normalizeTask(t)
	if err := r.checkProjectOwnership(userID, t.ProjectID); err != nil {
		return err
	}
	t.UserID = userID
	t.ID = 0
	labels := t.Labels
	// 标签单独处理，别让 gorm 的 many2many 顺手去建标签记录
	t.Labels = nil

	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Omit("Labels").Create(t).Error; err != nil {
			return err
		}
		if t.Number == 0 {
			t.Number = t.ID
			if err := tx.Model(&model.Task{}).Where("user_id = ? AND id = ?", userID, t.ID).
				Update("number", t.Number).Error; err != nil {
				return err
			}
		}
		return setTaskLabels(tx, userID, t.ID, labels)
	})
	if err != nil {
		return err
	}
	// 回填标签的 id 与颜色、项目名：前端拿返回值直接渲染，别给它一堆 id=0 的壳子
	return r.fillTaskExtras(userID, t, labels)
}

// UpdateTask 更新任务；Labels 为 nil 表示「不动标签」
func (r *Repository) UpdateTask(userID int, t *model.Task) error {
	normalizeTask(t)
	if err := r.checkProjectOwnership(userID, t.ProjectID); err != nil {
		return err
	}
	labels := t.Labels
	t.Labels = nil

	// 提前收工：原计划是往后的日子，今天却做完了。
	// 把开始日期落到今天，否则这条「今天该做的」不算今天的活——它还没开始就被做完了，
	// 今日统计里也该有它一条（状态是已完成就是了）。
	startOnToday := false
	if t.Status == "DONE" {
		var current model.Task
		if err := r.db.Model(&model.Task{}).
			Where("user_id = ? AND id = ?", userID, t.ID).
			Select("status", "created_at").First(&current).Error; err == nil && current.Status != "DONE" {
			scheduled := t.ScheduledDate
			if scheduled == nil || strings.TrimSpace(*scheduled) == "" {
				created := current.CreatedAt.Format("2006-01-02")
				scheduled = &created
			}
			if today := r.TodayFor(userID); *scheduled > today {
				t.ScheduledDate = &today
				startOnToday = true
			}
		}
	}

	err := r.db.Transaction(func(tx *gorm.DB) error {
		if startOnToday {
			// 开始日期改成今天这一步不能丢：Select 会把零值字段跳过，得单独补一次
			if err := tx.Model(&model.Task{}).
				Where("user_id = ? AND id = ?", userID, t.ID).
				Update("scheduled_date", *t.ScheduledDate).Error; err != nil {
				return err
			}
		}
		res := tx.Model(&model.Task{}).
			Where("user_id = ? AND id = ?", userID, t.ID).
			Select("title", "content", "category", "status", "opportunity_status", "priority",
				"project_id", "due_date", "scheduled_date", "estimated_hours", "actual_hours",
				"is_today", "today_order", "closed_at").
			Updates(t)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		if labels == nil {
			return nil
		}
		if err := tx.Where("task_id = ?", t.ID).Delete(&taskLabel{}).Error; err != nil {
			return err
		}
		return setTaskLabels(tx, userID, t.ID, labels)
	})
	if err != nil {
		return err
	}
	if labels == nil {
		return nil
	}
	// 同 CreateTask：把 id 与颜色回填给调用方
	return r.fillTaskExtras(userID, t, labels)
}

// DeleteTask 删任务，顺手清掉标签关联（库里也配了级联，多一层保险）。
// 不是自己的任务一律当作不存在，上层回 404。
func (r *Repository) DeleteTask(userID, id int) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// 先清关联再删主体：task_labels 对 tasks 有外键，顺序反了会撞约束。
		// 子查询里带 user_id，别人的任务连关联都碰不到。
		if err := tx.Where("task_id IN (?)",
			tx.Model(&model.Task{}).Select("id").Where("user_id = ? AND id = ?", userID, id)).
			Delete(&taskLabel{}).Error; err != nil {
			return err
		}
		res := tx.Where("user_id = ? AND id = ?", userID, id).Delete(&model.Task{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

// SetTaskToday 加入/移出今日
func (r *Repository) SetTaskToday(userID, id int, isToday bool, order int) error {
	updates := map[string]any{"is_today": false, "today_order": nil}
	if isToday {
		updates["is_today"] = true
		updates["today_order"] = order
	}
	return r.db.Model(&model.Task{}).
		Where("user_id = ? AND id = ?", userID, id).
		Updates(updates).Error
}

// NextTodayOrder 该用户今日任务的最大排序值+1
func (r *Repository) NextTodayOrder(userID int) (int, error) {
	var max *int
	err := r.db.Model(&model.Task{}).
		Where("user_id = ? AND is_today = ?", userID, true).
		Select("MAX(today_order)").Scan(&max).Error
	if err != nil {
		return 0, err
	}
	if max == nil {
		return 0, nil
	}
	return *max + 1, nil
}

// ReorderToday 今日任务排序，只动自己的任务
func (r *Repository) ReorderToday(userID int, orders map[int]int) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		for id, order := range orders {
			if err := tx.Model(&model.Task{}).
				Where("user_id = ? AND id = ? AND is_today = ?", userID, id, true).
				Update("today_order", order).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// GetTaskStats 该用户的任务统计
func (r *Repository) GetTaskStats(userID int) (*model.TaskStats, error) {
	stats := &model.TaskStats{ByCategory: map[string]int{}, ByStatus: map[string]int{}}

	type groupCount struct {
		Name  string
		Total int
	}
	// 归档项目的任务不进任何统计：首页的分类、状态计数都不该再被旧项目撑着。
	// gorm 的链式条件会互相污染，所以每回都从一条干净查询起头。
	cond, args := r.notArchivedProjectID(userID, "tasks")
	visible := func() *gorm.DB {
		return r.db.Model(&model.Task{}).Where("user_id = ?", userID).Where(cond, args...)
	}

	var byCategory []groupCount
	if err := visible().Select("category AS name, COUNT(*) AS total").
		Group("category").Scan(&byCategory).Error; err != nil {
		return nil, err
	}
	for _, g := range byCategory {
		stats.ByCategory[g.Name] = g.Total
	}

	var byStatus []groupCount
	if err := visible().Select("status AS name, COUNT(*) AS total").
		Group("status").Scan(&byStatus).Error; err != nil {
		return nil, err
	}
	for _, g := range byStatus {
		stats.ByStatus[g.Name] = g.Total
	}

	var today struct {
		Total      int
		Done       int
		EstimatedH float64
		ActualH    float64
	}
	// 今日口径：今天该做的（没完成 + 今天落在开始与结束时间之间），加上今天完成的。
	// 不再看 is_today（那只是手动钉住），也不再是「加进过今日的都算」。
	timezone := r.timezoneOf(userID)
	todayDate := model.TodayIn(timezone)
	// 跟列表走同一套口径：SQL 筛日期列（含今天完成那条），Go 补「没排期的看创建日期」。
	// 统计要的是合计，没法只挑几行，索性把候选行捞回来在 Go 里累加 ——
	// 今日任务本来也就几十条，比在 SQL 里拼方言函数稳。
	// 前面几条统计已经把 cond/args 占住了，这里另起一份，别把归档条件和今日条件串在一起
	todayCond, todayArgs := r.todayCond(r.db.Dialector.Name(), timezone, todayDate)
	var candidates []model.Task
	if err := visible().Where(todayCond, todayArgs...).
		Select("id", "status", "scheduled_date", "due_date", "created_at",
			"estimated_hours", "actual_hours").
		Find(&candidates).Error; err != nil {
		return nil, err
	}
	for _, task := range candidates {
		// 今天完成的不用再过这两关：它已经干完了
		if task.Status != "DONE" && (!model.HasWindow(task) || !model.IsStarted(task, todayDate)) {
			continue
		}
		today.Total++
		if task.Status == "DONE" {
			today.Done++
		}
		today.EstimatedH += task.EstimatedHours
		today.ActualH += task.ActualHours
	}
	stats.TodayDate = todayDate
	stats.Timezone = model.NormalizeTimezone(r.timezoneOf(userID))
	stats.TodayTotal = today.Total
	stats.TodayDone = today.Done
	stats.TodayEstimatedHours = today.EstimatedH
	stats.TodayActualHours = today.ActualH

	return stats, nil
}

// fillTaskExtras 把「只读字段」补进刚写库的任务：标签的 id / 颜色、项目名
func (r *Repository) fillTaskExtras(userID int, t *model.Task, labels []model.Label) error {
	t.Labels = labels
	stored := []model.Task{*t}
	if err := r.attachLabels(userID, stored); err != nil {
		return err
	}
	if err := r.attachProjectNames(userID, stored); err != nil {
		return err
	}
	t.Labels = stored[0].Labels
	t.ProjectName = stored[0].ProjectName
	return nil
}

// checkProjectOwnership 关联的 project_id 必须是自己的项目。
// 否则可以把任务挂到别人的项目 id 上，虽然查不出对方数据，
// 但会造出一堆指向不存在项目的脏记录，项目汇总也会变得不可信。
func (r *Repository) checkProjectOwnership(userID int, projectID *int) error {
	if projectID == nil {
		return nil
	}
	var count int64
	err := r.db.Model(&model.Project{}).
		Where("user_id = ? AND id = ?", userID, *projectID).Count(&count).Error
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrProjectNotFound
	}
	return nil
}

// taskLabel 任务-标签关联表，显式建模型是为了能直接按 task_id 删
type taskLabel struct {
	TaskID  int `gorm:"primaryKey"`
	LabelID int `gorm:"primaryKey"`
}

func (taskLabel) TableName() string { return "task_labels" }

// setTaskLabels 按标签名写入关联，标签不存在就在当前用户名下建一个
func setTaskLabels(tx *gorm.DB, userID, taskID int, labels []model.Label) error {
	for _, l := range labels {
		name := strings.TrimSpace(l.Name)
		if name == "" {
			continue
		}
		label := model.Label{UserID: userID, Name: name, Color: defaultLabelColor(l.Color)}
		if err := tx.Where("user_id = ? AND name = ?", userID, name).
			FirstOrCreate(&label, label).Error; err != nil {
			return err
		}
		link := taskLabel{TaskID: taskID, LabelID: label.ID}
		if err := tx.Where(link).FirstOrCreate(&link).Error; err != nil {
			return err
		}
	}
	return nil
}

func defaultLabelColor(color string) string {
	if strings.TrimSpace(color) == "" {
		return "#1890ff"
	}
	return color
}

// normalizeTask 补齐默认值，避免脏数据
func normalizeTask(t *model.Task) {
	if t.Category == "" {
		t.Category = "todo"
	}
	if t.Status == "" {
		t.Status = "TODO"
	}
	if t.Priority == "" {
		t.Priority = "MEDIUM"
	}
	if t.EstimatedHours < 0 {
		t.EstimatedHours = 0
	}
	if t.ActualHours < 0 {
		t.ActualHours = 0
	}
	if t.Category != "opportunity" {
		t.OpportunityStatus = ""
	}
	if t.DueDate != nil && strings.TrimSpace(*t.DueDate) == "" {
		t.DueDate = nil
	}
	if t.ScheduledDate != nil && strings.TrimSpace(*t.ScheduledDate) == "" {
		t.ScheduledDate = nil
	}
	if t.Status == "DONE" {
		if t.ClosedAt == nil {
			now := time.Now()
			t.ClosedAt = &now
		}
	} else {
		t.ClosedAt = nil
	}
}

// todayCond 拼好 model.TodayCondition（按方言填完成时间那段），再补齐参数。
// Postgres 那段要先把完成时间挪进用户时区，所以多一个时区参数；
// 判定「今天完成的」也得挪，否则东八区早上八点前勾掉的事会被算成昨天。
func (r *Repository) todayCond(driver, timezone, today string) (string, []any) {
	args := []any{today, today}
	if driver == Postgres {
		args = append(args, model.NormalizeTimezone(timezone))
	}
	return fmt.Sprintf(model.TodayCondition, model.ClosedAtExpr(driver)), append(args, today)
}
