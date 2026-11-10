# umore v4 · 接单业务工作台 PRD

版本：v4.0
定位：**个人独立开发者的接单业务工作台**（不是通用项目管理）
基线：前端基于 satnaing/shadcn-admin v2.2.1 改造
状态：待确认

---

## 0. 为什么要重梳

现有 PRD v3.0 试图覆盖"今日任务 / 看板 / 日历 / 服务器 / 环境 / 账户 / 灵感 / 财务"共 10 个模块，是一个**通用个人效率工具**。
但独立开发者的真实痛点是另一条链：

> 有人来问 → 我要不要接 → 多少钱 → 什么时候交 → 交付了吗 → 钱到了吗

信息断在微信聊天、备忘录、Excel 里，导致三件事反复发生：
1. 报价时想不起上次类似项目多少钱
2. 客户催了才发现还有尾款没要
3. 手上堆了 5 个活，不知道哪个该先做

所以 v4 的重心：**一条从商机到回款的业务主线**，其余模块围绕这条线服务。

---

## 1. 产品边界

### 要做
- 一个人用，单机/自建，数据在自己手里
- 管"接单"这件事的全流程：商机 → 报价 → 合同 → 交付 → 收付款
- 交付期间的需求/问题/工时记录
- 轻量客户档案（谁欠我钱、历史合作）

### 不做
- 多人协作、权限体系、审批流（一个人用不需要）
- 通用任务看板（把任务挂在"单"上就够，不做独立任务产品）
- 完整会计（不做凭证、科目、报表合规，只做收付款流水 + 汇总）
- CRM 营销自动化（不做线索池、漏斗分析、群发）
- 服务器/SSH 运维平台（v3 的 Server 模块降级为项目里的"凭证备忘"）

### 关键约束
- 单用户：认证用邮箱 + 密码注册/登录（不做邮箱验证、不做找回），数据只在本机
- 离线优先：断网也能看今天的活
- 数据可迁移：SQLite 单文件 + JSON 导出，随时能带走

---

## 2. 业务主线（核心）

```
商机 Lead ──qualify──▶ 报价 Quote ──accept──▶ 项目 Project ──▶ 交付 Deliver ──▶ 回款 Payment
   │                       │                     │                  │              │
   └─ 放弃(丢单)           └─ 拒绝               └─ 需求/问题        └─ 工时        └─ 应收账款
                                                  └─ 里程碑                          └─ 对账
```

### 状态机（一句话版）

**商机（Lead）**：新咨询 → 跟进中 → 已报价 → 赢单(转项目) / 丢单

**项目（Project）**：洽谈中 → 进行中 → 待验收 → 已交付 → 已结项 / 中止
> 赢单后自动建项目，商机数据带过去（客户、需求描述、来源）。

**问题（Issue）**：待处理 → 处理中 → 待客户确认 → 已解决 / 挂起
> 只属于项目，不独立存在。分三类：需求变更 / Bug / 咨询。变更类要能标"是否额外收费"。

**收付款（Payment Plan 分期 + Payment 实收）**：
- 计划分期：定金 30% / 中期 40% / 尾款 30%（自由编辑）
- 实收记录：日期、金额、方式、备注
- 自动算：应收 / 已收 / 未收 / 逾期天数

### 一条龙用户体验（关键交互）

1. 有人微信来问 → 打开"快速记录"（全局 `N`），一句话需求 + 客户昵称 + 来源 + 日期，10 秒存下
2. 聊完后 → 在商机详情填报价（报价明细：人天 × 单价）
3. 客户答应 → 点"赢单"，填合同金额 + 分期，自动建项目
4. 项目里陆续记需求/问题，记工时
5. 到节点 → 记一笔收款，未收金额和逾期自动挂到首页
6. 交付完 → 结项，沉淀成"历史案例"，下次报价时可参考

---

## 3. 功能模块与优先级

