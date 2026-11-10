# umore - 个人管理工具

个人日常办公业务管理系统，包括日志、项目、财务、账户等统一管理。

## 功能模块

### 第一期 MVP
1. **任务管理（核心）** - 日常待办 / 问题 / 需求 / 商机 统一为 Task，按工作模式推进
   - 今日视图：手动加入今日 + 完成率统计
   - 列表视图：按类型/状态/优先级/项目/标签/关键词筛选
   - 商机看板：线索 → 跟进中 → 已报价 → 赢单/输单，赢单后可一键转项目
   - 标签：全局字典，任务可挂多个标签
   - 工时：任务上手动填「工作量评估 / 实际用时」（小时），今日视图带偏差提示，项目页按单汇总累计投入
   - 评论富文本：受限 Markdown，工具栏 + 预览，支持直接粘贴/拖入截图
   - 日历视图：月/周切换，拖拽改排期
2. **项目管理** - 项目信息、Git仓库、客户、环境配置、登录凭证(手动创建)
   - 金额：合同总金额 + 金额变更记录（谁加的、加了多少、为什么），收款汇总由财务收入流水实时汇总
   - 项目记录：像任务一样写评论，变更说明 / 金额变更 / Issue 都记在这张单上
3. **回款管理** - 待收款记一笔：金额 + 到期时间 + 备注，到期标红提醒；未收/逾期/已收/按项目分 tab
4. **财务管理** - 收入支出记录
5. **账户管理** - 在线系统账号密码(AES加密)
6. **灵感管理** - 灵感收集


### 第二期
- agent
- 桌面助手(史莱姆形象)
- 全局悬浮窗
- 智能提醒(SSL证书到期等)

## 技术栈

- **后端**: Go + Gin
- **前端**: React + Vite + TypeScript + Tailwind 4（shadcn 风格组件，见 [docs/UI_REFRESH.md](docs/UI_REFRESH.md)）
- **数据层**: gorm，一份模型定义同时建 SQLite（纯 Go 驱动，无需 cgo）与 PostgreSQL 的表；金额用 `numeric(18,2)`
- **多用户**: 所有业务表带 `user_id`，仓储层每个方法都按登录用户过滤，数据互相看不见
- **缓存**: 正式环境 Redis，存验证码与登录 token；本地不配就退化成进程内存（不支持集群）
- **认证**: 邮箱 + 密码注册/登录，注册前用 Resend 发邮箱验证码，登录后签发 token
- **加密**: 登录密码派生密钥 + AES-256-GCM（密钥只在客户端，后端不留）

## 项目结构

```
umore/
├── backend/          # Go后端
│   ├── cmd/         # 入口
│   ├── internal/    # 核心逻辑（model 里是表定义，建表走 AutoMigrate）
│   └── config.example.json  # Resend 参数模板（真实配置不提交）
├── frontend/        # React前端
│   ├── src/
│   └── public/
└── docs/           # 文档
```

## 快速开始

### 一键构建 + 运行（推荐）

```bash
cd backend
go mod tidy
go run ./cmd/server
```

细节见 [docs/BUILD_AND_DEPLOY.md](docs/BUILD_AND_DEPLOY.md)。

注册要发邮箱验证码，先把 Resend 参数填进 `backend/config.json`（模板 `backend/config.example.json`）：

```bash
cp backend/config.example.json backend/config.json
# 填 api_key 与 from，或直接设 RESEND_API_KEY 环境变量
```


本地开发默认 SQLite + 进程内缓存，不用装数据库。


### 开发模式

```bash
# 后端
cd backend && go run ./cmd/server

# 前端（另开终端，/api 代理到 8080）
cd frontend && npm install && npm run dev
```

## License

MIT
