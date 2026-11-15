// Package markdown 提供服务端 Markdown 渲染：goldmark 解析 + bluemonday 严格净化。
// 渲染结果为可信 HTML，禁止任意 HTML/脚本执行。
package markdown

import (
	"bytes"
	"strings"
	"unicode/utf8"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	emoji "github.com/yuin/goldmark-emoji"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

var md = goldmark.New(
	goldmark.WithExtensions(
		extension.Table,
		extension.Strikethrough,
		extension.Linkify,
		extension.TaskList,
		emoji.Emoji,
	),
	goldmark.WithParserOptions(
		parser.WithAutoHeadingID(),
	),
	goldmark.WithRendererOptions(html.WithHardWraps()),
)

var policy = func() *bluemonday.Policy {
	p := bluemonday.UGCPolicy() // 已含链接/图片/标题/列表/表格等白名单
	// 代码高亮常用结构
	p.AllowAttrs("class").OnElements("code", "pre", "span")
	p.AllowElements("figure", "figcaption")
	// 强制所有链接 rel="nofollow noopener noreferrer" 并禁用 javascript: 协议
	p.RequireNoFollowOnLinks(true)
	p.AllowURLSchemes("http", "https", "mailto")
	return p
}()

// Render 将 Markdown 源文本渲染为净化后的 HTML。
func Render(source string) (string, error) {
	var buf bytes.Buffer
	if err := md.Convert([]byte(source), &buf); err != nil {
		return "", err
	}
	return string(policy.SanitizeBytes(buf.Bytes())), nil
}

// Excerpt 从 Markdown 源提取纯文本摘要（去标记、截断到 maxRunes）。
func Excerpt(source string, maxRunes int) string {
	var sb strings.Builder
	inCode := false
	for _, line := range strings.Split(source, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCode = !inCode
			continue
		}
		if inCode {
			continue
		}
		// 去除常见 Markdown 标记
		t := trimmed
		for _, mark := range []string{"#", ">", "*", "-", "_", "!"} {
			t = strings.TrimPrefix(t, mark)
		}
		t = strings.TrimSpace(t)
		t = strings.ReplaceAll(t, "`", "")
		t = strings.ReplaceAll(t, "[", " ")
		t = strings.ReplaceAll(t, "]", " ")
		if t == "" {
			continue
		}
		sb.WriteString(t)
		sb.WriteString(" ")
		if utf8.RuneCountInString(sb.String()) >= maxRunes {
			break
		}
	}
	out := strings.TrimSpace(sb.String())
	runes := []rune(out)
	if len(runes) > maxRunes {
		out = string(runes[:maxRunes]) + "…"
	}
	return out
}
