package repository

import (
	"testing"
	"time"

	"github.com/imloama/umore/internal/model"
)

// 今日口径有两条路：
//  1. 今天该做：没完成 + 开始时间（计划日期，缺了看创建日期）≤ 今天 ≤ 结束时间（截止日期，空着算一直有效）；
//  2. 今天收的尾：今天完成的，日期区间在哪天都算。
//
// 一条时间都没排的（未开始，也没有开始和结束时间）不算今天的活：那是待办池里的存货。
// 手动「加入今日」不算数，那是列表里的钉住，不是统计口径。
func TestTodayRule(t *testing.T) {
	eachRepo(t, func(t *testing.T, repo *Repository) {
		uid := newUser(t, repo, "today@example.com")
		today := repo.TodayFor(uid)

		day := func(offset int) *string {
			v := time.Now().AddDate(0, 0, offset).Format("2006-01-02")
			return &v
		}

		cases := []struct {
			name    string
			task    model.Task
			expects bool
		}{
			{"今天排期、没截止", model.Task{Title: "今天排", ScheduledDate: day(0)}, true},
			// 没排开始也没排结束：还在待办池里，不算今天的活
			{"没排期、没截止", model.Task{Title: "没排期"}, false},
			{"只排了截止日期", model.Task{Title: "只排了截止", DueDate: day(3)}, true},
			{"排期区间跨今天", model.Task{Title: "跨天", ScheduledDate: day(-2), DueDate: day(3)}, true},
			{"明天才开始", model.Task{Title: "未开始", ScheduledDate: day(1), DueDate: day(5)}, false},
			{"昨天就到期了", model.Task{Title: "已过期", ScheduledDate: day(-5), DueDate: day(-1)}, false},
			{"今天到期", model.Task{Title: "今天截止", ScheduledDate: day(-3), DueDate: day(0)}, true},
			// 今天勾掉的都算今天的活，哪怕它压根没排到今天
			{"已完成的今天该做", model.Task{Title: "做完了", ScheduledDate: day(0), Status: "DONE"}, true},
			{"已完成但区间在今天之后", model.Task{Title: "提前做完", ScheduledDate: day(2), DueDate: day(9), Status: "DONE"}, true},
			{"已完成且早就过期", model.Task{Title: "补了个旧账", ScheduledDate: day(-9), DueDate: day(-3), Status: "DONE"}, true},
			// 手动钉在今日，但排期在明年：今天并不该做
			{"手动加今日但其实没开始", model.Task{Title: "钉住的", ScheduledDate: day(30), IsToday: true}, false},
		}

		for _, c := range cases {
			task := c.task
			if task.Status == "" {
				task.Status = "TODO"
			}
			task.Category = "todo"
			if err := repo.CreateTask(uid, &task); err != nil {
				t.Fatalf("create %s: %v", c.name, err)
			}
			tasks, err := repo.GetTasks(uid, model.TaskFilter{TodayOnly: true})
			if err != nil {
				t.Fatalf("filter today: %v", err)
			}
			got := false
			for _, item := range tasks {
				if item.ID == task.ID {
					got = true
				}
			}
			if got != c.expects {
				t.Errorf("%s: 今日判定应为 %v，实际 %v", c.name, c.expects, got)
			}
		}

		// 列表里的 today 标记跟筛选、统计是同一口径
		all, err := repo.GetTasks(uid, model.TaskFilter{})
		if err != nil {
			t.Fatalf("list all: %v", err)
		}
		flagged := 0
		for _, task := range all {
			if task.TodayFlag {
				flagged++
			}
			if task.Title == "未开始" && task.TodayFlag {
				t.Errorf("未开始的任务不该标成今天")
			}
		}
		if flagged != 7 {
			t.Fatalf("today 标记应有 7 条，实际 %d", flagged)
		}

		// 统计跟列表同一口径：12 条里 7 条算今天（4 条没做完该做的 + 3 条今天收的尾）
		stats, err := repo.GetTaskStats(uid)
		if err != nil {
			t.Fatalf("stats: %v", err)
		}
		if stats.TodayTotal != 7 {
			t.Fatalf("今日统计应为 7，实际 %d", stats.TodayTotal)
		}
		if stats.TodayDone != 3 {
			t.Fatalf("今日完成应为 3（今天勾掉的都算），实际 %d", stats.TodayDone)
		}
		if stats.TodayDate != today {
			t.Fatalf("统计日期应为 %s，实际 %s", today, stats.TodayDate)
		}

		// 取反口径：今日页的「已过期 / 未开始」
		others, err := repo.GetTasks(uid, model.TaskFilter{TodayOnly: true, Today: "!" + today})
		if err != nil {
			t.Fatalf("filter not today: %v", err)
		}
		// 未开始、已过期、没排期，外加钉住但排期在明年那条；今天完成的不在这个栏里
		if len(others) != 4 {
			t.Fatalf("不该算今天的有 4 条，实际 %d", len(others))
		}
		for _, task := range others {
			if task.Status == "DONE" {
				t.Errorf("今天完成的 %q 不该落进「已过期 / 未开始」", task.Title)
			}
		}
	})
}

