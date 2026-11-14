package repository

import (
	"testing"

	"github.com/imloama/umore/internal/model"
)

// 项目工时汇总：任务上的两个手填数字按项目 SUM 出来，不落 projects 表
func TestProjectHoursSummary(t *testing.T) {
	repo := newTestRepo(t)
	uid := newUser(t, repo, "max@example.com")

	project := &model.Project{Name: "门店预约系统", Status: "active"}
	if err := repo.CreateProject(uid, project); err != nil {
		t.Fatalf("create project: %v", err)
	}

	pid := project.ID
	for _, task := range []*model.Task{
		{Title: "排班接口", Category: "todo", ProjectID: &pid, EstimatedHours: 4, ActualHours: 6},
		{Title: "小程序页面", Category: "requirement", ProjectID: &pid, EstimatedHours: 8, ActualHours: 5},
		// 没关联项目 / 没填工时的不该混进来
		{Title: "自己看看文档", Category: "todo", EstimatedHours: 2, ActualHours: 1},
	} {
		if err := repo.CreateTask(uid, task); err != nil {
			t.Fatalf("create task: %v", err)
		}
	}

	list, err := repo.GetProjects(uid, "")
	if err != nil {
		t.Fatalf("list projects: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("want 1 project, got %d", len(list))
	}
	if got := list[0]; got.TaskCount != 2 || got.EstimatedHours != 12 || got.ActualHours != 11 {
		t.Fatalf("汇总不对: count=%d est=%v act=%v", got.TaskCount, got.EstimatedHours, got.ActualHours)
	}

	// 改任务工时后汇总跟着变
	tasks, err := repo.GetTasks(uid, model.TaskFilter{ProjectID: &pid})
	if err != nil || len(tasks) != 2 {
		t.Fatalf("list tasks: %v (%d)", err, len(tasks))
	}
	expect := 0.0
	for i := range tasks {
		tasks[i].ActualHours = tasks[i].ActualHours + 1
		expect += tasks[i].ActualHours
		if err := repo.UpdateTask(uid, &tasks[i]); err != nil {
			t.Fatalf("update task: %v", err)
		}
	}
	one, _ := repo.GetProject(uid, pid)
	if one.ActualHours != expect {
		t.Fatalf("改工时后汇总没跟上: got %v want %v", one.ActualHours, expect)
	}

	// 没有任务的项目也要能列出（汇总为 0，不能是 NULL 扫不进去）
	empty := &model.Project{Name: "还没开工的单", Status: "active"}
	if err := repo.CreateProject(uid, empty); err != nil {
		t.Fatalf("create empty project: %v", err)
	}
	list, _ = repo.GetProjects(uid, "")
	var blank *model.Project
	for i := range list {
		if list[i].ID == empty.ID {
			blank = &list[i]
		}
	}
	if blank == nil || blank.TaskCount != 0 || blank.ActualHours != 0 || !blank.Amount.IsZero() {
		t.Fatalf("空项目汇总异常: %+v", blank)
	}
}

// 金额变更：改金额自动留一条记录，没变就不记
func TestProjectAmountChangeTracking(t *testing.T) {
	repo := newTestRepo(t)
	uid := newUser(t, repo, "max@example.com")

	project := &model.Project{Name: "跨境电商小程序", Status: "active", Amount: model.Money("50000")}
	if err := repo.CreateProject(uid, project); err != nil {
		t.Fatalf("create project: %v", err)
	}
	pid := project.ID

	if changes, _ := repo.GetProjectAmountChanges(uid, pid); len(changes) != 0 {
		t.Fatalf("新建项目不该有变更记录，实际 %d 条", len(changes))
	}

	// 只改文案，金额没变 -> 不记
	project.Name = "跨境电商小程序 v2"
	if err := repo.UpdateProject(uid, project, ""); err != nil {
		t.Fatalf("update project: %v", err)
	}
	if changes, _ := repo.GetProjectAmountChanges(uid, pid); len(changes) != 0 {
		t.Fatalf("金额没变不该记变更，实际 %d 条", len(changes))
	}

	// 加钱 -> 一条记录，delta 正数
	project.Amount = model.Money("62000")
	if err := repo.UpdateProject(uid, project, "客户加了一个拼团模块"); err != nil {
		t.Fatalf("update amount: %v", err)
	}
	changes, err := repo.GetProjectAmountChanges(uid, pid)
	if err != nil {
		t.Fatalf("list changes: %v", err)
	}
	if len(changes) != 1 {
		t.Fatalf("want 1 change, got %d", len(changes))
	}
	first := changes[0]
	if !first.OldAmount.Equal(model.Money("50000")) ||
		!first.NewAmount.Equal(model.Money("62000")) ||
		!first.Delta.Equal(model.Money("12000")) {
		t.Fatalf("变更金额不对: %+v", first)
	}
	if first.Reason != "客户加了一个拼团模块" {
		t.Fatalf("变更说明丢了: %q", first.Reason)
	}

	// 直接加一条变更：项目金额跟着改
	change, err := repo.AddProjectAmountChange(uid, pid, model.Money("58000"), "砍掉一个模块")
	if err != nil {
		t.Fatalf("add amount change: %v", err)
	}
	if !change.Delta.Equal(model.Money("-4000")) {
		t.Fatalf("追加变更算错: %+v", change)
	}

	got, _ := repo.GetProject(uid, pid)
	if !got.Amount.Equal(model.Money("58000")) {
		t.Fatalf("项目总金额没跟着变: %s", got.Amount)
	}
	if got.AmountChanges != 2 || got.ChangedAt == "" {
		t.Fatalf("变更汇总不对: %+v", got)
	}

	if err := repo.DeleteProjectAmountChange(uid, change.ID); err != nil {
		t.Fatalf("delete change: %v", err)
	}
	after, _ := repo.GetProject(uid, pid)
	if after.AmountChanges != 1 {
		t.Fatalf("删记录后次数不对: %d", after.AmountChanges)
	}
	if !after.Amount.Equal(model.Money("58000")) {
		t.Fatal("删变更记录不该回滚项目当前金额")
	}
}

// 已收款汇总只看 income 流水，支出不算
func TestProjectReceivedSummary(t *testing.T) {
	repo := newTestRepo(t)
	uid := newUser(t, repo, "max@example.com")

	project := &model.Project{Name: "门店预约系统", Status: "active", Amount: model.Money("30000")}
	if err := repo.CreateProject(uid, project); err != nil {
		t.Fatalf("create project: %v", err)
	}
	pid := project.ID
	other := &model.Project{Name: "别的单", Status: "active", Amount: model.Money("1000")}
	if err := repo.CreateProject(uid, other); err != nil {
		t.Fatalf("create other project: %v", err)
	}

	for _, f := range []*model.Finance{
		{Type: "income", Amount: model.Money("9000"), Category: "首付款", Date: "2026-11-01", ProjectID: &pid},
		{Type: "income", Amount: model.Money("6000"), Category: "尾款", Date: "2026-11-20", ProjectID: &pid},
		// 支出不该算进收款
		{Type: "expense", Amount: model.Money("500"), Category: "服务器", Date: "2026-11-05", ProjectID: &pid},
		// 关联别的项目，不该混进来
		{Type: "income", Amount: model.Money("1000"), Category: "定金", Date: "2026-11-02", ProjectID: &other.ID},
		// 没关联项目
		{Type: "income", Amount: model.Money("777"), Category: "零活", Date: "2026-11-03"},
	} {
		if err := repo.CreateFinance(uid, f); err != nil {
			t.Fatalf("create finance: %v", err)
		}
	}

	got, _ := repo.GetProject(uid, pid)
	if !got.Received.Equal(model.Money("15000")) {
		t.Fatalf("收款汇总不对: %s (want 15000)", got.Received)
	}

	list, _ := repo.GetProjects(uid, "")
	for _, p := range list {
		if p.ID == other.ID && !p.Received.Equal(model.Money("1000")) {
			t.Fatalf("另一个项目收款不对: %s", p.Received)
		}
	}
}

// 财务区间汇总：收入、支出、结余
func TestFinanceSummary(t *testing.T) {
	repo := newTestRepo(t)
	uid := newUser(t, repo, "max@example.com")

	for _, f := range []*model.Finance{
		{Type: "income", Amount: model.Money("1000.10"), Date: "2026-11-01"},
		{Type: "income", Amount: model.Money("0.20"), Date: "2026-11-02"},
		{Type: "expense", Amount: model.Money("300.05"), Date: "2026-11-03"},
		// 区间外
		{Type: "income", Amount: model.Money("9999"), Date: "2026-10-01"},
	} {
		if err := repo.CreateFinance(uid, f); err != nil {
			t.Fatalf("create finance: %v", err)
		}
	}

	sum, err := repo.GetFinanceSummary(uid, "2026-11-01", "2026-11-30")
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if !sum.TotalIncome.Equal(model.Money("1000.3")) {
		t.Fatalf("收入汇总不对: %s", sum.TotalIncome)
	}
	if !sum.TotalExpense.Equal(model.Money("300.05")) {
		t.Fatalf("支出汇总不对: %s", sum.TotalExpense)
	}
	// 0.1 + 0.2 这类小数累加用 decimal 才不出误差
	if !sum.Balance.Equal(model.Money("700.25")) {
		t.Fatalf("结余不对: %s", sum.Balance)
	}
}

// 删项目：关联的任务与收款留下，只把 project_id 置空
func TestDeleteProjectKeepsRelatedRecords(t *testing.T) {
	repo := newTestRepo(t)
	uid := newUser(t, repo, "max@example.com")

	project := &model.Project{Name: "临时项目", Status: "active"}
	if err := repo.CreateProject(uid, project); err != nil {
		t.Fatalf("create project: %v", err)
	}
	pid := project.ID
	task := &model.Task{Title: "留着", Category: "todo", ProjectID: &pid}
	if err := repo.CreateTask(uid, task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := repo.CreateReceivable(uid, &model.Receivable{Title: "首款", Amount: model.Money("1"), ProjectID: &pid}); err != nil {
		t.Fatalf("create receivable: %v", err)
	}

	if err := repo.DeleteProject(uid, pid); err != nil {
		t.Fatalf("delete project: %v", err)
	}
	got, _ := repo.GetTask(uid, task.ID)
	if got == nil {
		t.Fatal("任务不该跟着项目被删")
	}
	if got.ProjectID != nil {
		t.Fatalf("项目的关联应被置空: %v", *got.ProjectID)
	}
	items, _ := repo.GetReceivables(uid, "", 0, false)
	if len(items) != 1 || items[0].ProjectID != nil {
		t.Fatalf("收款计划应保留并解除关联: %+v", items)
	}
}

// 同一用户下项目重名给 409 用的错误；不同用户可以同名
func TestProjectNameConflict(t *testing.T) {
	eachRepo(t, func(t *testing.T, repo *Repository) {
		alice := newUser(t, repo, "alice@example.com")
		bob := newUser(t, repo, "bob@example.com")

		if err := repo.CreateProject(alice, &model.Project{Name: "官网", Status: "active"}); err != nil {
			t.Fatalf("create project: %v", err)
		}
		err := repo.CreateProject(alice, &model.Project{Name: "官网", Status: "active"})
		if err != ErrProjectNameTaken {
			t.Fatalf("同用户重名应返回 ErrProjectNameTaken，实际 %v", err)
		}
		if err := repo.CreateProject(bob, &model.Project{Name: "官网", Status: "active"}); err != nil {
			t.Fatalf("不同用户同名应允许: %v", err)
		}
	})
}

// 标签重名同样只在用户名下唯一
func TestLabelNameConflict(t *testing.T) {
	repo := newTestRepo(t)
	uid := newUser(t, repo, "max@example.com")

	first := &model.Label{Name: "紧急", Color: "#f00"}
	if err := repo.CreateLabel(uid, first); err != nil {
		t.Fatalf("create label: %v", err)
	}
	// 再建同名：返回已有那条，不报错（任务里顺手带的标签走的就是这条路）
	same := &model.Label{Name: "紧急"}
	if err := repo.CreateLabel(uid, same); err != nil {
		t.Fatalf("同名标签应复用: %v", err)
	}
	if same.ID != first.ID {
		t.Fatalf("同名标签应返回已有记录: %d vs %d", same.ID, first.ID)
	}

	second := &model.Label{Name: "日常", Color: "#0f0"}
	if err := repo.CreateLabel(uid, second); err != nil {
		t.Fatalf("create label: %v", err)
	}
	// 把「日常」改成已存在的「紧急」应被挡下
	second.Name = "紧急"
	if err := repo.UpdateLabel(uid, second); err != ErrLabelNameTaken {
		t.Fatalf("改成重名应返回 ErrLabelNameTaken，实际 %v", err)
	}
}

// 归档：状态能切、归档时间能取、重复归档不留多条记录，
// 且归档后这个项目的任务 / 待收款从日常视图里消失，恢复后又回来。
func TestProjectArchiving(t *testing.T) {
	eachRepo(t, func(t *testing.T, repo *Repository) {
		uid := newUser(t, repo, "archive@example.com")

		old := &model.Project{Name: "旧单子", Status: "active", Amount: model.Money("1000")}
		fresh := &model.Project{Name: "在做的单子", Status: "active"}
		for _, p := range []*model.Project{old, fresh} {
			if err := repo.CreateProject(uid, p); err != nil {
				t.Fatalf("create project: %v", err)
			}
			if p.Status != ProjectStatusActive {
				t.Fatalf("新建项目应是活跃: %s", p.Status)
			}
		}

		oldID := old.ID
		for _, task := range []*model.Task{
			{Title: "旧单子的活", Category: "todo", ProjectID: &oldID},
			{Title: "新单子的活", Category: "todo", ProjectID: &fresh.ID},
			{Title: "没挂项目的活", Category: "todo"},
		} {
			if err := repo.CreateTask(uid, task); err != nil {
				t.Fatalf("create task: %v", err)
			}
		}
		if err := repo.CreateReceivable(uid, &model.Receivable{Title: "旧单尾款",
			Amount: model.Money("500"), ProjectID: &oldID}); err != nil {
			t.Fatalf("create receivable: %v", err)
		}

		archived, err := repo.SetProjectArchived(uid, oldID, true)
		if err != nil {
			t.Fatalf("archive project: %v", err)
		}
		if archived == nil || archived.Status != ProjectStatusArchived || archived.ArchivedAt == nil {
			t.Fatalf("归档后状态/时间不对: %+v", archived)
		}

		// 重复归档不再写记录
		if _, err := repo.SetProjectArchived(uid, oldID, true); err != nil {
			t.Fatalf("archive twice: %v", err)
		}
		changes, _ := repo.GetProjectStatusChanges(uid, oldID)
		if len(changes) != 1 || changes[0].FromStatus != ProjectStatusActive || changes[0].ToStatus != ProjectStatusArchived {
			t.Fatalf("状态变更记录不对: %+v", changes)
		}

		// 列表按状态筛：全部里两个都在，active 只剩一个，archived 只有它
		if list, _ := repo.GetProjects(uid, ""); len(list) != 2 {
			t.Fatalf("全部项目应有 2 个: %+v", list)
		}
		actives, _ := repo.GetProjects(uid, ProjectStatusActive)
		if len(actives) != 1 || actives[0].ID != fresh.ID {
			t.Fatalf("活跃列表不对: %+v", actives)
		}
		archives, _ := repo.GetProjects(uid, ProjectStatusArchived)
		if len(archives) != 1 || archives[0].ID != oldID || archives[0].ArchivedAt == nil {
			t.Fatalf("归档列表不对: %+v", archives)
		}

		// 归档项目的任务从默认视图里消失，只剩没挂项目的那条
		tasks, _ := repo.GetTasks(uid, model.TaskFilter{})
		if len(tasks) != 2 {
			t.Fatalf("归档后默认任务列表该剩 2 条: %+v", tasks)
		}
		for _, task := range tasks {
			if task.ProjectID != nil && *task.ProjectID == oldID {
				t.Fatalf("归档项目的任务不该出现: %+v", task)
			}
		}
		if all, _ := repo.GetTasks(uid, model.TaskFilter{IncludeArchived: true}); len(all) != 3 {
			t.Fatalf("include_archived 时应 3 条: %+v", all)
		}
		// 明确按项目筛就照给：看单个项目的历史要用
		if own, _ := repo.GetTasks(uid, model.TaskFilter{ProjectID: &oldID}); len(own) != 1 {
			t.Fatalf("按项目筛应拿到 1 条: %+v", own)
		}

		// 统计也不该被归档项目的任务撑着
		stats, err := repo.GetTaskStats(uid)
		if err != nil {
			t.Fatalf("stats: %v", err)
		}
		if stats.ByCategory["todo"] != 2 {
			t.Fatalf("统计应只算 2 条未归档任务: %+v", stats.ByCategory)
		}

		// 待收款同理：默认视图不催归档项目的账，汇总也不算
		if list, _ := repo.GetReceivables(uid, "", 0, false); len(list) != 0 {
			t.Fatalf("归档项目的待收款不该出现: %+v", list)
		}
		if list, _ := repo.GetReceivables(uid, "", 0, true); len(list) != 1 {
			t.Fatalf("include_archived 时应 1 笔: %+v", list)
		}
		if sum, _ := repo.GetReceivableSummary(uid); sum.PendingCount != 0 {
			t.Fatalf("归档项目的待收款不该进汇总: %+v", sum)
		}

		// 恢复：任务与待收款回来，状态变更记第二条
		restored, err := repo.SetProjectArchived(uid, oldID, false)
		if err != nil {
			t.Fatalf("restore project: %v", err)
		}
		if restored == nil || restored.Status != ProjectStatusActive || restored.ArchivedAt != nil {
			t.Fatalf("恢复后状态不对: %+v", restored)
		}
		if tasks, _ := repo.GetTasks(uid, model.TaskFilter{}); len(tasks) != 3 {
			t.Fatalf("恢复后任务应回到 3 条: %+v", tasks)
		}
		if list, _ := repo.GetReceivables(uid, "", 0, false); len(list) != 1 {
			t.Fatalf("恢复后待收款应回来: %+v", list)
		}
		if changes, _ := repo.GetProjectStatusChanges(uid, oldID); len(changes) != 2 {
			t.Fatalf("恢复也该记一条: %+v", changes)
		}
	})
}

// 编辑项目不再能改状态：归档 / 恢复只走 SetProjectArchived，
// 免得旧表单提交一遍就把归档状态改回去。
func TestUpdateProjectKeepsStatus(t *testing.T) {
	repo := newTestRepo(t)
	uid := newUser(t, repo, "keep@example.com")

	project := &model.Project{Name: "要归档的单子", Status: "active"}
	if err := repo.CreateProject(uid, project); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if _, err := repo.SetProjectArchived(uid, project.ID, true); err != nil {
		t.Fatalf("archive project: %v", err)
	}

	edited := *project
	edited.Name = "改了名字"
	edited.Status = ProjectStatusActive
	if err := repo.UpdateProject(uid, &edited, ""); err != nil {
		t.Fatalf("update project: %v", err)
	}
	got, _ := repo.GetProject(uid, project.ID)
	if got.Name != "改了名字" || got.Status != ProjectStatusArchived {
		t.Fatalf("改名生效但状态该保持归档: %+v", got)
	}
}

// 归档是本人的事：别人归档不了我的项目
func TestArchiveProjectIsolation(t *testing.T) {
	repo := newTestRepo(t)
	alice := newUser(t, repo, "alice-archive@example.com")
	bob := newUser(t, repo, "bob-archive@example.com")

	project := &model.Project{Name: "Alice 的单", Status: "active"}
	if err := repo.CreateProject(alice, project); err != nil {
		t.Fatalf("create project: %v", err)
	}
	if _, err := repo.SetProjectArchived(bob, project.ID, true); err == nil {
		t.Fatal("B 归档 A 的项目该报 not found")
	}
	if again, _ := repo.GetProject(alice, project.ID); again == nil || again.Status != ProjectStatusActive {
		t.Fatalf("A 的项目被动了: %+v", again)
	}
}
