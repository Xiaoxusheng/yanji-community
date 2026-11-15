package auth

import "testing"

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
