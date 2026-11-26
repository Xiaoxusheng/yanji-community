package storage

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
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
	for _, bad := range []string{
		"../../etc/passwd",
		"a/../../b",
		"/etc/passwd",
		`uploads\..\..\secret.png`,
		"uploads/bad\x00.png",
	} {
		if _, ok := ls.LocalPath(bad); ok {
			t.Fatalf("路径穿越变体未被阻断: %q", bad)
		}
	}
}

func makeFileHeader(t *testing.T, filename string, data []byte) *multipart.FileHeader {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fw.Write(data); err != nil {
		t.Fatal(err)
	}
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/upload", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if err := req.ParseMultipartForm(4 << 20); err != nil {
		t.Fatal(err)
	}
	return req.MultipartForm.File["file"][0]
}

func TestValidateImageStrictSecurity(t *testing.T) {
	validPNG := append([]byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}, make([]byte, 32)...)

	// 1. 合法 8 字节 PNG 头通过
	ext, err := ValidateImage(makeFileHeader(t, "avatar.png", validPNG), 1024)
	if err != nil || ext != ".png" {
		t.Fatalf("合法 PNG 应通过校验，got ext=%q err=%v", ext, err)
	}

	// 2. 截断为 4 字节的伪造 PNG 被拒绝（严格 8 字节魔数）
	shortPNG := []byte{0x89, 'P', 'N', 'G', 'X', 'Y', 'Z', '0'}
	if _, err := ValidateImage(makeFileHeader(t, "fake.png", shortPNG), 1024); err == nil {
		t.Fatal("仅匹配前 4 字节的伪造 PNG 应被拒绝")
	}

	// 3. 危险扩展名（SVG / HTML / PHP / 双扩展名）全部拒绝
	for _, name := range []string{"xss.svg", "page.html", "shell.php", "avatar.png.php"} {
		if _, err := ValidateImage(makeFileHeader(t, name, validPNG), 1024); err == nil {
			t.Fatalf("非白名单扩展名 %q 应被拒绝", name)
		}
	}

	// 4. 伪装为 .png 的 HTML/SVG 多态内容被魔数校验拒绝
	htmlAsPNG := []byte("<html><script>alert(1)</script></html>")
	if _, err := ValidateImage(makeFileHeader(t, "evil.png", htmlAsPNG), 1024); err == nil {
		t.Fatal("HTML 内容伪装为 .png 应被魔数校验拦截")
	}

	// 5. 超大体积文件被拒绝
	if _, err := ValidateImage(makeFileHeader(t, "big.png", validPNG), 16); err == nil {
		t.Fatal("超过 maxBytes 限制的文件应被拒绝")
	}
}