| 模块 | 优先级 | 说明 |
|---|---|---|
| 工作台 Dashboard | P0 | 今天要做什么、谁欠我钱、这月收了多钱 |
| 商机 Lead | P0 | 一句话快速记录，转化率是唯一指标 |
| 项目 Project | P0 | 业务主实体，聚合需求/问题/工时/收款 |
| 收付款 Finance | P0 | 分期计划 + 实收 + 应收台账 |
| 问题/需求 Issue | P1 | 项目下三类，变更标记额外收费 |
| 客户 Customer | P1 | 人的维度看合作历史与欠款 |
| 工时 TimeEntry | P1 | 记工时 → 反哺报价参考和项目成本 |
| 凭证备忘 Vault | P1 | 平台账号/密钥/环境配置，本地加密 |
| 灵感/知识 | P2 | 链接与片段收集，弱化 |
| 桌面助手/悬浮窗 | P2 | 延后，先跑通 Web |

### 3.1 工作台（首页）

优先级从高到低的信息块：
1. **今日聚焦**：今天要交付的、今天要跟进的（自动从项目/商机算）
2. **资金盘**：本月已收 / 应收未收 / 逾期未收（红色标出）
3. **进行中项目**：状态、剩余天数、未收金额
4. **需要跟进**：商机停留 > 3 天未动、项目超过里程碑未反馈
5. **最近动态**：最近 10 条业务事件（新建商机/收款/问题关闭）

### 3.2 商机

字段：一句话需求*、客户、来源（微信直聊/小红书/抖音/老客转介绍/其他）、预算区间、期望交期、状态、下次跟进日、原始聊天记录/链接
操作：转项目、丢单（必填原因，用于回看"我为什么丢单"）

### 3.3 项目

字段：名称*、客户*、合同金额、计价方式（固定总价/按人天/按月）、开始-结束日、状态、当前里程碑、Git 仓库、备注
子视图：概览 / 需求问题 / 工时 / 收付款 / 附件凭证 / 凭证备忘

### 3.4 收付款

- 项目下编辑分期计划（金额或比例）
- 记一笔实收：自动推导对应分期、是否结清
- 台账视图：按客户/项目/时间筛选，逾期高亮
- 汇总：本月、本年、应收账款总额

### 3.5 问题/需求

字段：标题*、类型（需求变更/Bug/咨询）、优先级、状态、是否额外收费+金额、涉及工时
不做独立 Issue 产品，只在项目上下文里用。

### 3.6 客户

字段：称呼*、微信号/备注名、来源、合作次数、历史合同额、未收金额、备注
详情：该客户所有商机/项目/收款时间线

### 3.7 工时

字段：项目*、日期*、小时数*、说明、是否收费（区分"白干"的成本）
用途：项目成本核算、下次同类需求报价参考

### 3.8 凭证备忘

平台账号：平台、账号、密码（AES-256-GCM 本地加密）、URL
环境配置：项目 × 环境（dev/staging/prod）的地址、备注

---

## 4. 数据模型（v4）

> 只列关键字段，SQLite 单库

```
customer   id, name, wechat_alias, source, note, created_at
lead       id, customer_id?, title(一句话需求), source, budget_min, budget_max,
           expect_date, status, lost_reason, next_follow_at, raw_note, created_at
project    id, lead_id?, customer_id, name, pricing_mode, contract_amount,
           start_date, end_date, status, milestone, git_repo, note, created_at
quote_item id, lead_id|project_id, name, qty(人天), unit_price, amount   -- 报价明细
issue      id, project_id, title, type, status, priority,
           extra_charge(bool), extra_amount, estimate_hours, created_at
time_entry id, project_id, issue_id?, date, hours, note, billable(bool)
payment_plan id, project_id, name(定金/中期/尾款), due_date, amount, ratio
payment    id, project_id, plan_id?, amount, paid_at, method, note
vault_item id, kind(account|env), platform, project_id?, username_ct,
           secret_ct, url, note, key_version, iv, salt
knowledge  id, title, type(link|text), content, tags
```

设计取舍：
- **金额一律整数分**，不用 float，避免 0.1+0.2 的坑
- **报价明细独立成表**，报价可复用到项目合同
- **payment_plan 与 payment 分离**：计划是"应该收"，实收是"真收了"，差额=应收
- 敏感字段命名带 `_ct`（ciphertext），后端只见密文

---

## 5. 信息架构（前端导航）

