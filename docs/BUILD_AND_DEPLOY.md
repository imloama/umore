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

产物：`backend/umore`（约 27MB）、`backend/umore-migrate`、`backend/web/`。

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
# 打开 http://localhost:8080，探活 http://localhost:8080/healthz
```

`umore` 的相对路径基准是**进程工作目录**：`data/`、`config.json`、`web/`
都按 CWD 找，用 systemd 起的话记得配 `WorkingDirectory`（或用 `UMORE_DATA_DIR`
`UMORE_CONFIG` `UMORE_WEB_DIR` 三个变量给绝对路径）。

服务启动时会再跑一次迁移兜底（已执行过的不会重跑），所以忘了单独跑也不会漏。

首次打开是注册页：邮箱 + 验证码 + 密码（至少 8 位）。
验证码由 Resend 发出，码通过才建号，账号落库即「邮箱已验证」，
随后返回登录 `token`（默认 7 天）与加密密钥。

## 前端如何托管的（单进程同源）

启一个 `./umore` 就同时提供页面和接口，浏览器里只有一个源，没有 CORS 也没有代理：

| 路径 | 谁处理 | 行为 |
| --- | --- | --- |
| `/api/*` | Gin 路由 | JSON；未登录 401，未命中 404 JSON |
| `/uploads/*` | `r.Static` | 上传的图片（只允许 png/jpeg/gif/webp，按文件头判定） |
| `/assets/*` | `r.Static` | 构建产物，带 hash，响应 `Cache-Control: immutable` |
| `/favicon.svg`、`robots.txt` 等 | `NoRoute` 里的落盘检查 | 产物目录里真实存在就直接发 |
| 其余 GET 路径 | `NoRoute` | 回 `index.html`，`Cache-Control: no-cache`，前端路由自己接管 |
| 缺失的 `*.js` / `*.png` | `NoRoute` | **404**（不会拿 index.html 冒充，否则浏览器报 MIME 错误） |

前端那边不需要任何环境变量：`src/services/api.ts` 用的是相对路径 `/api`，
头像和附件也是后端返回的 `/uploads/...` 相对地址。默认部署在**域名根**；
要挂子路径（`www.a.com/umore/`）得同时改 `vite.config.ts` 的 `base`、
`BrowserRouter` 的 `basename`、axios 的 `baseURL` 和 401 跳转路径。

`GET /healthz` 返回 `200 ok`，只表示进程活着（不查数据库和缓存），给探针用。

## 反向代理（nginx）

```nginx
server {
    listen 443 ssl http2;
    server_name www.a.com;
    client_max_body_size 12m;          # 上传上限 10MB，留点余量

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host              $host;
        proxy_set_header X-Real-IP         $remote_addr;
        proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

要点：

- **别写 `try_files ... /index.html`**，SPA 兜底在后端做；nginx 一旦自己兜底，
  静态资源真的 404 就会被永久掩盖，发版后白屏只能靠猜。
- 同机部署不用配 `UMORE_TRUSTED_PROXIES`（默认只信任 `127.0.0.1` / `::1`），
  nginx 和 umore 不同机时填 nginx 的地址，否则日志里记的是代理 IP。
- 想让 nginx 做 gzip / brotli 压缩，注意给 `text/javascript` 开，前端主包 437KB → gzip 后约 137KB。

## Docker（仓库里未包含，需要自己加）

`Dockerfile` 和 `docker-compose.yml` 目前没有随仓库提交，要用就照下面这份自己建：

```dockerfile
FROM node:20-alpine AS web
WORKDIR /app
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.23-alpine AS api
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -o umore ./cmd/server \
 && CGO_ENABLED=0 go build -trimpath -o umore-migrate ./cmd/migrate

FROM alpine:3.20
WORKDIR /app
RUN adduser -D -u 10001 umore
COPY --from=api /src/umore /src/umore-migrate ./
COPY --from=web /app/dist ./web
RUN mkdir -p data && chown -R umore data
USER umore
EXPOSE 8080
CMD ["sh", "-c", "./umore-migrate up && exec ./umore"]
```

数据用卷挂到 `/app/data`（SQLite 与 `uploads/` 都在里面），配置用 `UMORE_*` 环境变量注入。

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
| `UMORE_TRUSTED_PROXIES` | `127.0.0.1,::1` | 只信任这些地址发来的 `X-Forwarded-For`；多级代理用逗号分隔 |
| `RESEND_API_KEY` | 空 | 优先级高于配置文件里的 `resend.api_key` |
| `RESEND_FROM` | 空 | 优先级高于配置文件里的 `resend.from`，容器里只注环境变量就够 |

配置文件模板见 `backend/config.example.json`。

备份：SQLite 就拷 `data/` 目录；Postgres 用 `pg_dump`；上传的图片始终在 `UMORE_DATA_DIR/uploads`，
所以换机器时 `data/` 和数据库备份要一起带走。

## 发布与回滚

```bash
./build.sh                      # 前端 + 后端一起出产物
cd backend && ./umore-migrate status && ./umore-migrate up
systemctl restart umore         # 或 ./start.sh
curl -fsS http://127.0.0.1:8080/healthz
```

进程收到 `SIGTERM` / `SIGINT` 会先停止收新请求，给手上的请求最多 15 秒收尾再退出，
所以发版可以直接 restart，不必担心把正在保存的请求打断。
旧二进制和旧 `backend/web` 留一份，回滚就是换回目录再 restart（迁移不保证可逆）。

systemd 可以直接用这份（`WorkingDirectory` 别省，相对路径全靠它）：

```ini
[Unit]
Description=umore
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=umore
WorkingDirectory=/opt/umore/backend
Environment=PORT=8080
Environment=UMORE_WEB_DIR=/opt/umore/backend/web
Environment=UMORE_DATA_DIR=/opt/umore/data
Environment=UMORE_CONFIG=/opt/umore/backend/config.json
ExecStart=/opt/umore/backend/umore
Restart=on-failure
RestartSec=3

[Install]
WantedBy=multi-user.target
```

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

开发期 CORS 白名单只放本机任意端口（`localhost:` / `127.0.0.1:`），
正式环境前后端同源用不上 CORS，所以这个白名单不用放开。