// 提前收工：原计划在后天，今天提前做完 —— 得进今天的统计，状态是已完成。
func TestFinishEarlyShowsInToday(t *testing.T) {
	eachRepo(t, func(t *testing.T, repo *Repository) {
		uid := newUser(t, repo, "early@example.com")
		today := repo.TodayFor(uid)
		tomorrow := time.Now().AddDate(0, 0, 2).Format("2006-01-02")

		task := model.Task{Title: "提前做完的", Category: "todo", Status: "TODO", ScheduledDate: &tomorrow}
		if err := repo.CreateTask(uid, &task); err != nil {
			t.Fatalf("create: %v", err)
		}
		stats, err := repo.GetTaskStats(uid)
		if err != nil {
			t.Fatalf("stats: %v", err)
		}
		if stats.TodayTotal != 0 {
			t.Fatalf("排在后天的任务不该算今天的活，实际 %d", stats.TodayTotal)
		}

		// 前端提交「完成+完成时间」时，日期区间还是原样传回来的
		task.Status = "DONE"
		closed := time.Now()
		task.ClosedAt = &closed
		task.ScheduledDate = &tomorrow
		if err := repo.UpdateTask(uid, &task); err != nil {
			t.Fatalf("update: %v", err)
		}

		saved, err := repo.GetTask(uid, task.ID)
		if err != nil || saved == nil {
			t.Fatalf("get task: %v", err)
		}
		if saved.ScheduledDate == nil || *saved.ScheduledDate != today {
			t.Fatalf("提前做完的任务，开始日期应落到今天 %s，实际 %v", today, saved.ScheduledDate)
		}
		if !saved.TodayFlag {
			t.Fatalf("今天做完的任务应算今日任务")
		}

		stats, err = repo.GetTaskStats(uid)
		if err != nil {
			t.Fatalf("stats: %v", err)
		}
		if stats.TodayTotal != 1 || stats.TodayDone != 1 {
			t.Fatalf("今日统计应为 1/1，实际 %d/%d", stats.TodayTotal, stats.TodayDone)
		}

		// 列表口径同上：今日该做里有它，状态已完成
		list, err := repo.GetTasks(uid, model.TaskFilter{TodayOnly: true})
		if err != nil {
			t.Fatalf("list today: %v", err)
		}
		if len(list) != 1 || list[0].Status != "DONE" {
			t.Fatalf("今日列表应只有刚做完那条且状态 DONE，实际 %d 条", len(list))
		}

		// 收回完成状态：开始日期已经落到今天，那就回到「今天该做」——还在今天的统计里，只是不算完成
		list[0].Status = "TODO"
		list[0].ClosedAt = nil
		if err := repo.UpdateTask(uid, &list[0]); err != nil {
			t.Fatalf("reopen: %v", err)
		}
		stats, err = repo.GetTaskStats(uid)
		if err != nil {
			t.Fatalf("stats: %v", err)
		}
		if stats.TodayTotal != 1 || stats.TodayDone != 0 {
			t.Fatalf("收回完成后应回到今日待办 1/0，实际 %d/%d", stats.TodayTotal, stats.TodayDone)
		}
	})
}

// 昨天做完了、今天还在翻出来看，不该冒进今天的统计。
func TestFinishedYesterdayNotInToday(t *testing.T) {
	eachRepo(t, func(t *testing.T, repo *Repository) {
		uid := newUser(t, repo, "yesterday@example.com")
		task := model.Task{Title: "昨天做完了", Category: "todo", Status: "DONE"}
		closed := time.Now().AddDate(0, 0, -1)
		task.ClosedAt = &closed
		if err := repo.CreateTask(uid, &task); err != nil {
			t.Fatalf("create: %v", err)
		}
		// 库里只留完成时间，创建时间改到前天，模拟「昨天干的活」
		old := time.Now().AddDate(0, 0, -2)
		if err := repo.db.Model(&model.Task{}).Where("id = ?", task.ID).
			Update("created_at", old).Error; err != nil {
			t.Fatalf("backdate: %v", err)
		}

		stats, err := repo.GetTaskStats(uid)
		if err != nil {
			t.Fatalf("stats: %v", err)
		}
		if stats.TodayTotal != 0 || stats.TodayDone != 0 {
			t.Fatalf("昨天完成的不该算今天，实际 %d/%d", stats.TodayTotal, stats.TodayDone)
		}
	})
}

