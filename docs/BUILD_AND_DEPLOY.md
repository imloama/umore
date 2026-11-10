# 构建与部署

目标是「一个二进制跑起来就能用」：前端静态资源打进 `backend/web`，迁移 SQL 打进二进制。
本地默认 SQLite（数据落在 `data/`，整个目录拷走就能搬家），
正式环境连 Postgres + Redis，二进制不变，换配置即可。

## 一键构建

```bash
./build.sh
```

做的事：

1. `frontend`: `npm ci` → `npm run build`，产物拷到 `backend/web`
2. `backend`: `CGO_ENABLED=0 go build -trimpath -o umore ./cmd/server`
3. `backend`: `CGO_ENABLED=0 go build -trimpath -o umore-migrate ./cmd/migrate`

产物：`backend/umore`（约 17MB）、`backend/umore-migrate`、`backend/web/`。

## 配置

```bash
cp backend/config.example.json backend/config.json
```

配置文件分四段：`database`（数据库）、`redis`（缓存）、`auth`（token 有效期）、
`resend`（发信）。字段说明与取舍见 [POSTGRES_REDIS.md](POSTGRES_REDIS.md)。

两条路，按环境选：

- **本地开发**：什么都不用配。默认 SQLite + 进程内缓存，只有注册发码需要在
  `config.json` 里把 `resend.log_only` 设成 `true`（验证码打到日志，不发真邮件）
- **正式环境**：`database.driver` 填 `postgres`、`redis.addr` 填上。这两样都可以用
  环境变量注入（优先于文件）：`UMORE_DB_DRIVER`、`UMORE_DB_DSN`、`UMORE_REDIS_ADDR`

配置了 `redis.addr` 却连不上时**服务直接启动失败**，不会静默退化成单机内存缓存——
多实例部署时那种降级会让验证码和登录态各存各的，比启动失败难查得多。

## 运行

```bash
# 发布前先迁移，能直接看到跑了哪几条
cd backend && ./umore-migrate status && ./umore-migrate up

./start.sh            # 等价于 cd backend && ./umore
# 打开 http://localhost:8080
```

服务启动时会再跑一次迁移兜底（已执行过的不会重跑），所以忘了单独跑也不会漏。

首次打开是注册页：邮箱 + 验证码 + 密码（至少 8 位）。
验证码由 Resend 发出，码通过才建号，账号落库即「邮箱已验证」，
随后返回登录 `token`（默认 7 天）与加密密钥。

## Docker

```bash
docker compose up -d postgres redis    # 只起依赖，本地 go run 连它们
RESEND_API_KEY=re_xxx docker compose up -d   # 连 umore 一起起
```

`docker-compose.yml` 里有 Postgres 16 + Redis 7 + umore，
数据落在命名卷（`pgdata` / `umore-data`），`down` 不删，要清空加 `-v`。
`Dockerfile` 启动命令是 `./umore-migrate up && exec ./umore`。

## 目录与环境变量

| 变量 | 默认 | 说明 |
| --- | --- | --- |
| `PORT` | `8080` | 监听端口 |
| `UMORE_DATA_DIR` | `./data` | SQLite 文件与上传图片 `uploads/` 都在这 |
| `UMORE_WEB_DIR` | `./web` | 前端产物目录，不存在时只跑 API |
| `UMORE_CONFIG` | `./config.json` | 配置文件，不存在时按默认值跑 |
| `UMORE_DB_DRIVER` | `sqlite` | `sqlite` / `postgres` |
| `UMORE_DB_DSN` | 空 | 连接串；SQLite 留空则落 `UMORE_DATA_DIR/umore.db` |
| `UMORE_REDIS_ADDR` | 空 | `host:port`；留空用进程内缓存（仅单机开发） |
| `UMORE_REDIS_PASSWORD` / `UMORE_REDIS_DB` / `UMORE_REDIS_PREFIX` | 空 / `0` / `umore` | Redis 细节与键前缀 |
| `UMORE_TOKEN_TTL_HOURS` | `168` | 登录 token 有效期 |
| `RESEND_API_KEY` | 空 | 优先级高于配置文件里的 `resend.api_key` |
| `RESEND_FROM` | 空 | 优先级高于配置文件里的 `resend.from`，容器里只注环境变量就够 |

配置文件模板见 `backend/config.example.json`。

备份：SQLite 就拷 `data/` 目录；Postgres 用 `pg_dump`；上传的图片始终在 `UMORE_DATA_DIR/uploads`，
所以换机器时 `data/` 和数据库备份要一起带走。

## 为什么后端不再需要 cgo

SQLite 驱动是 `github.com/glebarez/sqlite`（纯 Go，底下是 `modernc.org/sqlite`），
Postgres 走 `pgx`（也是纯 Go）。
好处是 `CGO_ENABLED=0` 也能编译，交叉编译到 Linux/macOS 不用装工具链，
镜像里也不用塞 gcc（运行阶段是 `alpine`）；代价是二进制大了几 MB，
本地单机场景下性能差异可以忽略。

## 前后端分离开发

```bash
# 终端 1
cd backend && go run ./cmd/server

# 终端 2
cd frontend && npm run dev     # http://localhost:5173，/api 代理到 8080
```

