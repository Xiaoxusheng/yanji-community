package storage

import (
	"strings"
	"testing"
)

func TestValidateImageMagicBytes(t *testing.T) {
	// 直接测试 matchMagic 的魔数判断
	png := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}
	if !matchMagic(png, ".png") {
		t.Fatal("PNG 魔数识别失败")
	}
	if matchMagic(png, ".jpg") {
		t.Fatal("PNG 不应识别为 JPG")
	}
	jpeg := []byte{0xFF, 0xD8, 0xFF, 0xE0}
	if !matchMagic(jpeg, ".jpg") {
		t.Fatal("JPEG 魔数识别失败")
	}
	gif := []byte("GIF89a....")
	if !matchMagic(gif, ".gif") {
		t.Fatal("GIF 魔数识别失败")
	}
	webp := append([]byte("RIFF"), make([]byte, 4)...)
	webp = append(webp, []byte("WEBP")...)
	if !matchMagic(webp, ".webp") {
		t.Fatal("WEBP 魔数识别失败")
	}
	txt := []byte("hello world")
	if matchMagic(txt, ".png") {
		t.Fatal("文本不应通过图片魔数校验")
	}
}

func TestRandomKeySafe(t *testing.T) {
	for i := 0; i < 20; i++ {
		key := RandomKey(".png")
		if !strings.HasPrefix(key, "uploads/") {
			t.Fatalf("key 前缀错误: %s", key)
		}
		if strings.Contains(key, "..") || strings.Contains(key, "\\") {
			t.Fatalf("key 含非法路径字符: %s", key)
		}
		if !strings.HasSuffix(key, ".png") {
			t.Fatalf("key 后缀错误: %s", key)
		}
	}
	// 唯一性抽查
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		k := RandomKey(".jpg")
		if seen[k] {
			t.Fatalf("key 重复: %s", k)
		}
		seen[k] = true
	}
}

func TestLocalStoragePathTraversal(t *testing.T) {
	dir := t.TempDir()
	ls, err := NewLocalStorage(dir, "/files")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := ls.LocalPath("../../etc/passwd"); ok {
		t.Fatal("路径穿越未被阻断")
	}
	if _, ok := ls.LocalPath("a/../../b"); ok {
		t.Fatal("路径穿越未被阻断")
	}
}
