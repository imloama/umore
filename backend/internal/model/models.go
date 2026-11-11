package model

import (
	"time"
)

// 模型层只描述「表长什么样」，不写 SQL。
// 建表交给 gorm 的 AutoMigrate（见 repository.Migrate），
// 因此这里不再有 sqlite / postgres 两套 DDL，也就不需要维护类型对照表。
//
// 多用户隔离靠一层约定，别处照这个来：
//  1. 除了 User，所有表都带 UserID，并打上 `gorm:"index;not null"`；
//  2. 仓储层的每个方法第一个参数都是 userID，读写都带 `user_id = ?`；
//  3. 关联表（task_labels）跟着主体表走，通过主体过滤，自己不重复存。

// User 账号。注册用邮箱 + 密码 + 验证码，密码只存哈希与盐。
// 昵称在注册时按「MORE + 自增 ID」自动生成，之后可在个人资料页改。
type User struct {
	ID            int    `gorm:"primaryKey" json:"id"`
	Email         string `gorm:"size:254;not null;uniqueIndex" json:"email"`
	EmailVerified bool   `gorm:"not null;default:false" json:"email_verified"`
	Nickname      string `gorm:"size:64" json:"nickname"`
	// Avatar 头像地址（/uploads/avatars/xxx.png），空着就显示昵称首字母
	Avatar       string    `gorm:"size:500" json:"avatar"`
	PasswordHash string    `gorm:"size:128;not null" json:"-"` // 密码哈希，不返回给前端
	PasswordSalt string    `gorm:"size:64;not null" json:"-"`  // 盐值
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Project 项目
type Project struct {
	ID          int    `gorm:"primaryKey" json:"id"`
	UserID      int    `gorm:"index;not null;uniqueIndex:idx_projects_user_name,priority:1" json:"-"`
	Name        string `gorm:"size:200;not null;uniqueIndex:idx_projects_user_name,priority:2" json:"name"`
	Description string `gorm:"type:text" json:"description"`
	GitRepo     string `gorm:"size:500" json:"git_repo"`
	Customer    string `gorm:"size:200" json:"customer"`                      // 客户
	Status      string `gorm:"size:20;not null;default:active" json:"status"` // active/archived
	// Amount 合同总金额，改它会产生一条金额变更记录
	Amount Amount `gorm:"type:numeric(18,2);not null;default:0" json:"amount"`

	// 以下字段只读，实时汇总而来（不在 projects 表里存）
	TaskCount         int       `gorm:"-" json:"task_count"`
	EstimatedHours    float64   `gorm:"-" json:"estimated_hours"`
	ActualHours       float64   `gorm:"-" json:"actual_hours"`
	Received          Amount    `gorm:"-" json:"received"`                    // 已收款：关联到本项目的收入合计
	ReceivableCount   int       `gorm:"-" json:"receivable_count"`            // 未收笔数
	ReceivableAmount  Amount    `gorm:"-" json:"receivable_amount"`           // 未收金额合计
	ReceivableOverdue Amount    `gorm:"-" json:"receivable_overdue_amount"`   // 其中已逾期的金额
	NextDueDate       string    `gorm:"-" json:"next_due_date,omitempty"`     // 最近一笔到期日
	AmountChanges     int       `gorm:"-" json:"amount_change_count"`         // 金额变更次数
	ChangedAt         string    `gorm:"-" json:"amount_changed_at,omitempty"` // 最近一次金额变更时间
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// ProjectAmountChange 项目金额变更记录
// 金额只记「改成多少」，差额 delta 由服务端算，历史可追溯
type ProjectAmountChange struct {
	ID        int       `gorm:"primaryKey" json:"id"`
	UserID    int       `gorm:"index;not null" json:"-"`
	ProjectID int       `gorm:"index;not null" json:"project_id"`
	OldAmount Amount    `gorm:"type:numeric(18,2);not null;default:0" json:"old_amount"`
	NewAmount Amount    `gorm:"type:numeric(18,2);not null;default:0" json:"new_amount"`
	Delta     Amount    `gorm:"type:numeric(18,2);not null;default:0" json:"delta"`
	Reason    string    `gorm:"size:500" json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}

// Receivable 待收款（收款计划的一笔）
// v1 只记录：金额 + 到期时间 + 备注，备注随时可改；到期只做提醒，不自动对账。
type Receivable struct {
	ID          int        `gorm:"primaryKey" json:"id"`
	UserID      int        `gorm:"index;not null" json:"-"`
	ProjectID   *int       `gorm:"index" json:"project_id,omitempty"`
	ProjectName string     `gorm:"-" json:"project_name,omitempty"` // 只读，列表展示用
	Title       string     `gorm:"size:200;not null" json:"title"`
	Amount      Amount     `gorm:"type:numeric(18,2);not null;default:0" json:"amount"`
	DueDate     *string    `gorm:"size:10" json:"due_date,omitempty"`
	Note        string     `gorm:"type:text" json:"note,omitempty"`
	Status      string     `gorm:"size:20;not null;default:pending" json:"status"` // pending/done
	DoneAt      *time.Time `json:"done_at,omitempty"`
	Overdue     bool       `gorm:"-" json:"overdue"`   // 只读：未收且已过期
	DaysLeft    int        `gorm:"-" json:"days_left"` // 只读：距到期还有几天，负数表示过期
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// ReceivableSummary 待收款汇总：页面顶部三格用
type ReceivableSummary struct {
	PendingCount  int    `json:"pending_count"`
	PendingAmount Amount `json:"pending_amount"`
	OverdueCount  int    `json:"overdue_count"`
	OverdueAmount Amount `json:"overdue_amount"`
	DueSoonCount  int    `json:"due_soon_count"` // 7 天内到期
	DueSoonAmount Amount `json:"due_soon_amount"`
	DoneCount     int    `json:"done_count"`
	DoneAmount    Amount `json:"done_amount"`
}

// Finance 财务
//
// ReceivableID 指向「标记已收」时自动生成这笔流水的收款计划；
// 手记的收支为空，撤回 / 删除收款计划时就只删自动记的那条，不误伤手记的账。
type Finance struct {
	ID           int       `gorm:"primaryKey" json:"id"`
	UserID       int       `gorm:"index;not null" json:"-"`
	Type         string    `gorm:"size:20;not null" json:"type"` // income/expense
	Amount       Amount    `gorm:"type:numeric(18,2);not null;default:0" json:"amount"`
	Category     string    `gorm:"size:100" json:"category"`
	Description  string    `gorm:"type:text" json:"description"`
	Date         string    `gorm:"size:10;not null;index" json:"date"` // YYYY-MM-DD
	ProjectID    *int      `gorm:"index" json:"project_id,omitempty"`
	ReceivableID *int      `gorm:"index" json:"receivable_id,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Idea 灵感。想到什么随手丢一条，正文本身是唯一的必填项。
//
// Status 决定它后面去哪儿：
//   - inbox 待整理：默认状态，卡片默认只列这个
//   - project 已成项目：想法长大成项目，转的时候顺手建一条 project 并互相关联
//   - archived 已归档：不想再看到，但也不删——想法多半过一阵又想起来
type Idea struct {
	ID        int       `gorm:"primaryKey" json:"id"`
	UserID    int       `gorm:"index;not null" json:"-"`
	Content   string    `gorm:"type:text;not null" json:"content"`
	Status    string    `gorm:"size:20;not null;default:inbox;index" json:"status"` // inbox/project/archived
	ProjectID *int      `gorm:"index" json:"project_id,omitempty"`                  // 转成项目后指回去
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// 以下字段只读，查询时补上，不在 ideas 表里存
	ProjectName string `gorm:"-" json:"project_name,omitempty"`
}

// Task 统一任务（日常待办 / 问题 / 需求 / 商机）
// Task 与 Issue 合并为同一模型，由 Category 区分；商机额外使用 OpportunityStatus。
type Task struct {
	ID                int        `gorm:"primaryKey" json:"id"`
	UserID            int        `gorm:"index;not null" json:"-"`
	Number            int        `gorm:"index" json:"number"`
	Title             string     `gorm:"size:500;not null" json:"title"`
	Content           string     `gorm:"type:text" json:"content"`
	Category          string     `gorm:"size:20;not null;default:todo;index" json:"category"` // todo/issue/requirement/opportunity
	Status            string     `gorm:"size:20;not null;default:TODO;index" json:"status"`   // TODO/DOING/BLOCKED/DONE
	OpportunityStatus string     `gorm:"size:20" json:"opportunity_status,omitempty"`
	Priority          string     `gorm:"size:20;not null;default:MEDIUM" json:"priority"` // LOW/MEDIUM/HIGH/URGENT
	ProjectID         *int       `gorm:"index" json:"project_id,omitempty"`
	ProjectName       string     `gorm:"-" json:"project_name,omitempty"` // 只读，列表展示用
	Labels            []Label    `gorm:"many2many:task_labels;" json:"labels"`
	DueDate           *string    `gorm:"size:10" json:"due_date,omitempty"`
	ScheduledDate     *string    `gorm:"size:10;index" json:"scheduled_date,omitempty"`
	EstimatedHours    float64    `gorm:"not null;default:0" json:"estimated_hours"` // 工作量评估（小时），手动填写
	ActualHours       float64    `gorm:"not null;default:0" json:"actual_hours"`    // 实际用时（小时），手动填写
	IsToday           bool       `gorm:"not null;default:false;index" json:"is_today"`
	TodayOrder        *int       `json:"today_order,omitempty"`
	ClosedAt          *time.Time `json:"closed_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// Label 标签
type Label struct {
	ID        int       `gorm:"primaryKey" json:"id"`
	UserID    int       `gorm:"index;not null;uniqueIndex:idx_labels_user_name,priority:1" json:"-"`
	Name      string    `gorm:"size:100;not null;uniqueIndex:idx_labels_user_name,priority:2" json:"name"`
	Color     string    `gorm:"size:20;not null;default:#1890ff" json:"color"`
	TaskCount int       `gorm:"-" json:"task_count,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// Comment 回复/讨论
// Content 保存 Markdown 源文本；ContentHTML 为渲染后的富文本，供前端直接展示。
type Comment struct {
	ID          int       `gorm:"primaryKey" json:"id"`
	UserID      int       `gorm:"index;not null" json:"-"`
	TargetType  string    `gorm:"size:20;not null;default:task;index:idx_comments_target,priority:1" json:"target_type"`
	TargetID    int       `gorm:"not null;index:idx_comments_target,priority:2" json:"target_id"`
	Content     string    `gorm:"type:text;not null" json:"content"`
	ContentHTML string    `gorm:"-" json:"content_html,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// Attachment 附件（评论/任务内容里粘贴的截图，或富文本里挂的文件）
//
// 只属于上传它的人：文件落在 uploads/<user_id>/ 下，读取走 GET /api/attachments/:id，
// 仓储层按 user_id 过滤，别人拿到 id 也是 404。URL 是鉴权接口，不是公开地址。
type Attachment struct {
	ID           int       `gorm:"primaryKey" json:"id"`
	UserID       int       `gorm:"index;not null" json:"-"`
	Filename     string    `gorm:"size:200;not null" json:"filename"` // 落盘名（随机串）
	OriginalName string    `gorm:"size:300" json:"original_name,omitempty"`
	MimeType     string    `gorm:"size:100;not null" json:"mime_type"`
	Size         int64     `gorm:"not null" json:"size"`
	Path         string    `gorm:"size:500;not null" json:"path"` // 相对 uploads 的路径
	URL          string    `gorm:"-" json:"url"`                  // 只读：/api/attachments/:id
	CreatedAt    time.Time `json:"created_at"`
}

// TaskFilter 任务查询条件
type TaskFilter struct {
	Category  string
	Status    string
	Priority  string
	ProjectID *int
	Label     string
	Keyword   string
	TodayOnly bool
	Scheduled string
	// 日期区间（依据 scheduled_date，缺失时回退到 due_date），用于日历视图
	StartDate string
	EndDate   string
}

// TaskStats 任务统计
type TaskStats struct {
	ByCategory map[string]int `json:"by_category"`
	ByStatus   map[string]int `json:"by_status"`
	TodayTotal int            `json:"today_total"`
	TodayDone  int            `json:"today_done"`
	// 今日工时（小时）：评估合计 / 实际合计，均由用户手动填写
	TodayEstimatedHours float64 `json:"today_estimated_hours"`
	TodayActualHours    float64 `json:"today_actual_hours"`
}

// All 参与建表迁移的全部表。
// 顺序即依赖顺序：用户 → 项目 → 依赖项目的表。
func All() []any {
	return []any{
		&User{}, &Project{}, &Task{}, &Label{}, &Comment{},
		&Finance{}, &Receivable{}, &ProjectAmountChange{},
		&Idea{}, &Attachment{},
	}
}
