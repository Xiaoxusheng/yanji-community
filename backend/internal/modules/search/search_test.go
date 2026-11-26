package search

import (
	"strings"
	"testing"
)

func TestEscapeLikeWildcards(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"golang", "golang"},
		{"100%_test", `100\%\_test`},
		{`a\b%c_d`, `a\\b\%c\_d`},
		{"%%%%", `\%\%\%\%`},
	}
	for _, tc := range cases {
		if got := EscapeLike(tc.in); got != tc.want {
			t.Fatalf("EscapeLike(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSanitizeHighlightPreventsStoredXSS(t *testing.T) {
	raw := `正文 <script>alert("xss")</script> 命中 [[YJ_MARK]]关键词[[/YJ_MARK]] 与 <img src=x onerror=alert(1)>`
	got := sanitizeHighlight(raw)

	if strings.Contains(got, "<script>") || strings.Contains(got, "<img") {
		t.Fatalf("搜索高亮存在未转义 HTML 标签: %s", got)
	}
	if !strings.Contains(got, "&lt;script&gt;") {
		t.Fatalf("危险标签应被实体转义: %s", got)
	}
	if !strings.Contains(got, "<mark>关键词</mark>") {
		t.Fatalf("安全高亮标签 <mark> 应正确还原: %s", got)
	}
}
