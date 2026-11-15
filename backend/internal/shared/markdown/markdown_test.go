package markdown

import (
	"strings"
	"testing"
)

func TestRenderSanitizesScript(t *testing.T) {
	out, err := Render("# Hello\n\n<script>alert(1)</script>")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(out), "<script") {
		t.Fatalf("script 标签未被净化: %s", out)
	}
	if !strings.Contains(out, "Hello") {
		t.Fatalf("正常内容丢失: %s", out)
	}
}

func TestRenderRemovesJavascriptLinks(t *testing.T) {
	out, err := Render("[click](javascript:alert(1))")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(out), "javascript:") {
		t.Fatalf("javascript: 链接未被移除: %s", out)
	}
}

func TestRenderKeepsSafeMarkdown(t *testing.T) {
	out, err := Render("**bold** and [link](https://example.com) and `code`")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "<strong>bold</strong>") {
		t.Fatalf("加粗渲染失败: %s", out)
	}
	if !strings.Contains(out, `href="https://example.com"`) {
		t.Fatalf("链接渲染失败: %s", out)
	}
}

func TestRenderStripsEventHandlers(t *testing.T) {
	out, err := Render(`<img src="x" onerror="alert(1)">`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(out), "onerror") {
		t.Fatalf("事件处理器未被移除: %s", out)
	}
}

func TestExcerpt(t *testing.T) {
	ex := Excerpt("# 标题\n\n这是正文第一段。\n\n```go\ncode block\n```\n\n第二段。", 50)
	if strings.Contains(ex, "code block") {
		t.Fatalf("代码块不应进入摘要: %s", ex)
	}
	if !strings.Contains(ex, "这是正文第一段") {
		t.Fatalf("摘要内容错误: %s", ex)
	}
}

func TestExcerptTruncates(t *testing.T) {
	ex := Excerpt(strings.Repeat("长", 200), 10)
	if len([]rune(ex)) > 12 {
		t.Fatalf("摘要未截断: %d", len([]rune(ex)))
	}
}
