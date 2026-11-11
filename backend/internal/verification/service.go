// Package verification 管注册验证码：生成、发信、校验、作废。
// 验证码只活几分钟，属于典型的临时数据，全部放缓存（正式环境 Redis），不落数据库：
// 一个邮箱同一用途只保留一条，过期由 TTL 自动收走，不用清表。
// 与 HTTP 层解耦，handler 只负责把这里返回的错误翻成提示。
package verification

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/imloama/umore/internal/cache"
	"github.com/imloama/umore/internal/config"
	"github.com/imloama/umore/internal/repository"
)

// 错误文本只用英文：提示语由前端按 handler 回的错误码渲染，后端不做多语。
var (
	// ErrNotConfigured 这台机器没配发信参数
	ErrNotConfigured = errors.New("mail service not configured")
	// ErrTooFrequent 距上次发码太近
	ErrTooFrequent = errors.New("verification code requested too frequently")
	// ErrNoCode 没发过码，或那条码已经被用掉
	ErrNoCode = errors.New("verification code not found, request a new one")
	// ErrExpired 码过期了
	ErrExpired = errors.New("verification code expired")
	// ErrTooManyAttempts 试错次数超上限，这条码作废
	ErrTooManyAttempts = errors.New("too many attempts, request a new code")
	// ErrMismatch 码不对
	ErrMismatch = errors.New("verification code is incorrect")
)

// PurposeRegister 目前只有注册用验证码，找回密码之类以后加在这里
const PurposeRegister = "register"

// CodeSender 真正把验证码送出去的东西。抽成接口是为了测试里能拿到码，
// 生产实现是 internal/mailer（走 Resend）。
type CodeSender interface {
	SendCode(ctx context.Context, to, code string) error
}

// Code 缓存里那份验证码记录。明文不进缓存，只留哈希。
type Code struct {
	CodeHash  string `json:"code_hash"`
	Attempts  int    `json:"attempts"`
	CreatedAt int64  `json:"created_at"` // Unix 秒
}

// Service 发码与校验
type Service struct {
	// Cache 存验证码的地方（Redis / 内存）
	Cache cache.Store
	// Repo 只用来判断邮箱是否已注册
	Repo   *repository.Repository
	Sender CodeSender // nil 表示没配发信，一切发码请求按未配置处理

	Digits      int
	TTL         time.Duration
	Interval    time.Duration
	MaxAttempts int
	// LogOnly 只打日志不发信，前端据此提示「去服务端日志里看码」
	LogOnly bool

	// Now 可替换时钟，测试里用来模拟过期
	Now func() time.Time
}

