package messaging

import (
	"testing"
	"time"
)

func TestPairKey(t *testing.T) {
	cases := []struct{ a, b int64 }{
		{1, 2}, {2, 1}, {100, 9}, {9, 100},
	}
	want := "dm:1:2"
	if PairKey(1, 2) != want || PairKey(2, 1) != want {
		t.Fatal("pair key 未按无序对归一")
	}
	if PairKey(100, 9) != PairKey(9, 100) {
		t.Fatal("大小组内未归一")
	}
	if PairKey(100, 9) == want {
		t.Fatal("不同用户对产生了相同键")
	}
	_ = cases
}

func TestCanRecall(t *testing.T) {
	now := time.Now().UTC()
	if !CanRecall(now.Add(-time.Minute), now) {
		t.Fatal("1 分钟前的消息应可撤回")
	}
	if CanRecall(now.Add(-6*time.Minute), now) {
		t.Fatal("6 分钟前的消息不应可撤回")
	}
	if !CanRecall(now, now) {
		t.Fatal("刚发出的消息应可撤回")
	}
	if CanRecall(now.Add(time.Minute), now) {
		t.Fatal("未来时间不应可撤回")
	}
}

func TestPreviewOf(t *testing.T) {
	if got := previewOf("abcdef", 3); got != "abc…" {
		t.Fatalf("previewOf 截断错误: %s", got)
	}
	if got := previewOf("abc", 10); got != "abc" {
		t.Fatalf("短文本不应截断: %s", got)
	}
}

func TestTrimSpace(t *testing.T) {
	if got := trimSpace("\n\t 你好 \r\n"); got != "你好" {
		t.Fatalf("trimSpace 错误: %q", got)
	}
	if trimSpace("   ") != "" {
		t.Fatal("纯空白应归空")
	}
}
