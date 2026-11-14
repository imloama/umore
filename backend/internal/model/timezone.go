package model

import (
	"fmt"
	"strings"
	"time"
)

// DefaultTimezone 用户没设置过时区时按 UTC+8 算「今天」。
// 这个「今天」决定今日任务、今日统计按哪天切，跟用户在哪、手机系统时区是什么无关。
const DefaultTimezone = "Asia/Shanghai"

// commonTimezones 是给个人中心下拉用的候选时区，值是 IANA 名字（带夏令时的地区会自己处理）。
// 只列常用的，不把 tzdata 全量（四百多条）塞给前端；真正校验还是走 time.LoadLocation。
var commonTimezones = []string{
	"UTC",
	"Asia/Shanghai",
	"Asia/Hong_Kong",
	"Asia/Taipei",
	"Asia/Tokyo",
	"Asia/Seoul",
	"Asia/Singapore",
	"Asia/Bangkok",
	"Asia/Kolkata",
	"Asia/Dubai",
	"Europe/London",
	"Europe/Paris",
	"Europe/Berlin",
	"Europe/Moscow",
	"America/New_York",
	"America/Chicago",
	"America/Denver",
	"America/Los_Angeles",
	"America/Sao_Paulo",
	"Australia/Sydney",
	"Pacific/Auckland",
}

// CommonTimezones 个人中心下拉的候选时区
func CommonTimezones() []string {
	return append([]string(nil), commonTimezones...)
}

// NormalizeTimezone 校验并归一化时区名：空值、认不出来的都退回默认值，
// 所以调用方永远拿到一个能 LoadLocation 的名字。
func NormalizeTimezone(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return DefaultTimezone
	}
	if loc, err := time.LoadLocation(name); err == nil {
		return loc.String()
	}
	// 手填的 UTC+8 这类写法也认，原样留着当名字（前端就能直接回显用户填的那串）
	if _, err := parseOffset(name); err == nil {
		return name
	}
	return DefaultTimezone
}

// parseOffset 把 "UTC+8" / "GMT-3" / "UTC+5:30" 解析成固定偏移时区
func parseOffset(name string) (*time.Location, error) {
	upper := strings.ToUpper(name)
	var rest string
	switch {
	case strings.HasPrefix(upper, "UTC"):
		rest = name[3:]
	case strings.HasPrefix(upper, "GMT"):
		rest = name[3:]
	default:
		return nil, fmt.Errorf("unknown timezone %q", name)
	}
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return time.UTC, nil
	}
	var sign int
	switch rest[0] {
	case '+':
		sign = 1
	case '-':
		sign = -1
	default:
		return nil, fmt.Errorf("unknown timezone %q", name)
	}
	var hour, minute int
	if _, err := fmt.Sscanf(rest[1:], "%d:%d", &hour, &minute); err != nil {
		if _, err := fmt.Sscanf(rest[1:], "%d", &hour); err != nil {
			return nil, fmt.Errorf("unknown timezone %q", name)
		}
		minute = 0
	}
	if hour > 14 || minute > 59 {
		return nil, fmt.Errorf("unknown timezone %q", name)
	}
	return time.FixedZone(name, sign*(hour*60+minute)*60), nil
}

// LocationOf 时区名 → 时区对象；认不出来退回默认时区，不报错。
// tzdata 在 docker 里可能没装，Windows 上也可能没配，这两种情况退到 UTC+8 固定偏移。
func LocationOf(name string) *time.Location {
	name = NormalizeTimezone(name)
	if loc, err := time.LoadLocation(name); err == nil {
		return loc
	}
	if fixed, err := time.FixedZone("UTC+8", 8*3600), error(nil); name == DefaultTimezone && err == nil {
		return fixed
	}
	return time.UTC
}

// TodayIn 用户所在时区的「今天」，按 00:00 切的日期串（YYYY-MM-DD）。
// 今日任务、今日统计都以它为准：UTC 还是北京时间，差的就是这一天的归属。
func TodayIn(name string) string {
	return time.Now().In(LocationOf(name)).Format("2006-01-02")
}

