package repository

import (
	"testing"
	"time"

	"github.com/imloama/umore/internal/model"
)

// 待收款：记一笔、改备注、标记已收、逾期与剩余天数都算对
func TestReceivableLifecycle(t *testing.T) {
	eachRepo(t, func(t *testing.T, repo *Repository) {
		uid := newUser(t, repo, "max@example.com")

		project := &model.Project{Name: "门店预约系统", Status: "active", Amount: model.Money("60000")}
		if err := repo.CreateProject(uid, project); err != nil {
			t.Fatalf("create project: %v", err)
		}

		past := time.Now().AddDate(0, 0, -3).Format("2006-01-02")
		soon := time.Now().AddDate(0, 0, 2).Format("2006-01-02")

		overdue := &model.Receivable{ProjectID: &project.ID, Title: "首付款 30%",
			Amount: model.Money("18000"), DueDate: &past, Note: "合同签订后 3 天"}
		willDue := &model.Receivable{ProjectID: &project.ID, Title: "尾款",
			Amount: model.Money("42000"), DueDate: &soon}
		for _, item := range []*model.Receivable{overdue, willDue} {
			if err := repo.CreateReceivable(uid, item); err != nil {
				t.Fatalf("create receivable: %v", err)
			}
		}

		list, err := repo.GetReceivables(uid, "pending", 0)
		if err != nil {
			t.Fatalf("list receivables: %v", err)
		}
		if len(list) != 2 {
			t.Fatalf("want 2 pending, got %d", len(list))
		}
		// 按到期日升序：逾期的排前面
		if list[0].Title != "首付款 30%" {
			t.Fatalf("排序不对，第一条是 %q", list[0].Title)
		}
		if !list[0].Overdue || list[0].DaysLeft != -3 {
			t.Fatalf("逾期判断不对: %+v", list[0])
		}
		if list[1].Overdue || list[1].DaysLeft != 2 {
			t.Fatalf("未到期判断不对: %+v", list[1])
		}
		if list[0].ProjectName != "门店预约系统" {
			t.Fatalf("项目名没带出来: %q", list[0].ProjectName)
		}

		// 备注随时改
		overdue.Note = "客户说月底给"
		overdue.Amount = model.Money("20000")
		if err := repo.UpdateReceivable(uid, overdue); err != nil {
			t.Fatalf("update receivable: %v", err)
		}
		reloaded, _ := repo.GetReceivable(uid, overdue.ID)
		if reloaded.Note != "客户说月底给" || !reloaded.Amount.Equal(model.Money("20000").Decimal) {
			t.Fatalf("改备注没生效: %+v", reloaded)
		}
		if !reloaded.Overdue {
			t.Fatalf("改完备注不该影响逾期判断: %+v", reloaded)
		}

		// 汇总：一笔逾期 + 一笔 7 天内到期
		sum, err := repo.GetReceivableSummary(uid)
		if err != nil {
			t.Fatalf("summary: %v", err)
		}
		if sum.PendingCount != 2 || !sum.PendingAmount.Equal(model.Money("62000").Decimal) {
			t.Fatalf("未收合计不对: %+v", sum)
		}
		if sum.OverdueCount != 1 || !sum.OverdueAmount.Equal(model.Money("20000").Decimal) {
			t.Fatalf("逾期合计不对: %+v", sum)
		}
		if sum.DueSoonCount != 1 || !sum.DueSoonAmount.Equal(model.Money("42000").Decimal) {
			t.Fatalf("7 天内到期不对: %+v", sum)
		}

		// 标记已收 -> 不再参与未收/逾期
		if err := repo.SetReceivableStatus(uid, overdue.ID, true); err != nil {
			t.Fatalf("set status: %v", err)
		}
		done, _ := repo.GetReceivable(uid, overdue.ID)
		if done.Status != "done" || done.DoneAt == nil || done.Overdue {
			t.Fatalf("已收状态不对: %+v", done)
		}
		sum, _ = repo.GetReceivableSummary(uid)
		if sum.PendingCount != 1 || sum.OverdueCount != 0 || sum.DoneCount != 1 ||
			!sum.DoneAmount.Equal(model.Money("20000").Decimal) {
			t.Fatalf("标记已收后汇总不对: %+v", sum)
		}

		// 撤回已收
		if err := repo.SetReceivableStatus(uid, overdue.ID, false); err != nil {
			t.Fatalf("revert status: %v", err)
		}
		back, _ := repo.GetReceivable(uid, overdue.ID)
		if back.Status != "pending" || back.DoneAt != nil {
			t.Fatalf("撤回已收没生效: %+v", back)
		}

		if err := repo.DeleteReceivable(uid, willDue.ID); err != nil {
			t.Fatalf("delete receivable: %v", err)
		}
		if rest, _ := repo.GetReceivables(uid, "", 0); len(rest) != 1 {
			t.Fatalf("删除后应该只剩 1 条，实际 %d", len(rest))
		}
	})
}

