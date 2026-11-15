package comments

import (
	"strconv"
	"testing"
	"time"
)

func rowFor(id int64, parent *int64, at time.Time) row {
	r := row{ID: id, PublicID: "c" + strconv.FormatInt(id, 10), CreatedAt: at}
	if parent != nil {
		p := *parent
		r.ParentID = &p
	}
	return r
}

func TestBuildTreeDepth(t *testing.T) {
	base := time.Now()
	var pid int64 = 1
	rows := []row{
		rowFor(1, nil, base),
		rowFor(2, &pid, base.Add(time.Minute)),
		rowFor(3, &pid, base.Add(2*time.Minute)),
	}
	tree := buildTree(rows, 0)
	if len(tree) != 1 {
		t.Fatalf("应只有 1 个顶层评论，实际 %d", len(tree))
	}
	if len(tree[0].Replies) != 2 {
		t.Fatalf("应有 2 条楼中楼回复，实际 %d", len(tree[0].Replies))
	}
	if tree[0].Replies[0].ID == "" || tree[0].Replies[1].ID == "" {
		t.Fatal("回复 ID 不应缺失")
	}
	// 楼中楼按时间升序
	if !tree[0].Replies[0].CreatedAt.Before(tree[0].Replies[1].CreatedAt) {
		t.Fatal("回复未按时间排序")
	}
}

func TestMentionRegex(t *testing.T) {
	hits := mentionRe.FindAllStringSubmatch("你好 @alice 和 @bob_2，@bad-name @x", -1)
	var names []string
	for _, h := range hits {
		names = append(names, h[1])
	}
	// @bad-name 中的 "bad" 是合法用户名模式（后随分隔符即截断），与主流论坛分词一致
	if len(names) != 3 || names[0] != "alice" || names[1] != "bob_2" || names[2] != "bad" {
		t.Fatalf("提及解析错误: %v", names)
	}
}
