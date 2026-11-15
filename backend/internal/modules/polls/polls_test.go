package polls

import (
	"testing"
	"time"
)

func TestValidateConfig(t *testing.T) {
	validEnd := time.Now().UTC().Add(24 * time.Hour)
	cases := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{"正常单选", Config{Options: []string{"A", "B"}}, false},
		{"正常多选", Config{Options: []string{"A", "B", "C"}, MultipleChoice: true, MaxChoices: 2}, false},
		{"选项不足", Config{Options: []string{"A"}}, true},
		{"选项过多", Config{Options: []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11"}}, true},
		{"重复选项", Config{Options: []string{"A", "A"}}, true},
		{"空选项", Config{Options: []string{"A", "  "}}, true},
		{"截止时间已过", Config{Options: []string{"A", "B"}, EndsAt: &[]time.Time{time.Now().UTC().Add(-time.Hour)}[0]}, true},
		{"合法截止", Config{Options: []string{"A", "B"}, EndsAt: &validEnd}, false},
		{"max_choices 超过选项数", Config{Options: []string{"A", "B"}, MultipleChoice: true, MaxChoices: 3}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateConfig(&c.cfg)
			if c.wantErr && err == nil {
				t.Fatal("期望失败，实际通过")
			}
			if !c.wantErr && err != nil {
				t.Fatalf("期望通过，实际失败: %v", err)
			}
		})
	}
}
