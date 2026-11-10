// Package mailer 负责发注册验证码邮件，走 Resend。
// 只做一件事：把验证码渲染成邮件发出去；验证码本身怎么生成、怎么校验不在这里。
package mailer

import (
	"context"
	"fmt"
	"html"
	"log"
	"strings"
	"time"

	"github.com/imloama/umore/internal/config"

	resend "github.com/resend/resend-go/v4"
)

// Mailer 发信入口。Resend 未配置时仍能构造，只是 Send 会返回错误，
// 好在注册接口里给出「邮件服务没配好」这种能看懂的提示。
type Mailer struct {
	cfg    config.Resend
	client *resend.Client
}

func New(cfg config.Resend) *Mailer {
	m := &Mailer{cfg: cfg}
	if key := cfg.APIKeyValue(); key != "" {
		m.client = resend.NewClient(key)
	}
	return m
}

// Configured 没配置时注册页可以直接告诉用户「这台机器没开发信」
func (m *Mailer) Configured() bool {
	return m.cfg.Configured()
}

// CodeTTL 邮件里写的有效期，跟后端校验用的是同一个值
func (m *Mailer) CodeTTL() time.Duration { return m.cfg.CodeTTL() }

// ResendInterval 前端倒计时也按这个来
func (m *Mailer) ResendInterval() time.Duration { return m.cfg.ResendInterval() }

func (m *Mailer) CodeDigits() int { return m.cfg.CodeLength }

// MaxAttempts 单条码最多试错几次
func (m *Mailer) MaxAttempts() int { return m.cfg.MaxAttempts }

// LogOnly 只打日志不发信（本地开发）
func (m *Mailer) LogOnly() bool { return m.cfg.LogOnly }

// MaskedFrom 日志里只露发件人，不露 key
func (m *Mailer) MaskedFrom() string { return m.cfg.FromValue() }

// SendCode 发一封带验证码的邮件。
// 幂等键用「用途 + 邮箱 + 验证码」，同一封重试不会重复投递。
func (m *Mailer) SendCode(ctx context.Context, to, code string) error {
	if strings.TrimSpace(to) == "" || strings.TrimSpace(code) == "" {
		return fmt.Errorf("mailer: empty recipient or code")
	}
	if m.cfg.LogOnly {
		log.Printf("MAIL(log_only) to=%s code=%s ttl=%s", to, code, m.cfg.CodeTTL())
		return nil
	}
	if m.client == nil || m.cfg.FromValue() == "" {
		return fmt.Errorf("mailer: resend api_key / from 未配置")
	}

	ttl := int(m.cfg.CodeTTL().Minutes())
	params := &resend.SendEmailRequest{
		From:    m.cfg.FromValue(),
		To:      []string{to},
		Subject: fmt.Sprintf("umore 注册验证码：%s", code),
		Html:    codeEmailHTML(code, ttl),
		Text:    fmt.Sprintf("你的 umore 注册验证码是 %s，%d 分钟内有效。不是本人操作请忽略。", code, ttl),
		Tags:    []resend.Tag{{Name: "type", Value: "verify-code"}},
	}

	// 重试用同一个幂等键，避免网络抖动发出两封不一样的码
	opts := &resend.SendEmailOptions{
		IdempotencyKey: fmt.Sprintf("verify-code/%s/%s", to, code),
	}

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(attempt) * 500 * time.Millisecond):
			}
		}
		resp, err := m.client.Emails.SendWithOptions(ctx, params, opts)
		if err == nil {
			log.Printf("MAIL sent to=%s id=%s", to, resp.Id)
			return nil
		}
		lastErr = err
		// 4xx 是参数或权限问题，重试没意义；只有 429/5xx 才退避重来
		if !retryable(err) {
			break
		}
	}
	return fmt.Errorf("mailer: send to %s failed: %w", to, lastErr)
}

func retryable(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "429") || strings.Contains(msg, "500") ||
		strings.Contains(msg, "502") || strings.Contains(msg, "503") ||
		strings.Contains(msg, "timeout") || strings.Contains(msg, "connection")
}

// codeEmailHTML 纯内联样式单列表格，各家邮箱客户端都认
func codeEmailHTML(code string, ttlMinutes int) string {
	digits := html.EscapeString(code)
	return `<!doctype html><html><body style="margin:0;padding:24px;background:#f6f7f9;font-family:-apple-system,Segoe UI,Helvetica,Arial,sans-serif;color:#1f2328">
  <table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:480px;margin:0 auto;background:#ffffff;border:1px solid #e5e7eb;border-radius:12px">
    <tr><td style="padding:28px 28px 8px">
      <div style="font-size:14px;font-weight:600;color:#6b7280;letter-spacing:.04em">umore</div>
      <h1 style="margin:12px 0 0;font-size:20px;line-height:1.4">注册验证码</h1>
    </td></tr>
    <tr><td style="padding:16px 28px 0;font-size:14px;line-height:1.7;color:#374151">
      在 umore 注册页面填入下面的验证码即可完成注册：
    </td></tr>
    <tr><td style="padding:20px 28px">
      <div style="font-size:34px;font-weight:700;letter-spacing:.36em;padding:16px 0;text-align:center;background:#f3f4f6;border-radius:10px;color:#111827">` + digits + `</div>
    </td></tr>
    <tr><td style="padding:0 28px 8px;font-size:13px;line-height:1.7;color:#6b7280">
      验证码 ` + fmt.Sprint(ttlMinutes) + ` 分钟内有效，只能用来注册这一个账号。<br>
      不是你在注册的话，忽略这封邮件就行，不会产生任何账号。
    </td></tr>
    <tr><td style="padding:16px 28px 28px;font-size:12px;line-height:1.6;color:#9ca3af;border-top:1px solid #f3f4f6">
      这封邮件由系统自动发送，直接回复收不到。
    </td></tr>
  </table>
</body></html>`
}