// New 从配置与发信实现组装服务
func New(repo *repository.Repository, store cache.Store, cfg config.Resend, sender CodeSender) *Service {
	if !cfg.Configured() {
		sender = nil
	}
	return &Service{
		Cache:       store,
		Repo:        repo,
		Sender:      sender,
		Digits:      cfg.CodeLength,
		TTL:         cfg.CodeTTL(),
		Interval:    cfg.ResendInterval(),
		MaxAttempts: cfg.MaxAttempts,
		LogOnly:     cfg.LogOnly,
		Now:         time.Now,
	}
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// Available 没配发信就别让前端干等
func (s *Service) Available() bool { return s.Sender != nil }

func (s *Service) DeliveryMode() string {
	if s.LogOnly {
		return "log"
	}
	return "email"
}

// SendRegisterCode 给邮箱发注册验证码。
// 先写缓存再发信，发信失败就把这条删掉，免得出现「缓存里有个码、用户手上没有」的悬空记录。
func (s *Service) SendRegisterCode(ctx context.Context, email string) error {
	if !s.Available() {
		return ErrNotConfigured
	}
	email = normalizeEmail(email)
	now := s.now()

	// 已有账号就不用发了
	if existing, err := s.Repo.GetUserByEmail(email); err != nil {
		return err
	} else if existing != nil {
		return repository.ErrEmailTaken
	}

	// 冷却：键的 TTL 用来自动清理，判断间隔则用 CreatedAt 比，
	// 这样「重发间隔」与「验证码有效期」走同一个时钟，测试也好控。
	coolKey := cooldownKey(email, PurposeRegister)
	if payload, ok, err := s.Cache.Get(ctx, coolKey); err != nil {
		return err
	} else if ok {
		if last, err := strconv.ParseInt(string(payload), 10, 64); err == nil &&
			now.Sub(time.Unix(last, 0)) < s.Interval {
			return ErrTooFrequent
		}
	}

	code, err := generateCode(s.Digits)
	if err != nil {
		return err
	}
	item := Code{CodeHash: hashCode(PurposeRegister, email, code), CreatedAt: now.Unix()}
	payload, err := json.Marshal(item)
	if err != nil {
		return err
	}
	// 一个邮箱同一用途只留一条，新码直接覆盖旧码
	codeKey := codeKey(email, PurposeRegister)
	if err := s.Cache.Set(ctx, codeKey, payload, s.TTL); err != nil {
		return err
	}
	coolTTL := s.Interval
	if coolTTL <= 0 {
		coolTTL = time.Minute
	}
	if err := s.Cache.Set(ctx, coolKey, []byte(strconv.FormatInt(now.Unix(), 10)), coolTTL); err != nil {
		return err
	}

	if err := s.Sender.SendCode(ctx, email, code); err != nil {
		// 信没发出去，把刚写的码删掉，让用户能立刻重发
		_ = s.Cache.Del(ctx, codeKey)
		_ = s.Cache.Del(ctx, coolKey)
		return fmt.Errorf("failed to send verification email: %w", err)
	}
	return nil
}

// VerifyRegisterCode 校验注册验证码，对了就当场删掉（一次性）
func (s *Service) VerifyRegisterCode(ctx context.Context, email, code string) error {
	email = normalizeEmail(email)
	code = strings.TrimSpace(code)
	if code == "" {
		return ErrNoCode
	}
	now := s.now()

	codeKey := codeKey(email, PurposeRegister)
	payload, ok, err := s.Cache.Get(ctx, codeKey)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNoCode
	}
	var item Code
	if err := json.Unmarshal(payload, &item); err != nil {
		// 缓存里的格式对不上，当作没有码，顺手清掉
		_ = s.Cache.Del(ctx, codeKey)
		return ErrNoCode
	}

	// TTL 由缓存兜着，这里再判一次，覆盖「内存实现时钟漂移」这类边角
	if now.Unix() > item.CreatedAt+int64(s.TTL.Seconds()) {
		_ = s.Cache.Del(ctx, codeKey)
		return ErrExpired
	}
	if item.Attempts >= s.MaxAttempts {
		_ = s.Cache.Del(ctx, codeKey)
		return ErrTooManyAttempts
	}

	if hashCode(PurposeRegister, email, code) != item.CodeHash {
		attempts, err := s.bumpAttempts(ctx, codeKey, item)
		if err != nil {
			return err
		}
		if attempts >= s.MaxAttempts {
			// 不删键：留着这条被锁住的码，用户再试仍然能拿到「错误次数过多」，
			// 而不是含糊的「请先获取验证码」
			return ErrTooManyAttempts
		}
		return ErrMismatch
	}

	if err := s.Cache.Del(ctx, codeKey); err != nil {
		return err
	}
	log.Printf("VERIFY email=%s ok", email)
	return nil
}

// bumpAttempts 错一次记一次。
// 缓存没有 CAS，这里用「读-改-写」：并发错码可能让计数少加一次，
// 但上限仍然拦得住暴力猜 6 位数字——不值得为它加锁。
// 写回时只续上原本剩下的时间，不能让试错把验证码的有效期越试越长。
func (s *Service) bumpAttempts(ctx context.Context, codeKey string, item Code) (int, error) {
	item.Attempts++
	remaining := time.Until(time.Unix(item.CreatedAt, 0).Add(s.TTL))
	if remaining <= 0 {
		remaining = time.Minute // 已过期就不该走到这；给个下限免得 Set 成永久键
	}
	payload, err := json.Marshal(item)
	if err != nil {
		return item.Attempts, err
	}
	if err := s.Cache.Set(ctx, codeKey, payload, remaining); err != nil {
		return item.Attempts, err
	}
	return item.Attempts, nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// codeKey 一个邮箱 + 一个用途 = 一个键；后发的码覆盖前一条
func codeKey(email, purpose string) string {
	return "verify:" + purpose + ":" + email
}

// cooldownKey 重发冷却，TTL 就是重发间隔
func cooldownKey(email, purpose string) string {
	return "verify:cooldown:" + purpose + ":" + email
}

// generateCode 纯数字验证码，用 crypto/rand 而不是 math/rand
func generateCode(digits int) (string, error) {
	if digits <= 0 {
		digits = 6
	}
	var sb strings.Builder
	for i := 0; i < digits; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		sb.WriteByte(byte('0' + n.Int64()))
	}
	return sb.String(), nil
}

// hashCode 缓存里只留哈希：缓存被 dump 出来也拿不到还能用的码
func hashCode(purpose, email, code string) string {
	sum := sha256.Sum256([]byte(purpose + ":" + email + ":" + code))
	return hex.EncodeToString(sum[:])
}
