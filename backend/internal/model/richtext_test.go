package model

import (
	"strings"
	"testing"
)

func TestRenderMarkdownBasic(t *testing.T) {
	out := RenderMarkdown("**加粗** 和 *斜体* 以及 `code`\n\n- 一\n- 二\n\n1. 甲\n\n> 引用\n\n# 标题")
	for _, want := range []string{
		"<strong>加粗</strong>", "<em>斜体</em>", "<code>code</code>",
		"<ul>", "<li>一</li>", "<ol>", "<li>甲</li>", "<blockquote>引用</blockquote>", "<h1>标题</h1>",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("缺少 %q，输出：%s", want, out)
		}
	}
}

func TestRenderMarkdownEscapesHTML(t *testing.T) {
	out := RenderMarkdown(`<script>alert(1)</script>`)
	if strings.Contains(out, "<script>") {
		t.Fatalf("脚本未被转义：%s", out)
	}
	if !strings.Contains(out, "&lt;script&gt;") {
		t.Fatalf("期望转义后的文本：%s", out)
	}
}

func TestRenderMarkdownRejectsUnsafeLink(t *testing.T) {
	out := RenderMarkdown("[点我](javascript:alert(1))")
	if strings.Contains(strings.ToLower(out), "javascript:") {
		t.Fatalf("不安全链接未被拦截：%s", out)
	}
}

func TestRenderMarkdownCodeBlock(t *testing.T) {
	out := RenderMarkdown("```go\nfmt.Println(\"hi\")\n```")
	if !strings.Contains(out, `<pre><code class="lang-go">`) {
		t.Fatalf("代码块渲染异常：%s", out)
	}
	if !strings.Contains(out, "fmt.Println") {
		t.Fatalf("代码块内容丢失：%s", out)
	}
	if strings.Contains(out, "<p><pre>") {
		t.Fatalf("代码块不应被段落包裹：%s", out)
	}
}
