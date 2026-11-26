package auth

import (
	"context"
	"strings"
	"testing"
)

func TestValidatePassword(t *testing.T) {
	cases := []struct {
		name string
		pwd  string
		ok   bool
	}{
		{"字母+数字", "password123", true},
		{"大小写+数字+符号", "Admin@2024", true},
		{"纯数字过弱", "12345678", false},
		{"纯小写字母", "abcdefgh", false},
		{"常见弱口令", "admin123", false},
		{"全相同字符", "aaaaaaaa", false},
		{"全空白", "        ", false},
		{"长度不足", "Aa12345", false},
		{"长度超限", string(make([]byte, 73)), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidatePassword(c.pwd)
			if c.ok && err != nil {
				t.Fatalf("期望通过，实际失败: %v", err)
			}
			if !c.ok && err == nil {
				t.Fatal("期望失败，实际通过")
			}
		})
	}
}

func TestValidateRegister(t *testing.T) {
	cases := []struct {
		name, username, email, password string
		ok                              bool
	}{
		{"正常注册", "hello_world1", "a@b.com", "password123", true},
		{"用户名过短", "ab", "a@b.com", "password123", false},
		{"用户名含非法字符", "hello-world", "a@b.com", "password123", false},
		{"用户名过长", "a123456789012345678901", "a@b.com", "password123", false},
		{"邮箱缺失@", "user1", "abc.com", "password123", false},
		{"密码过短", "user1", "a@b.com", "1234567", false},
		{"密码超长", "user1", "a@b.com", string(make([]byte, 73)), false},
		{"弱密码拒绝", "user1", "a@b.com", "12345678", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateRegister(c.username, c.email, c.password)
			if c.ok && err != nil {
				t.Fatalf("期望通过，实际失败: %v", err)
			}
			if !c.ok && err == nil {
				t.Fatal("期望失败，实际通过")
			}
		})
	}
}

func TestPasswordHashRoundtrip(t *testing.T) {
	hash, err := HashPassword("s3cret-password")
	if err != nil {
		t.Fatal(err)
	}
	if hash == "s3cret-password" || len(hash) < 50 {
		t.Fatalf("哈希不符合 bcrypt 特征: %s", hash)
	}
	if !CheckPassword(hash, "s3cret-password") {
		t.Fatal("正确密码校验失败")
	}
	if CheckPassword(hash, "wrong-password") {
		t.Fatal("错误密码竟然通过")
	}
}

func TestTokenHashDeterministic(t *testing.T) {
	h1 := TokenHash("abc")
	h2 := TokenHash("abc")
	h3 := TokenHash("abd")
	if h1 != h2 || h1 == h3 || len(h1) != 64 {
		t.Fatal("token 摘要行为不符合预期")
	}
}

func TestCaptchaStoreOneTimeAndRender(t *testing.T) {
	ctx := context.Background()
	store := NewCaptchaStore(nil)

	ch, err := store.Generate(ctx)
	if err != nil {
		t.Fatalf("生成验证码失败: %v", err)
	}
	if len(ch.ID) != 32 || !strings.HasPrefix(ch.ImageData, "data:image/png;base64,") {
		t.Fatalf("验证码响应格式异常: id=%q prefix=%q", ch.ID, ch.ImageData[:min(32, len(ch.ImageData))])
	}
	if err := store.Verify(ctx, ch.ID, "WRONG"); err == nil {
		t.Fatal("错误验证码不应通过")
	}
	// 错误尝试后同一 captcha_id 应被立即销毁防暴破
	if err := store.Verify(ctx, ch.ID, "ABCD"); err == nil {
		t.Fatal("已消费的验证码应已销毁")
	}

	// 手动写入已知密文验证大小写不敏感 + 成功后即消费防重放
	store.putMem("testid0123456789abcdef0123456789", TokenHash("testid0123456789abcdef0123456789:AB3C"))
	if err := store.Verify(ctx, "testid0123456789abcdef0123456789", "ab3c"); err != nil {
		t.Fatalf("大小写不敏感匹配应通过: %v", err)
	}
	if err := store.Verify(ctx, "testid0123456789abcdef0123456789", "AB3C"); err == nil {
		t.Fatal("验证通过后禁止重放")
	}
}

func TestLoginLockoutMemoryFallback(t *testing.T) {
	ctx := context.Background()
	svc := &Service{failMem: make(map[string]loginFailEntry)}
	for i := 0; i < loginMaxFailPerPair-1; i++ {
		svc.recordLoginFailure(ctx, "Alice", "1.2.3.4")
		if err := svc.checkLoginLockout(ctx, "alice", "1.2.3.4"); err != nil {
			t.Fatalf("未达阈值不应锁定: %v", err)
		}
	}
	svc.recordLoginFailure(ctx, "Alice", "1.2.3.4")
	if err := svc.checkLoginLockout(ctx, "alice", "1.2.3.4"); err == nil {
		t.Fatal("连续 5 次失败后应触发账户+IP锁定")
	}
	svc.clearLoginFailure(ctx, "alice", "1.2.3.4")
	if err := svc.checkLoginLockout(ctx, "alice", "1.2.3.4"); err != nil {
		t.Fatalf("成功后应清空锁定状态: %v", err)
	}
}