// TodayCondition 判定「这条任务算不算今日任务」，三个参数都是用户时区的今天（YYYY-MM-DD）。
//
// 两条路，满足一条就算：
//  1. 今天该做的：状态没完成（DONE 的不再算「该做」）、已经开始、还没结束；
//  2. 今天收的尾：今天完成的，不管它排在哪天。今天早上开、下午勾掉的一条得算进去，
//     否则「今天干了什么」在统计里查无此人，完成率还只算剩活的那些。
//
// 所以「开始时间 ≤ 今天 ≤ 结束时间」不成立的任务不会自动进今日，
// 也不会因为手动点过一次「加入今日」就永久算在今天。
//
// 一条都不排期的（未开始、也没有开始和结束时间）不算今天的活：
// 那种任务是「待办池」里的存货，还没轮到今天。见 HasWindow。
//
// 两条路都避开了在 SQL 里切 created_at：它在 SQLite 里是日期字符串、在 Postgres 里是 timestamptz，
// 同一个函数切不出同一种结果（PG 的 substr 根本不认 timestamptz），硬拼方言函数又会把
// 本该跨库一致的用例撕成两份。所以这里只写能跨库比较的东西：
//   - 没排期的任务先当「今天该做」，创建日期那一刀和「压根没排期」那一刀由调用方用
//     IsStarted / HasWindow 补（CreatedAt 是系统写的时间，正常不会晚于今天，多筛掉的只是脏数据）；
//   - 「今天完成」那半截，完成时间在 SQLite 里是带时区的文本、在 PG 里是 timestamptz，
//     于是把这段表达式交给 ClosedAtExpr 按方言拼，PG 那边顺带多一个时区参数。
const TodayCondition = `((tasks.status <> 'DONE'
	AND (COALESCE(tasks.scheduled_date, '') <> '' OR COALESCE(tasks.due_date, '') <> '')
	AND (tasks.scheduled_date IS NULL OR tasks.scheduled_date = '' OR tasks.scheduled_date <= ?)
	AND (tasks.due_date IS NULL OR tasks.due_date = '' OR tasks.due_date >= ?))
	OR (tasks.status = 'DONE' AND substr(%s, 1, 10) = ?))`

// ClosedAtExpr 把「完成时间」换算成用户时区里的日期串（YYYY-MM-DD），喂进 TodayCondition 的 %s。
//
//   - SQLite：Go 写进去的是 "2026-11-12 08:00:00.1+08:00" 这种带时区的文本，date() 解析不出来，截前 10 位；
//   - Postgres：真 timestamptz，用 timezone(时区, 时间) 转进用户时区再取日期，代价是带一个 ? 参数（时区名）。
//
// 优先看完成时间 closed_at；老数据可能没这个值，退回 updated_at 兜底
// （改了备注又没动状态的，理论上会跟着冒出今天，但比起整天不显示，宁可多算）。
func ClosedAtExpr(driver string) string {
	if driver == "postgres" {
		return `to_char(timezone(?, COALESCE(tasks.closed_at, tasks.updated_at)), 'YYYY-MM-DD')`
	}
	return `substr(COALESCE(tasks.closed_at, tasks.updated_at), 1, 10)`
}

// NotDoneAndOpen 是 TodayCondition 里「状态没完成、截止日期还没过」这半截的 Go 版。
// 只在 Go 侧要判定整条规则成不成立时用（「今天不算」那个取反查询），
// 日期那半截交给 IsStarted，两边拼起来才是完整规则。
func NotDoneAndOpen(t Task, today string) bool {
	if t.Status == "DONE" {
		return false
	}
	if t.DueDate == nil || strings.TrimSpace(*t.DueDate) == "" {
		return true
	}
	return strings.TrimSpace(*t.DueDate) >= today
}

// FinishedOn 这条任务是不是在「用户时区的今天」那天收的尾。
// timezone 是用户设置的时区名（那一刀得跟 SQL 里 ClosedAtExpr 切的同一天），
// today 是同一时区的日期串（model.TodayIn(timezone)）。
//
// 跟 TodayCondition 里「今天完成」那半截是一个意思，只是换成 Go 算：
// 取反筛选、列表分组这些 SQL 条件写不顺的地方拿它对一下。完成时间优先看 closed_at，
// 老数据没有就退回 updated_at，跟 SQL 那边保持一致。
func FinishedOn(t Task, timezone, today string) bool {
	if t.Status != "DONE" {
		return false
	}
	closed := t.UpdatedAt
	if t.ClosedAt != nil {
		closed = *t.ClosedAt
	}
	return closed.In(LocationOf(timezone)).Format("2006-01-02") == today
}

// HasWindow 这条任务排过期吗：开始时间（计划日期）或结束时间（截止日期）至少有一个。
// 一个都没有意味着它还在待办池里没轮到哪天，不进今日。
func HasWindow(t Task) bool {
	return DateString(t.ScheduledDate) != "" || DateString(t.DueDate) != ""
}

// DateString 取日期字段的值，空指针、空白串都当没填。
func DateString(d *string) string {
	if d == nil {
		return ""
	}
	return strings.TrimSpace(*d)
}

// IsStarted 在 TodayCondition 的基础上，用 Go 再确认一次「这条是不是已经开始」：
// 排了期的看计划日期，没排期的看创建日期。today 是用户时区的今天（YYYY-MM-DD）。
func IsStarted(t Task, today string) bool {
	start := today
	if t.ScheduledDate != nil && strings.TrimSpace(*t.ScheduledDate) != "" {
		start = strings.TrimSpace(*t.ScheduledDate)
	} else if !t.CreatedAt.IsZero() {
		start = t.CreatedAt.Format("2006-01-02")
	}
	return start <= today
}
