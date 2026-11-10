package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/imloama/umore/internal/auth"
	"github.com/imloama/umore/internal/cache"
	"github.com/imloama/umore/internal/config"
	"github.com/imloama/umore/internal/handler"
	"github.com/imloama/umore/internal/mailer"
	"github.com/imloama/umore/internal/repository"
	"github.com/imloama/umore/internal/verification"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	// 数据目录可由环境变量覆盖，方便部署到任意位置
	dataDir := envOr("UMORE_DATA_DIR", filepath.Join(".", "data"))
	// Resend 等发信参数放配置文件，key 不写进代码
	cfgPath := envOr("UMORE_CONFIG", filepath.Join(".", "config.json"))
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("Failed to load config %s: %v", cfgPath, err)
	}
	if !cfg.Resend.Configured() {
		log.Printf("WARN: resend 未配置（%s），注册时无法发送验证码", cfgPath)
	} else if cfg.Resend.LogOnly {
		log.Printf("WARN: resend.log_only 已开启，验证码只打到日志，不发真邮件")
	}
	uploadDir := filepath.Join(dataDir, "uploads")
	webDir := envOr("UMORE_WEB_DIR", filepath.Join(".", "web"))

	repo, err := openDatabase(cfg, dataDir)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer repo.Close()
	log.Printf("database: %s", describeDatabase(cfg, dataDir))

	// 建表 / 补字段：结构体即 schema，两种库各自翻译，不用维护两套 DDL
	if err := repo.Migrate(); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}
	if err := handler.EnsureUploadDir(uploadDir); err != nil {
		log.Fatalf("Failed to create upload dir: %v", err)
	}

	// 缓存：正式环境 Redis，本地没配就用进程内存
	store, err := cache.Open(context.Background(), cache.Options{
		Addr:        cfg.Redis.AddrValue(),
		Password:    cfg.Redis.PasswordValue(),
		DB:          cfg.Redis.DBValue(),
		Prefix:      cfg.Redis.PrefixValue(),
		DialTimeout: time.Duration(cfg.Redis.DialTimeoutSeconds) * time.Second,
	})
	if err != nil {
		log.Fatalf("Failed to init cache: %v", err)
	}
	defer store.Close()

	// 登录态：token 存缓存，退出登录即删键
	sessions := auth.NewManager(store, cfg.Auth.TokenTTL())

	// 发信与验证码（验证码同样存缓存，不落库）
	mailerSvc := mailer.New(cfg.Resend)
	verifySvc := verification.New(repo, store, cfg.Resend, mailerSvc)

	// 初始化Gin
	r := gin.Default()

	// CORS配置
	r.Use(cors.New(cors.Config{
		// 开发期允许本机任意端口（127.0.0.1 / [::1]），避免换端口后被预检拦掉
		AllowOriginFunc: func(origin string) bool {
			return strings.HasPrefix(origin, "http://localhost:") ||
				strings.HasPrefix(origin, "http://127.0.0.1:")
		},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	// 初始化handlers
	authHandler := &handler.AuthHandler{Handler: handler.Handler{Repo: repo}, Verify: verifySvc, Sessions: sessions, UploadDir: uploadDir}
	projectHandler := &handler.ProjectHandler{Handler: handler.Handler{Repo: repo}}
	financeHandler := &handler.FinanceHandler{Handler: handler.Handler{Repo: repo}}
	ideaHandler := &handler.IdeaHandler{Handler: handler.Handler{Repo: repo}}
	taskHandler := &handler.TaskHandler{Handler: handler.Handler{Repo: repo}}
	labelHandler := &handler.LabelHandler{Handler: handler.Handler{Repo: repo}}
	receivableHandler := &handler.ReceivableHandler{Handler: handler.Handler{Repo: repo}}
	attachmentHandler := &handler.AttachmentHandler{Handler: handler.Handler{Repo: repo}, UploadDir: uploadDir}

	// API路由
	api := r.Group("/api")
	{
		// 认证：注册登录相关不要求 token，其余业务接口全部要求
		authGroup := api.Group("/auth")
		{
			authGroup.GET("/check-setup", authHandler.CheckSetup)
			authGroup.POST("/send-code", authHandler.SendCode)
			authGroup.POST("/register", authHandler.Register)
			authGroup.POST("/login", authHandler.Login)
			authGroup.POST("/logout", authHandler.Logout)
			authGroup.GET("/me", handler.RequireAuth(sessions), authHandler.Me)
			authGroup.PUT("/profile", handler.RequireAuth(sessions), authHandler.UpdateProfile)
			authGroup.POST("/password", handler.RequireAuth(sessions), authHandler.ChangePassword)
			authGroup.POST("/avatar", handler.RequireAuth(sessions), authHandler.UploadAvatar)
		}

		// 从这里往下都要登录：token 由 /auth/login 或 /auth/register 签发
		api.Use(handler.RequireAuth(sessions))

		// 项目
		projects := api.Group("/projects")
		{
			projects.GET("", projectHandler.GetAll)
			projects.DELETE("/amount-changes/:id", projectHandler.DeleteAmountChange)
			projects.GET("/:id", projectHandler.GetOne)
			projects.POST("", projectHandler.Create)
			projects.PUT("/:id", projectHandler.Update)
			projects.DELETE("/:id", projectHandler.Delete)
			projects.GET("/:id/amount-changes", projectHandler.GetAmountChanges)
			projects.POST("/:id/amount-changes", projectHandler.AddAmountChange)
			projects.GET("/:id/comments", projectHandler.GetComments)
			projects.POST("/:id/comments", projectHandler.AddComment)
		}

		// 财务
		finances := api.Group("/finances")
		{
			finances.GET("", financeHandler.GetByRange)
			finances.GET("/summary", financeHandler.GetSummary)
			finances.POST("", financeHandler.Create)
			finances.PUT("/:id", financeHandler.Update)
			finances.DELETE("/:id", financeHandler.Delete)
		}

		// 待收款（收款计划）
		receivables := api.Group("/receivables")
		{
			receivables.GET("", receivableHandler.GetAll)
			receivables.GET("/summary", receivableHandler.GetSummary)
			receivables.POST("", receivableHandler.Create)
			receivables.PUT("/:id", receivableHandler.Update)
			receivables.POST("/:id/status", receivableHandler.SetStatus)
			receivables.DELETE("/:id", receivableHandler.Delete)
		}

		// 任务（统一 todo / issue / requirement / opportunity）
		tasks := api.Group("/tasks")
		{
			tasks.GET("", taskHandler.GetAll)
			tasks.GET("/stats", taskHandler.Stats)
			tasks.PUT("/reorder", taskHandler.ReorderToday)
			tasks.DELETE("/comments/:id", taskHandler.DeleteComment)
			tasks.GET("/:id", taskHandler.GetOne)
			tasks.POST("", taskHandler.Create)
			tasks.PUT("/:id", taskHandler.Update)
			tasks.DELETE("/:id", taskHandler.Delete)
			tasks.POST("/:id/today", taskHandler.SetToday)
			tasks.GET("/:id/comments", taskHandler.GetComments)
			tasks.POST("/:id/comments", taskHandler.AddComment)
		}

		// 标签字典
		labels := api.Group("/labels")
		{
			labels.GET("", labelHandler.GetAll)
			labels.POST("", labelHandler.Create)
			labels.PUT("/:id", labelHandler.Update)
			labels.DELETE("/:id", labelHandler.Delete)
		}

		// 附件（评论/内容里粘贴的截图）
		attachments := api.Group("/attachments")
		{
			attachments.POST("", attachmentHandler.Upload)
			attachments.POST("/upload", attachmentHandler.Upload)
		}

		// 灵感
		ideas := api.Group("/ideas")
		{
			ideas.GET("", ideaHandler.GetAll)
			ideas.POST("", ideaHandler.Create)
			ideas.PUT("/:id", ideaHandler.Update)
			ideas.POST("/:id/project", ideaHandler.Promote)
			ideas.DELETE("/:id", ideaHandler.Delete)
		}
	}

	// 上传的图片走静态托管
	r.Static("/uploads", uploadDir)

	// 前端构建产物存在时一并托管，单进程即可部署（前后端同源，不用再配 CORS/代理）
	if indexPath := filepath.Join(webDir, "index.html"); fileExists(indexPath) {
		r.Static("/assets", filepath.Join(webDir, "assets"))
		r.StaticFile("/vite.svg", filepath.Join(webDir, "vite.svg"))
		serveIndex := func(c *gin.Context) {
			// 静态资源和 API 已经匹配过，剩下的路径一律交给前端路由
			c.File(indexPath)
		}
		r.NoRoute(func(c *gin.Context) {
			if strings.HasPrefix(c.Request.URL.Path, "/api") {
				c.JSON(404, gin.H{"error": "not found"})
				return
			}
			serveIndex(c)
		})
		log.Printf("Serving frontend from %s", webDir)
	} else {
		log.Printf("Frontend bundle not found at %s, API only", indexPath)
	}

	addr := ":" + envOr("PORT", "8080")
	log.Printf("Server starting on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// openDatabase 按配置打开数据库；SQLite 没给路径时落到 dataDir 下的 umore.db
func openDatabase(cfg config.Config, dataDir string) (*repository.Repository, error) {
	driver := cfg.Database.DriverValue()
	dsn := cfg.Database.DSNValue()
	if driver == config.DriverSQLite && strings.TrimSpace(dsn) == "" {
		dsn = filepath.Join(dataDir, "umore.db")
	}
	return repository.Open(repository.Options{
		Driver:          driver,
		DSN:             dsn,
		MaxOpenConns:    cfg.Database.MaxOpenConns,
		MaxIdleConns:    cfg.Database.MaxIdleConns,
		ConnMaxLifetime: time.Duration(cfg.Database.ConnMaxLifetimeMinutes) * time.Minute,
	})
}

// describeDatabase 启动日志里说清楚连的是哪儿，别让人猜
func describeDatabase(cfg config.Config, dataDir string) string {
	driver := cfg.Database.DriverValue()
	dsn := cfg.Database.DSNValue()
	if driver == config.DriverSQLite {
		if dsn == "" {
			dsn = filepath.Join(dataDir, "umore.db")
		}
		return "sqlite " + dsn
	}
	// 连接串里可能带密码，日志里只留个大概位置
	if at := strings.LastIndex(dsn, "@"); at >= 0 {
		return driver + " " + dsn[at+1:]
	}
	return driver
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
