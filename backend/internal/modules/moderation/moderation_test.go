package moderation

import "testing"

func TestMatchWordsAntiBypass(t *testing.T) {
	words := []Word{
		{ID: 1, Word: "违禁词", MatchType: "contains", Action: "block", IsEnabled: true},
		{ID: 2, Word: "敏感审核", MatchType: "contains", Action: "review", IsEnabled: true},
		{ID: 3, Word: "exactword", MatchType: "exact", Action: "block", IsEnabled: true},
		{ID: 4, Word: "已禁用词", MatchType: "contains", Action: "block", IsEnabled: false},
	}

	cases := []struct {
		name string
		text string
		want string
	}{
		{"正常文本放行", "这是一段正常的技术讨论内容", ""},
		{"直接命中 block", "包含违禁词的文本", "block"},
		{"零宽字符绕过被拦截", "包\u200b含违\u200c禁\u200d词\ufeff测试", "block"},
		{"软连字符绕过被拦截", "违\u00ad禁\u00ad词", "block"},
		{"Markdown 加粗拆分绕过被拦截", "这是 违**禁**词 测试", "block"},
		{"Markdown 下划线与反引号绕过被拦截", "敏__感__`审`~~核~~内容", "review"},
		{"block 优先级高于 review", "同时出现敏感审核与违禁词", "block"},
		{"exact 大小写不敏感匹配", "  EXACTWORD  ", "block"},
		{"exact 不误拦子串", "my exactword here", ""},
		{"已停用规则不生效", "这里出现已禁用词", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := MatchWords(words, tc.text); got != tc.want {
				t.Fatalf("文本 %q 期望动作 %q，实际 %q", tc.text, tc.want, got)
			}
		})
	}
}
