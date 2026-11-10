package model

import (
	"html"
	"regexp"
	"strings"
)

// 富文本渲染：评论只接受一小组安全的 Markdown 语法，
// 渲染时先把原始内容整体转义，再还原 Markdown 结构，避免 XSS。

var (
	reCodeBlock  = regexp.MustCompile("(?s)```([a-zA-Z0-9+#-]*)\\n?(.*?)```")
	reInlineCode = regexp.MustCompile("`([^`\\n]+)`")
	reImage      = regexp.MustCompile(`!\[([^\]]*)\]\(([^)\s]+)\)`)
	reLink       = regexp.MustCompile(`\[([^\]]+)\]\(([^)\s]+)\)`)
	reBold       = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	reItalic     = regexp.MustCompile(`(^|[^*])\*([^*\n]+)\*`)
	reStrike     = regexp.MustCompile(`~~([^~]+)~~`)
	reHeading    = regexp.MustCompile(`^(#{1,4})\s+(.*)$`)
	reOrdered    = regexp.MustCompile(`^\d+\.\s+(.*)$`)
	reUnordered  = regexp.MustCompile(`^[-*]\s+(.*)$`)
	reTaskItem   = regexp.MustCompile(`^[-*]\s+\[([ xX])\]\s+(.*)$`)
	reQuote      = regexp.MustCompile(`^(&gt;|>)\s?(.*)$`)
)

func safeURL(u string) bool {
	lower := strings.ToLower(strings.TrimSpace(u))
	return strings.HasPrefix(lower, "http://") ||
		strings.HasPrefix(lower, "https://") ||
		strings.HasPrefix(lower, "/") ||
		strings.HasPrefix(lower, "mailto:")
}

// RenderMarkdown 把受限 Markdown 渲染为 HTML
func RenderMarkdown(src string) string {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	// 1. 抽出代码块，避免其中内容被其它规则处理
	codes := []string{}
	src = reCodeBlock.ReplaceAllStringFunc(src, func(block string) string {
		m := reCodeBlock.FindStringSubmatch(block)
		lang := strings.TrimSpace(m[1])
		body := strings.TrimSuffix(m[2], "\n")
		class := ""
		if lang != "" {
			class = ` class="lang-` + html.EscapeString(lang) + `"`
		}
		codes = append(codes, "<pre><code"+class+">"+html.EscapeString(body)+"</code></pre>")
		return "\x00CODE" + itoa(len(codes)-1) + "\x00"
	})

	// 2. 整体转义
	src = html.EscapeString(src)

	// 3. 行内语法
	src = reInlineCode.ReplaceAllString(src, "<code>$1</code>")
	src = reImage.ReplaceAllStringFunc(src, func(s string) string {
		m := reImage.FindStringSubmatch(s)
		if !safeURL(html.UnescapeString(m[2])) {
			return m[1]
		}
		return `<img src="` + m[2] + `" alt="` + m[1] + `" />`
	})
	src = reLink.ReplaceAllStringFunc(src, func(s string) string {
		m := reLink.FindStringSubmatch(s)
		if !safeURL(html.UnescapeString(m[2])) {
			return m[1]
		}
		return `<a href="` + m[2] + `" target="_blank" rel="noopener noreferrer">` + m[1] + "</a>"
	})
	src = reBold.ReplaceAllString(src, "<strong>$1</strong>")
	src = reItalic.ReplaceAllString(src, "$1<em>$2</em>")
	src = reStrike.ReplaceAllString(src, "<del>$1</del>")

	// 4. 块级语法
	lines := strings.Split(src, "\n")
	out := []string{}
	listType := ""
	closeList := func() {
		if listType != "" {
			out = append(out, "</"+listType+">")
			listType = ""
		}
	}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "":
			closeList()
			out = append(out, "")
		case reHeading.MatchString(trimmed):
			closeList()
			m := reHeading.FindStringSubmatch(trimmed)
			level := len(m[1])
			out = append(out, "<h"+itoa(level)+">"+m[2]+"</h"+itoa(level)+">")
		case reQuote.MatchString(trimmed):
			closeList()
			m := reQuote.FindStringSubmatch(trimmed)
			out = append(out, "<blockquote>"+m[2]+"</blockquote>")
		case reTaskItem.MatchString(trimmed):
			// Markdown 待办项渲染成勾选框，评论里写 - [ ] xxx 才看得出是清单
			if listType != "ul" {
				closeList()
				listType = "ul"
				out = append(out, `<ul class="task-list">`)
			}
			m := reTaskItem.FindStringSubmatch(trimmed)
			mark := "&#9744;"
			cls := "task-item"
			if m[1] != " " {
				mark = "&#9745;"
				cls = "task-item is-done"
			}
			out = append(out, `<li class="`+cls+`">`+mark+" "+m[2]+"</li>")
		case reUnordered.MatchString(trimmed):
			if listType != "ul" {
				closeList()
				listType = "ul"
				out = append(out, "<ul>")
			}
			out = append(out, "<li>"+reUnordered.FindStringSubmatch(trimmed)[1]+"</li>")
		case reOrdered.MatchString(trimmed):
			if listType != "ol" {
				closeList()
				listType = "ol"
				out = append(out, "<ol>")
			}
			out = append(out, "<li>"+reOrdered.FindStringSubmatch(trimmed)[1]+"</li>")
		case strings.Contains(trimmed, "\x00CODE"):
			// 占位符独占一行时不要再包 <p>，避免代码块外多出段落边距
			closeList()
			out = append(out, trimmed)
		default:
			closeList()
			out = append(out, "<p>"+strings.TrimSuffix(line, "  ")+"</p>")
		}
	}
	closeList()

	result := strings.Join(out, "\n")
	for i, code := range codes {
		result = strings.ReplaceAll(result, "\x00CODE"+itoa(i)+"\x00", code)
	}
	return result
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}