```
工作台            /                        Dashboard
业务
  ├ 商机          /leads                    列表 + 抽屉详情
  ├ 项目          /projects                 列表
  │   └ 详情      /projects/$id             概览/需求/工时/收款/附件/凭证
  ├ 客户          /customers
  └ 收付款        /payments                 台账 + 汇总
记录
  ├ 工时          /timesheets
  └ 灵感          /knowledge
工具
  ├ 凭证备忘      /vault
  └ 设置          /settings
```

---

## 6. 基于 shadcn-admin 的改造方案

### 6.1 直接复用（省最多时间）

| 能力 | 位置 | 用途 |
|---|---|---|
| Sidebar 布局 / 折叠 / 移动端 | `components/layout/*` | 导航外壳 |
| 主题/暗色/字体/Direction | `context/*` | 设置页 |
| DataTable 全家桶（排序/筛选/分页/批量/列显示） | `components/data-table/*` | 商机/项目/收款/客户列表 |
| 全局搜索 ⌘K | `components/command-menu` | 搜项目/客户/商机 |
| 表单 + zod + RHF 的 Drawer 模式 | `features/tasks/*` | 复制成 lead/project 的增改抽屉 |
| TanStack Router 文件路由 + code splitting | `routes/*` | 保持 |
| TanStack Query 封装与错误处理 | `main.tsx` | 保持，接真实 API |
| 错误页 / 认证页 / 设置页 | `features/*` | 裁剪即用 |
| ECharts→recharts 图表卡片 | `features/dashboard/*` | 首页统计 |

### 6.2 要拆掉的
- **Clerk 全部**：`routes/clerk/*`、`@clerk/react`、`features/apps`（Clerk 市场页）、sidebar 里的 Clerk 分组
- **users / chats / help-center**：与业务无关
- **apps 模块**：无多应用场景

### 6.3 要新增的
- `features/leads` `features/projects` `features/customers` `features/payments` `features/timesheets` `features/vault`
- 每个 feature 遵循基线约定：`components/` + `data/`（schema、query hooks）+ `index.tsx`
- `-` 前缀路由文件做分组（TanStack Router 约定）

### 6.4 技术栈对接（有几处要动手）

| 项 | 基线状态 | 改造动作 |
|---|---|---|
| React 19 + Tailwind 4 | ✅ | 直接吃红利 |
| TanStack Router 文件路由 | ✅ | 加业务路由 |
| TanStack Query | ✅ | 接后端 API，替换 mock |
| **认证** | Clerk | 换成自有 `sign-in` 路由 + 本地主密码 JWT，`_authenticated` 守卫改为读本地 token |
| **Store** | zustand 已有 auth-store | 沿用，改字段 |
| **后端 API** | 无（mock） | Go + Gin 已存在，按 v4 模型扩表与 handler |
| **图表** | recharts | 沿用 |
| i18n | 无 | 先中文硬编码，不做多语言 |

### 6.5 落地顺序

**M0 骨架（1-2 天）**
拉基线 → 拆 Clerk/无用模块 → sidebar 改成 v4 导航 → 空页面路由通 → 主题色定调

**M1 业务主线（核心）**
Customer / Lead / Project 三张表 CRUD + 列表 + 抽屉
Lead → Project 转化
Project 详情页多 tab

**M2 钱**
分期计划 + 实收 + 台账 + 首页资金盘

**M3 交付配套**
Issue（含变更收费）+ 工时

**M4 收尾**
Vault 加密、灵感、导出备份

---

## 7. 待确认（需要你拍板）

1. **商机和项目的边界**：赢单后是"新建项目"还是"商机原地升级"？（我倾向新建，保留商机作为销售记录）
2. **是否需要报价单导出 PDF**：接单场景很实用，但会引入 pdf 库
3. **工时要多细**：只到项目级，还是到 issue 级？（到 issue 能算清"变更该收多少钱"）
4. **凭证备忘是否保留**：v3 花了大力气做零知识加密，如果实际你用的是 1Password/浏览器，这部分可以先砍
5. **是否需要发票/开票状态记录**：个人接单常遇到"要开票加 6 个点"，要不要单独一个状态

---