// 项目上的待收款汇总：改收款计划后立刻反映
func TestProjectReceivableSummary(t *testing.T) {
	repo := newTestRepo(t)
	uid := newUser(t, repo, "max@example.com")

	project := &model.Project{Name: "跨境电商小程序", Status: "active", Amount: model.Money("90000")}
	if err := repo.CreateProject(uid, project); err != nil {
		t.Fatalf("create project: %v", err)
	}
	past := time.Now().AddDate(0, 0, -1).Format("2006-01-02")

	item := &model.Receivable{ProjectID: &project.ID, Title: "首款", Amount: model.Money("30000"), DueDate: &past}
	if err := repo.CreateReceivable(uid, item); err != nil {
		t.Fatalf("create receivable: %v", err)
	}
	if err := repo.CreateReceivable(uid, &model.Receivable{ProjectID: &project.ID, Title: "中款",
		Amount: model.Money("30000")}); err != nil {
		t.Fatalf("create receivable: %v", err)
	}
	// 没挂项目的私人垫款，不该混进项目汇总
	if err := repo.CreateReceivable(uid, &model.Receivable{Title: "帮朋友垫的",
		Amount: model.Money("500")}); err != nil {
		t.Fatalf("create receivable: %v", err)
	}

	got, _ := repo.GetProject(uid, project.ID)
	if got.ReceivableCount != 2 || !got.ReceivableAmount.Equal(model.Money("60000").Decimal) {
		t.Fatalf("项目待收款汇总不对: %+v", got)
	}
	if !got.ReceivableOverdue.Equal(model.Money("30000").Decimal) {
		t.Fatalf("项目逾期金额不对: %+v", got)
	}
	if got.NextDueDate != past {
		t.Fatalf("最近到期日不对: %q", got.NextDueDate)
	}

	if err := repo.SetReceivableStatus(uid, item.ID, true); err != nil {
		t.Fatalf("set status: %v", err)
	}
	after, _ := repo.GetProject(uid, project.ID)
	if after.ReceivableCount != 1 || !after.ReceivableAmount.Equal(model.Money("30000").Decimal) ||
		!after.ReceivableOverdue.IsZero() {
		t.Fatalf("标记已收后项目汇总不对: %+v", after)
	}
}

// 排序：未收在前，没填到期日的垫后
func TestReceivableOrdering(t *testing.T) {
	repo := newTestRepo(t)
	uid := newUser(t, repo, "max@example.com")

	early, late := "2020-01-01", "2030-01-01"
	items := []*model.Receivable{
		{Title: "无到期日", Amount: model.Money("1")},
		{Title: "早到期", Amount: model.Money("2"), DueDate: &early},
		{Title: "晚到期", Amount: model.Money("3"), DueDate: &late},
		{Title: "已收", Amount: model.Money("4"), Status: "done"},
	}
	for _, item := range items {
		if err := repo.CreateReceivable(uid, item); err != nil {
			t.Fatalf("create receivable %s: %v", item.Title, err)
		}
	}

	list, err := repo.GetReceivables(uid, "", 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	want := []string{"早到期", "晚到期", "无到期日", "已收"}
	if len(list) != len(want) {
		t.Fatalf("want %d rows, got %d", len(want), len(list))
	}
	for i := range want {
		if list[i].Title != want[i] {
			t.Fatalf("排序不对，want %v", want)
		}
	}
}
