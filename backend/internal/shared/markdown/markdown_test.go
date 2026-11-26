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

func TestRenderXSSPayloadMatrix(t *testing.T) {
	payloads := []struct {
		name      string
		input     string
		forbidden []string
	}{
		{
			name:      "SVG onload XSS",
			input:     `<svg onload="alert(1)"><circle r="10"/></svg>`,
			forbidden: []string{"<svg", "onload", "alert(1)"},
		},
		{
			name:      "iframe srcdoc XSS",
			input:     `<iframe srcdoc="<script>alert(1)</script>"></iframe>`,
			forbidden: []string{"<iframe", "srcdoc", "<script"},
		},
		{
			name:      "MathML 嵌套脚本",
			input:     `<math><mtext><script>alert(1)</script></mtext></math>`,
			forbidden: []string{"<math", "<mtext", "<script"},
		},
		{
			name:      "vbscript 伪协议链接",
			input:     `[click](vbscript:msgbox(1))`,
			forbidden: []string{"vbscript:"},
		},
		{
			name:      "data:text/html 伪协议链接",
			input:     `[click](data:text/html,<script>alert(1)</script>)`,
			forbidden: []string{"data:text/html", "<script"},
		},
		{
			name:      "HTML 实体编码 javascript 协议",
			input:     `<a href="javascript&#58;alert(1)">click</a>`,
			forbidden: []string{"javascript:", "alert(1)"},
		},
		{
			name:      "非法 class 属性值被拦截",
			input:     `<code class="lang-go evil space">x</code>`,
			forbidden: []string{"evil space"},
		},
	}

	for _, tc := range payloads {
		t.Run(tc.name, func(t *testing.T) {
			out, err := Render(tc.input)
			if err != nil {
				t.Fatalf("渲染报错: %v", err)
			}
			low := strings.ToLower(out)
			for _, bad := range tc.forbidden {
				if strings.Contains(low, bad) {
					t.Fatalf("检测到未净化的危险片段 %q，输出: %s", bad, out)
				}
			}
		})
	}

	// 外链必须自动附带 noreferrer 与 nofollow
	out, err := Render("[ext](https://example.com/path)")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "noreferrer") || !strings.Contains(out, "nofollow") {
		t.Fatalf("外链缺少 nofollow/noreferrer 保护: %s", out)
	}
}
