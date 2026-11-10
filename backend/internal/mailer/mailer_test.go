package mailer

import (
	"context"
	"strings"
	"testing"

	"github.com/imloama/umore/internal/config"
)

// log_only 模式下不发真邮件，也不该报错
func TestSendCodeLogOnly(t *testing.T) {
	m := New(config.Resend{LogOnly: true, From: "umore <no-reply@example.com>", CodeLength: 6, CodeTTLMinutes: 10})
	if !m.Configured() {
		t.Fatal("log_only 应视为可用")
	}
	if err := m.SendCode(context.Background(), "max@example.com", "123456"); err != nil {
		t.Fatalf("log_only 发码不该报错: %v", err)
	}
}

// 参数空着直接拒掉，别把空邮件丢给 Resend
func TestSendCodeEmptyArgs(t *testing.T) {
	m := New(config.Resend{LogOnly: true})
	if err := m.SendCode(context.Background(), "", "123456"); err == nil {
		t.Fatal("收件人为空应报错")
	}
	if err := m.SendCode(context.Background(), "max@example.com", ""); err == nil {
		t.Fatal("验证码为空应报错")
	}
}

// 没配 key / from 时 Configured 为 false，SendCode 会报未配置
func TestSendCodeNotConfigured(t *testing.T) {
	m := New(config.Resend{})
	if m.Configured() {
		t.Fatal("没配 key/from 不该算可用")
	}
	err := m.SendCode(context.Background(), "max@example.com", "123456")
	if err == nil || !strings.Contains(err.Error(), "未配置") {
		t.Fatalf("应报未配置, got %v", err)
	}
}

// 邮件 HTML 里要有验证码与有效期，且做了转义
func TestCodeEmailHTML(t *testing.T) {
	html := codeEmailHTML("129<", 10)
	if !strings.Contains(html, "129&lt;") {
		t.Fatal("验证码应做 HTML 转义")
	}
	if !strings.Contains(html, "10 分钟") {
		t.Fatal("正文应写明有效期")
	}
}