// 「今天」按用户设置的时区切：同一时刻，UTC-11 还在昨天，UTC+14 已经到明天了。
func TestTodayFollowsUserTimezone(t *testing.T) {
	repo := newTestRepo(t)
	uid := newUser(t, repo, "tz@example.com")

	// 默认 UTC+8
	user, err := repo.GetUser(uid)
	if err != nil || user == nil {
		t.Fatalf("get user: %v", err)
	}
	if user.Timezone != model.DefaultTimezone {
		t.Fatalf("默认时区应为 %s，实际 %q", model.DefaultTimezone, user.Timezone)
	}

	// 换成地球最西边的时区：那边的「今天」要么跟 UTC+8 同一天，要么是前一天
	if err := repo.UpdateProfile(uid, user.Nickname, user.Avatar, "Pacific/Midway"); err != nil {
		t.Fatalf("update timezone: %v", err)
	}
	west := repo.TodayFor(uid)
	if west != model.TodayIn("Pacific/Midway") {
		t.Fatalf("Pacific/Midway 的今天应为 %s，实际 %s", model.TodayIn("Pacific/Midway"), west)
	}

	// 换到最东边：日期只会往前走，不该比 UTC+8 更早
	if err := repo.UpdateProfile(uid, user.Nickname, user.Avatar, "Pacific/Kiritimati"); err != nil {
		t.Fatalf("update timezone: %v", err)
	}
	east := repo.TodayFor(uid)
	if east < model.TodayIn(model.DefaultTimezone) {
		t.Fatalf("UTC+14 的今天 %s 不该早于 UTC+8 的 %s", east, model.TodayIn(model.DefaultTimezone))
	}

	// 认不出来的时区退回默认值，不炸
	if err := repo.UpdateProfile(uid, user.Nickname, user.Avatar, "Mars/Olympus"); err != nil {
		t.Fatalf("update timezone: %v", err)
	}
	if got := repo.TimezoneFor(uid); got != model.DefaultTimezone {
		t.Fatalf("非法时区应退回 %s，实际 %s", model.DefaultTimezone, got)
	}

	// UTC+8 这类手写偏移也认
	if got := model.NormalizeTimezone("UTC+5:30"); got != "UTC+5:30" {
		t.Fatalf("手写偏移应原样保留，实际 %s", got)
	}
	if got := model.NormalizeTimezone("  "); got != model.DefaultTimezone {
		t.Fatalf("空时区应退回默认，实际 %q", got)
	}
}

// 没排开始、也没排结束的任务（未开始的一条存货）不该混进今日列表、今日统计，
// 但从今日页的「已过期 / 未开始」里还得能找着它，否则它就彻底失联了。
func TestUnscheduledNotInToday(t *testing.T) {
	eachRepo(t, func(t *testing.T, repo *Repository) {
		uid := newUser(t, repo, "unscheduled@example.com")
		today := repo.TodayFor(uid)

		task := model.Task{Title: "还没排期", Category: "todo", Status: "TODO"}
		if err := repo.CreateTask(uid, &task); err != nil {
			t.Fatalf("create: %v", err)
		}
		// 手动钉成今日也没用：钉住只影响排序，不改今日口径
		if err := repo.SetTaskToday(uid, task.ID, true, 0); err != nil {
			t.Fatalf("pin today: %v", err)
		}

		list, err := repo.GetTasks(uid, model.TaskFilter{TodayOnly: true})
		if err != nil {
			t.Fatalf("list today: %v", err)
		}
		if len(list) != 0 {
			t.Fatalf("没排期的任务不该进今日列表，实际 %d 条", len(list))
		}

		stats, err := repo.GetTaskStats(uid)
		if err != nil {
			t.Fatalf("stats: %v", err)
		}
		if stats.TodayTotal != 0 {
			t.Fatalf("没排期的任务不该进今日统计，实际 %d", stats.TodayTotal)
		}

		others, err := repo.GetTasks(uid, model.TaskFilter{TodayOnly: true, Today: "!" + today})
		if err != nil {
			t.Fatalf("list not today: %v", err)
		}
		if len(others) != 1 || others[0].ID != task.ID {
			t.Fatalf("没排期的任务应落在「已过期 / 未开始」，实际 %+v", others)
		}

		saved, err := repo.GetTask(uid, task.ID)
		if err != nil || saved == nil {
			t.Fatalf("get task: %v", err)
		}
		if saved.TodayFlag {
			t.Fatalf("没排期的任务不该被打上今日标记")
		}
	})
}

// 只排了截止日期、没排开始日期的，仍然是今天的活：开始时间缺省看创建日期。
func TestDueOnlyStillInToday(t *testing.T) {
	eachRepo(t, func(t *testing.T, repo *Repository) {
		uid := newUser(t, repo, "dueonly@example.com")
		due := time.Now().AddDate(0, 0, 3).Format("2006-01-02")
		task := model.Task{Title: "只排了截止", Category: "todo", Status: "TODO", DueDate: &due}
		if err := repo.CreateTask(uid, &task); err != nil {
			t.Fatalf("create: %v", err)
		}
		stats, err := repo.GetTaskStats(uid)
		if err != nil {
			t.Fatalf("stats: %v", err)
		}
		if stats.TodayTotal != 1 {
			t.Fatalf("只排了截止日期的任务算今天，实际 %d", stats.TodayTotal)
		}
	})
}
