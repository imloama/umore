package repository

import (
	"testing"

	"github.com/imloama/umore/internal/model"
)

// 任务主流程：建、查、改、今日排序、统计
func TestTaskLifecycle(t *testing.T) {
	repo := newTestRepo(t)
	uid := newUser(t, repo, "max@example.com")

	project := &model.Project{Name: "官网改版", Status: "active", Amount: model.Money("50000")}
	if err := repo.CreateProject(uid, project); err != nil {
		t.Fatalf("create project: %v", err)
	}

	task := &model.Task{
		Title: "写报价", Category: "todo", ProjectID: &project.ID,
		EstimatedHours: 4, ActualHours: 3, IsToday: true,
		Labels: []model.Label{{Name: "重要"}, {Name: "报价"}},
	}
	if err := repo.CreateTask(uid, task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	if task.ID == 0 || task.Number != task.ID {
		t.Fatalf("任务 id/number 不对: id=%d number=%d", task.ID, task.Number)
	}

	got, err := repo.GetTask(uid, task.ID)
	if err != nil || got == nil {
		t.Fatalf("get task: %v", err)
	}
	if !got.IsToday || got.EstimatedHours != 4 || len(got.Labels) != 2 {
		t.Fatalf("任务读回来不对: %+v", got)
	}
	if got.ProjectName != "官网改版" {
		t.Fatalf("项目名没带出来: %q", got.ProjectName)
	}

	// 标签复用：同名标签不会建出第二条
	second := &model.Task{Title: "另一个", Category: "todo", Labels: []model.Label{{Name: "重要"}}}
	if err := repo.CreateTask(uid, second); err != nil {
		t.Fatalf("重复标签应能插入: %v", err)
	}
	labels, err := repo.GetLabels(uid)
	if err != nil {
		t.Fatalf("get labels: %v", err)
	}
	if len(labels) != 2 {
		t.Fatalf("同名标签应复用，实际 %d 个: %+v", len(labels), labels)
	}

	// 按标签筛
	filtered, err := repo.GetTasks(uid, model.TaskFilter{Label: "重要"})
	if err != nil {
		t.Fatalf("filter by label: %v", err)
	}
	if len(filtered) != 2 {
		t.Fatalf("按标签应查出 2 条，实际 %d", len(filtered))
	}

	stats, err := repo.GetTaskStats(uid)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if stats.TodayTotal != 1 || stats.ByCategory["todo"] != 2 {
		t.Fatalf("统计不对: %+v", stats)
	}
	if stats.TodayEstimatedHours != 4 || stats.TodayActualHours != 3 {
		t.Fatalf("今日工时统计不对: %+v", stats)
	}

	// 今日排序
	order, err := repo.NextTodayOrder(uid)
	if err != nil {
		t.Fatalf("next today order: %v", err)
	}
	if err := repo.SetTaskToday(uid, second.ID, true, order); err != nil {
		t.Fatalf("set today: %v", err)
	}
	if err := repo.ReorderToday(uid, map[int]int{task.ID: 5, second.ID: 1}); err != nil {
		t.Fatalf("reorder: %v", err)
	}
	today, err := repo.GetTasks(uid, model.TaskFilter{TodayOnly: true})
	if err != nil {
		t.Fatalf("list today: %v", err)
	}
	if len(today) != 2 || today[0].ID != second.ID {
		t.Fatalf("今日排序不对: %+v", today)
	}

	// 移出今日
	if err := repo.SetTaskToday(uid, second.ID, false, 0); err != nil {
		t.Fatalf("remove today: %v", err)
	}
	if stats, _ := repo.GetTaskStats(uid); stats.TodayTotal != 1 {
		t.Fatalf("移出今日后统计不对: %+v", stats)
	}

	// 改任务：标签为 nil 表示不动，显式给空数组才清空
	got.Title = "写报价 v2"
	got.Labels = nil
	if err := repo.UpdateTask(uid, got); err != nil {
		t.Fatalf("update task: %v", err)
	}
	again, _ := repo.GetTask(uid, task.ID)
	if again.Title != "写报价 v2" || len(again.Labels) != 2 {
		t.Fatalf("更新不合预期: %+v", again)
	}

	if err := repo.DeleteTask(uid, second.ID); err != nil {
		t.Fatalf("delete task: %v", err)
	}
	if rest, _ := repo.GetTasks(uid, model.TaskFilter{}); len(rest) != 1 {
		t.Fatalf("删除后应只剩 1 条，实际 %d", len(rest))
	}
}

// 任务完成会写 closed_at，撤销完成会清掉
func TestTaskDoneClosesAt(t *testing.T) {
	repo := newTestRepo(t)
	uid := newUser(t, repo, "max@example.com")

	task := &model.Task{Title: "发版", Category: "todo"}
	if err := repo.CreateTask(uid, task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	if task.ClosedAt != nil {
		t.Fatalf("新建任务不该有完成时间")
	}

	task.Status = "DONE"
	if err := repo.UpdateTask(uid, task); err != nil {
		t.Fatalf("done task: %v", err)
	}
	done, _ := repo.GetTask(uid, task.ID)
	if done.ClosedAt == nil {
		t.Fatal("标成 DONE 应写入完成时间")
	}

	done.Status = "DOING"
	if err := repo.UpdateTask(uid, done); err != nil {
		t.Fatalf("reopen task: %v", err)
	}
	back, _ := repo.GetTask(uid, task.ID)
	if back.ClosedAt != nil {
		t.Fatal("撤回完成应清掉完成时间")
	}
}

// 任务评论挂在 target_type=task 上，与项目评论互不串台
func TestTaskComments(t *testing.T) {
	repo := newTestRepo(t)
	uid := newUser(t, repo, "max@example.com")

	task := &model.Task{Title: "排班接口", Category: "todo"}
	if err := repo.CreateTask(uid, task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	project := &model.Project{Name: "门店", Status: "active"}
	if err := repo.CreateProject(uid, project); err != nil {
		t.Fatalf("create project: %v", err)
	}

	comment := &model.Comment{TargetType: "task", TargetID: task.ID, Content: "**先做接口**"}
	if err := repo.CreateComment(uid, comment); err != nil {
		t.Fatalf("create comment: %v", err)
	}
	// 同样的 target_id，但类型是 project，不该混进来
	if err := repo.CreateComment(uid, &model.Comment{TargetType: "project", TargetID: task.ID, Content: "x"}); err != nil {
		t.Fatalf("create project comment: %v", err)
	}

	list, err := repo.GetComments(uid, "task", task.ID)
	if err != nil {
		t.Fatalf("get comments: %v", err)
	}
	if len(list) != 1 || list[0].ContentHTML == "" {
		t.Fatalf("任务评论不对: %+v", list)
	}

	if err := repo.DeleteComment(uid, comment.ID); err != nil {
		t.Fatalf("delete comment: %v", err)
	}
	if rest, _ := repo.GetComments(uid, "task", task.ID); len(rest) != 0 {
		t.Fatalf("删除后不该还有评论")
	}
}

// 建任务时带的标签要回填 id 与颜色，前端拿到就能渲染
func TestCreateTaskFillsLabels(t *testing.T) {
	repo := newTestRepo(t)
	uid := newUser(t, repo, "max@example.com")

	task := &model.Task{Title: "带标签", Category: "todo",
		Labels: []model.Label{{Name: "重要"}}}
	if err := repo.CreateTask(uid, task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	if len(task.Labels) != 1 {
		t.Fatalf("应当回填 1 个标签: %+v", task.Labels)
	}
	if task.Labels[0].ID == 0 {
		t.Fatalf("标签 id 应当回填: %+v", task.Labels[0])
	}
	if task.Labels[0].Color != "#1890ff" {
		t.Fatalf("标签默认颜色应当回填: %+v", task.Labels[0])
	}

	// 更新后同样回填
	task.Labels = []model.Label{{Name: "紧急", Color: "#f00"}}
	if err := repo.UpdateTask(uid, task); err != nil {
		t.Fatalf("update task: %v", err)
	}
	if len(task.Labels) != 1 || task.Labels[0].ID == 0 || task.Labels[0].Color != "#f00" {
		t.Fatalf("更新后标签没回填: %+v", task.Labels)
	}
}

// 建/改任务时一并回填项目名，前端不用再查一次
func TestTaskFillsProjectName(t *testing.T) {
	repo := newTestRepo(t)
	uid := newUser(t, repo, "max@example.com")

	project := &model.Project{Name: "门店", Status: "active"}
	if err := repo.CreateProject(uid, project); err != nil {
		t.Fatalf("create project: %v", err)
	}
	task := &model.Task{Title: "排班", Category: "todo", ProjectID: &project.ID}
	if err := repo.CreateTask(uid, task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	if task.ProjectName != "门店" {
		t.Fatalf("建任务应回填项目名: %q", task.ProjectName)
	}

	task.ProjectID = nil
	if err := repo.UpdateTask(uid, task); err != nil {
		t.Fatalf("update task: %v", err)
	}
	if reloaded, _ := repo.GetTask(uid, task.ID); reloaded.ProjectID != nil {
		t.Fatalf("解除项目关联没生效: %+v", reloaded.ProjectID)
	}
}
