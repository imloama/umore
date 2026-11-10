package repository

import (
	"errors"
	"testing"

	"github.com/imloama/umore/internal/model"

	"gorm.io/gorm"
)

// 灵感：默认待整理、状态可切、能挂到自己的项目上；别人的项目挂不上
func TestIdeaLifecycle(t *testing.T) {
	eachRepo(t, func(t *testing.T, repo *Repository) {
		user := newUser(t, repo, "idea@example.com")

		inbox := &model.Idea{Content: "  做一个读 RSS 的小工具  "}
		if err := repo.CreateIdea(user, inbox); err != nil {
			t.Fatalf("create idea: %v", err)
		}
		if inbox.ID == 0 || inbox.Status != IdeaStatusInbox {
			t.Fatalf("新灵感应是待整理: %+v", inbox)
		}
		if inbox.Content != "做一个读 RSS 的小工具" {
			t.Fatalf("首尾空白应被去掉: %q", inbox.Content)
		}

		archived := &model.Idea{Content: "过时的想法", Status: IdeaStatusArchived}
		if err := repo.CreateIdea(user, archived); err != nil {
			t.Fatalf("create archived idea: %v", err)
		}

		// 状态先归档再捞回来，正文跟着保留
		inbox.Status = IdeaStatusArchived
		inbox.Content = "做一个读 RSS 的小工具，支持播客"
		if err := repo.UpdateIdea(user, inbox); err != nil {
			t.Fatalf("update idea: %v", err)
		}
		if err := repo.UpdateIdea(user, &model.Idea{ID: inbox.ID, Content: inbox.Content, Status: "bogus"}); err != nil {
			t.Fatalf("update idea status: %v", err)
		}
		got, err := repo.GetIdea(user, inbox.ID)
		if err != nil || got == nil {
			t.Fatalf("get idea: %v %+v", err, got)
		}
		if got.Status != IdeaStatusInbox {
			t.Fatalf("非法状态应落回待整理: %+v", got)
		}
		if got.Content != "做一个读 RSS 的小工具，支持播客" {
			t.Fatalf("正文没改上: %q", got.Content)
		}

		// 按状态筛：归档的只在归档里
		if list, _ := repo.GetIdeas(user, IdeaStatusArchived); len(list) != 1 || list[0].ID != archived.ID {
			t.Fatalf("归档列表不对: %+v", list)
		}
		if list, _ := repo.GetIdeas(user, IdeaStatusInbox); len(list) != 1 || list[0].ID != inbox.ID {
			t.Fatalf("待整理列表不对: %+v", list)
		}
		if list, _ := repo.GetIdeas(user, ""); len(list) != 2 {
			t.Fatalf("不筛状态应返回全部: %+v", list)
		}

		// 挂到自己的项目上，列表能带出项目名
		project := &model.Project{Name: "RSS 工具"}
		if err := repo.CreateProject(user, project); err != nil {
			t.Fatalf("create project: %v", err)
		}
		if err := repo.SetIdeaProject(user, inbox.ID, &project.ID); err != nil {
			t.Fatalf("set idea project: %v", err)
		}
		got, _ = repo.GetIdea(user, inbox.ID)
		if got.ProjectID == nil || *got.ProjectID != project.ID || got.ProjectName != "RSS 工具" {
			t.Fatalf("项目关联没写进去: %+v", got)
		}

		// 别人的项目挂不上
		other := newUser(t, repo, "other@example.com")
		foreign := &model.Project{Name: "别人的项目"}
		if err := repo.CreateProject(other, foreign); err != nil {
			t.Fatalf("create foreign project: %v", err)
		}
		if err := repo.SetIdeaProject(user, inbox.ID, &foreign.ID); !errors.Is(err, ErrProjectNotFound) {
			t.Fatalf("挂到别人项目上应被拦住，实际 %v", err)
		}

		if err := repo.DeleteIdea(user, inbox.ID); err != nil {
			t.Fatalf("delete idea: %v", err)
		}
		if err := repo.DeleteIdea(user, inbox.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Fatalf("删两次应报 not found，实际 %v", err)
		}
	})
}
