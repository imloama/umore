package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/imloama/umore/internal/model"
	"github.com/imloama/umore/internal/repository"

	"github.com/gin-gonic/gin"
)

// /tasks/today 给的是用户时区里的今天；/tasks/stats 按同一天算，两边不能打架
func TestTaskTodayEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newHandlerRepo(t)
	uid := newHandlerUser(t, repo, "today@example.com")

	h := &TaskHandler{Handler: Handler{Repo: repo}}
	r := gin.New()
	r.GET("/api/tasks/today", func(c *gin.Context) {
		c.Set(CtxUserID, uid)
		h.Today(c)
	})
	r.GET("/api/tasks/stats", func(c *gin.Context) {
		c.Set(CtxUserID, uid)
		h.Stats(c)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/tasks/today", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("today want 200, got %d: %s", w.Code, w.Body.String())
	}
	var info struct {
		Today    string `json:"today"`
		Timezone string `json:"timezone"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &info); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if info.Today != model.TodayIn(model.DefaultTimezone) {
		t.Fatalf("默认时区的今天应为 %s，实际 %s", model.TodayIn(model.DefaultTimezone), info.Today)
	}
	if info.Timezone != model.DefaultTimezone {
		t.Fatalf("时区应为 %s，实际 %s", model.DefaultTimezone, info.Timezone)
	}

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/tasks/stats", nil))
	var stats model.TaskStats
	if err := json.Unmarshal(w.Body.Bytes(), &stats); err != nil {
		t.Fatalf("decode stats: %v", err)
	}
	if stats.TodayDate != info.Today || stats.Timezone != info.Timezone {
		t.Fatalf("统计口径与 /tasks/today 不一致: %+v vs %+v", stats, info)
	}
}

// 建一个带 AutoMigrate 的库 + 一个账号，够这些 handler 用例用
func newHandlerRepo(t *testing.T) *repository.Repository {
	t.Helper()
	repo, err := repository.NewRepository(t.TempDir() + "/handler.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { repo.Close() })
	if err := repo.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return repo
}

func newHandlerUser(t *testing.T, repo *repository.Repository, email string) int {
	t.Helper()
	user, err := repo.CreateUser(email, "hash", "salt", true)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return user.ID
}
