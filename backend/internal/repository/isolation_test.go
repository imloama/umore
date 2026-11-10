package repository

import (
	"errors"
	"testing"

	"github.com/imloama/umore/internal/model"

	"gorm.io/gorm"
)

// 多用户隔离：A 的数据在 B 的视角下必须完全不可见、不可改、不可删。
// 这是本轮 user_id 改造的核心约束，所以两种库都跑一遍。
func TestUserIsolation(t *testing.T) {
	eachRepo(t, func(t *testing.T, repo *Repository) {
		alice := newUser(t, repo, "alice@example.com")
		bob := newUser(t, repo, "bob@example.com")

		// Alice 建一整套数据
		project := &model.Project{Name: "Alice 的单", Status: "active", Amount: model.Money("1000")}
		if err := repo.CreateProject(alice, project); err != nil {
			t.Fatalf("create project: %v", err)
		}
		task := &model.Task{Title: "Alice 的活", Category: "todo", ProjectID: &project.ID,
			Labels: []model.Label{{Name: "alice-tag"}}}
		if err := repo.CreateTask(alice, task); err != nil {
			t.Fatalf("create task: %v", err)
		}
		if err := repo.CreateFinance(alice, &model.Finance{Type: "income", Amount: model.Money("100"),
			Date: "2026-11-01", ProjectID: &project.ID}); err != nil {
			t.Fatalf("create finance: %v", err)
		}
		if err := repo.CreateReceivable(alice, &model.Receivable{Title: "首款",
			Amount: model.Money("500"), ProjectID: &project.ID}); err != nil {
			t.Fatalf("create receivable: %v", err)
		}
		idea := &model.Idea{Content: "Alice 的灵感"}
		if err := repo.CreateIdea(alice, idea); err != nil {
			t.Fatalf("create idea: %v", err)
		}
		if err := repo.CreateAttachment(alice, &model.Attachment{Filename: "a.png", MimeType: "image/png", Size: 1, Path: "p"}); err != nil {
			t.Fatalf("create attachment: %v", err)
		}

		// Bob 一个都看不见
		if list, _ := repo.GetProjects(bob); len(list) != 0 {
			t.Fatalf("B 不该看到 A 的项目: %+v", list)
		}
		if list, _ := repo.GetTasks(bob, model.TaskFilter{}); len(list) != 0 {
			t.Fatalf("B 不该看到 A 的任务: %+v", list)
		}
		if list, _ := repo.GetLabels(bob); len(list) != 0 {
			t.Fatalf("B 不该看到 A 的标签: %+v", list)
		}
		if list, _ := repo.GetFinances(bob, "2000-01-01", "2099-01-01"); len(list) != 0 {
			t.Fatalf("B 不该看到 A 的流水: %+v", list)
		}
		if list, _ := repo.GetReceivables(bob, "", 0); len(list) != 0 {
			t.Fatalf("B 不该看到 A 的待收款: %+v", list)
		}
		if list, _ := repo.GetIdeas(bob, ""); len(list) != 0 {
			t.Fatalf("B 不该看到 A 的灵感: %+v", list)
		}

		// 拿 id 直查也拿不到：越权一律当成不存在
		if one, _ := repo.GetTask(bob, task.ID); one != nil {
			t.Fatalf("B 不该按 id 拿到 A 的任务: %+v", one)
		}
		if one, _ := repo.GetProject(bob, project.ID); one != nil {
			t.Fatalf("B 不该按 id 拿到 A 的项目: %+v", one)
		}
		if one, _ := repo.GetReceivable(bob, 1); one != nil {
			t.Fatalf("B 不该按 id 拿到 A 的待收款: %+v", one)
		}
		if one, _ := repo.GetIdea(bob, idea.ID); one != nil {
			t.Fatalf("B 不该按 id 拿到 A 的灵感: %+v", one)
		}

		// 改与删都影响不到 A 的数据：越权一律按「记录不存在」处理，接口层回 404。
		// 更新时先把 project_id 摘掉：那是 A 的项目 id，B 手上没有，会先被关联校验挡下。
		clone := *task
		clone.Title = "被 B 改了"
		clone.ProjectID = nil
		if err := repo.UpdateTask(bob, &clone); !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Fatalf("B 改 A 的任务应报 not found，实际 %v", err)
		}
		if err := repo.DeleteTask(bob, task.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Fatalf("B 删 A 的任务应报 not found，实际 %v", err)
		}
		if err := repo.DeleteProject(bob, project.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Fatalf("B 删 A 的项目应报 not found，实际 %v", err)
		}
		if err := repo.UpdateIdea(bob, &model.Idea{ID: idea.ID, Content: "被 B 改了"}); !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Fatalf("B 改 A 的灵感应报 not found，实际 %v", err)
		}
		if err := repo.DeleteIdea(bob, idea.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Fatalf("B 删 A 的灵感应报 not found，实际 %v", err)
		}
		// 关联校验先撞上：B 手上没有 A 的项目 id
		if err := repo.SetIdeaProject(bob, idea.ID, &project.ID); !errors.Is(err, ErrProjectNotFound) {
			t.Fatalf("B 把灵感挂到别人项目上应被拦住，实际 %v", err)
		}

		if again, _ := repo.GetTask(alice, task.ID); again == nil || again.Title != "Alice 的活" {
			t.Fatalf("A 的任务被动了: %+v", again)
		}
		if again, _ := repo.GetProject(alice, project.ID); again == nil {
			t.Fatal("A 的项目被删了")
		}

		// B 建同名项目/标签，各归各的，不冲突也不串
		if err := repo.CreateProject(bob, &model.Project{Name: "Alice 的单", Status: "active"}); err != nil {
			t.Fatalf("不同用户名下同名项目应当允许: %v", err)
		}
		if err := repo.CreateLabel(bob, &model.Label{Name: "alice-tag"}); err != nil {
			t.Fatalf("不同用户名下同名标签应当允许: %v", err)
		}
		if labels, _ := repo.GetLabels(bob); len(labels) != 1 || labels[0].TaskCount != 0 {
			t.Fatalf("B 的标签计数不该算上 A 的任务: %+v", labels)
		}

		// 汇总只算自己的
		if sum, _ := repo.GetReceivableSummary(bob); sum.PendingCount != 0 {
			t.Fatalf("B 的待收款汇总不该有数: %+v", sum)
		}
		if sum, _ := repo.GetFinanceSummary(bob, "2000-01-01", "2099-01-01"); !sum.TotalIncome.IsZero() {
			t.Fatalf("B 的财务汇总不该有数: %+v", sum)
		}

		// 任务/收款不能挂到别人的项目上（否则会造出指向别人项目的脏记录）
		if err := repo.CreateTask(bob, &model.Task{Title: "蹭项目", Category: "todo",
			ProjectID: &project.ID}); !errors.Is(err, ErrProjectNotFound) {
			t.Fatalf("挂到别人项目上应被拦住，实际 %v", err)
		}
		if err := repo.CreateFinance(bob, &model.Finance{Type: "income", Amount: model.Money("1"),
			Date: "2026-11-01", ProjectID: &project.ID}); !errors.Is(err, ErrProjectNotFound) {
			t.Fatalf("流水挂到别人项目上应被拦住，实际 %v", err)
		}
		if err := repo.CreateReceivable(bob, &model.Receivable{Title: "蹭",
			Amount: model.Money("1"), ProjectID: &project.ID}); !errors.Is(err, ErrProjectNotFound) {
			t.Fatalf("待收款挂到别人项目上应被拦住，实际 %v", err)
		}
	})
}

// 用户表本身：邮箱唯一、大小写不敏感
func TestCreateUserUniqueness(t *testing.T) {
	eachRepo(t, func(t *testing.T, repo *Repository) {
		newUser(t, repo, "max@example.com")

		if _, err := repo.CreateUser("MAX@example.com", "h", "s", true); err != ErrEmailTaken {
			t.Fatalf("重复邮箱（大小写不同）应返回 ErrEmailTaken，实际 %v", err)
		}
		if user, _ := repo.GetUserByEmail("Max@Example.com"); user == nil {
			t.Fatal("按邮箱查询应大小写不敏感")
		}
		if count, _ := repo.CountUsers(); count != 1 {
			t.Fatalf("want 1 user, got %d", count)
		}
	})
}
