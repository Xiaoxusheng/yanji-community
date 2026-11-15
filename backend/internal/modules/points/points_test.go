package points

import "testing"

func TestCalculateLevel(t *testing.T) {
	levels := []LevelInfo{
		{Level: 0, Name: "新用户", MinPoints: 0},
		{Level: 1, Name: "见习", MinPoints: 50},
		{Level: 2, Name: "初级", MinPoints: 200},
		{Level: 3, Name: "活跃", MinPoints: 500},
	}
	cases := []struct {
		total    int64
		wantLvl  int
		wantNext int
	}{
		{0, 0, 1},
		{49, 0, 1},
		{50, 1, 2},
		{199, 1, 2},
		{200, 2, 3},
		{10000, 3, 0}, // 顶级无下一级
	}
	for _, c := range cases {
		cur, next := CalculateLevel(levels, c.total)
		if cur.Level != c.wantLvl {
			t.Fatalf("total=%d 期望等级 %d 实际 %d", c.total, c.wantLvl, cur.Level)
		}
		if c.wantNext == 0 {
			if next != nil {
				t.Fatalf("total=%d 顶级不应有下一级", c.total)
			}
		} else if next == nil || next.Level != c.wantNext {
			t.Fatalf("total=%d 下一级期望 %d", c.total, c.wantNext)
		}
	}
}

func TestValidatePollConfig(t *testing.T) {
	// 注意：ValidateConfig 属于 polls 包，这里只验证本包积分常量不被误改
	if PointsAnswerAccepted != 20 || PointsSignIn != 5 {
		t.Fatal("积分规则常量被意外修改")
	}
}
