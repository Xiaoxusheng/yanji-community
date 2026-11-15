package posts

import "testing"

func TestValidateContent(t *testing.T) {
	long := string(make([]byte, 50))
	cases := []struct {
		name     string
		title    string
		body     string
		tags     []string
		maxTags  int
		wantFail bool
	}{
		{"正常", "这是一个合法的标题", "正文内容", []string{"go"}, 5, false},
		{"标题过短", "短", "正文", nil, 5, true},
		{"标题过长", long + string(make([]byte, 160)), "正文", nil, 5, true},
		{"正文为空", "这是一个合法的标题", "   ", nil, 5, true},
		{"标签过多", "这是一个合法的标题", "正文", []string{"a", "b", "c", "d", "e", "f"}, 5, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateContent(c.title, c.body, c.tags, c.maxTags)
			if c.wantFail && err == nil {
				t.Fatal("期望失败，实际通过")
			}
			if !c.wantFail && err != nil {
				t.Fatalf("期望通过，实际失败: %v", err)
			}
		})
	}
}

func TestViewerCanModerateCategory(t *testing.T) {
	staff := &Viewer{IsStaff: true}
	if !staff.CanModerateCategory(999) {
		t.Fatal("staff 应可管理任意版块")
	}
	mod := &Viewer{ModeratedCategories: []int64{1, 2}}
	if !mod.CanModerateCategory(2) {
		t.Fatal("授权版主应可管理版块 2")
	}
	if mod.CanModerateCategory(3) {
		t.Fatal("未授权版主不应可管理版块 3")
	}
	normal := &Viewer{}
	if normal.CanModerateCategory(1) {
		t.Fatal("普通用户不应可管理版块")
	}
}
