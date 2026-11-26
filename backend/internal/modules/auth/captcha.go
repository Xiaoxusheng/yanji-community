package auth

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"image"
	"image/color"
	"image/png"
	"math"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/yanji-community/backend/internal/shared/apperr"
)

const (
	// 剔除 0/O、1/I/l、5/S、2/Z 等易混淆字符，兼顾辨识度与抗简单猜测。
	captchaAlphabet = "2346789ABCDEFGHJKLMNPQRTUVWXY"
	captchaLength   = 4
	captchaTTL      = 5 * time.Minute
	captchaMaxMem   = 10000
)

// CaptchaChallenge 返回给前端的图形验证码载荷。
type CaptchaChallenge struct {
	ID        string `json:"captcha_id"`
	ImageData string `json:"image_data"`
	ExpiresIn int    `json:"expires_in"`
}

// CaptchaVerifier 图形验证码生成与一次性校验接口。
type CaptchaVerifier interface {
	Generate(ctx context.Context) (CaptchaChallenge, error)
	Verify(ctx context.Context, id, code string) error
}

type captchaEntry struct {
	digest    string
	expiresAt time.Time
}

// CaptchaStore 图形验证码存储与渲染器（优先 Redis，未配置时安全降级为有界内存缓存）。
type CaptchaStore struct {
	rdb *redis.Client
	mu  sync.Mutex
	mem map[string]captchaEntry
	ttl time.Duration
}

// NewCaptchaStore 构造验证码服务。
func NewCaptchaStore(rdb *redis.Client) *CaptchaStore {
	return &CaptchaStore{
		rdb: rdb,
		mem: make(map[string]captchaEntry),
		ttl: captchaTTL,
	}
}

var luaGetDel = redis.NewScript(`
local v = redis.call('GET', KEYS[1])
if v then
	redis.call('DEL', KEYS[1])
end
return v
`)

// Generate 生成一次性图形验证码并写入存储。
func (s *CaptchaStore) Generate(ctx context.Context) (CaptchaChallenge, error) {
	idBytes := make([]byte, 16)
	if _, err := rand.Read(idBytes); err != nil {
		return CaptchaChallenge{}, apperr.Internal(err)
	}
	id := hex.EncodeToString(idBytes)

	codeBuf := make([]byte, captchaLength)
	for i := range codeBuf {
		idx := cryptoRandInt(len(captchaAlphabet))
		codeBuf[i] = captchaAlphabet[idx]
	}
	code := string(codeBuf)

	imgData, err := renderCaptchaPNG(code)
	if err != nil {
		return CaptchaChallenge{}, apperr.Internal(err)
	}

	digest := TokenHash(id + ":" + code)
	if s.rdb != nil {
		if err := s.rdb.Set(ctx, captchaRedisKey(id), digest, s.ttl).Err(); err != nil {
			return CaptchaChallenge{}, apperr.Internal(err)
		}
	} else {
		s.putMem(id, digest)
	}

	return CaptchaChallenge{
		ID:        id,
		ImageData: imgData,
		ExpiresIn: int(s.ttl.Seconds()),
	}, nil
}

// Verify 原子消费并校验验证码：无论成功或失败，同一 ID 均立即失效，防止重放和单码爆破。
func (s *CaptchaStore) Verify(ctx context.Context, id, code string) error {
	id = strings.TrimSpace(id)
	code = strings.ToUpper(strings.TrimSpace(code))
	if len(id) < 16 || len(id) > 64 || len(code) != captchaLength {
		return apperr.BadRequest("CAPTCHA_INVALID", "请输入正确的图形验证码")
	}

	var stored string
	if s.rdb != nil {
		res, err := luaGetDel.Run(ctx, s.rdb, []string{captchaRedisKey(id)}).Result()
		if err != nil {
			return apperr.BadRequest("CAPTCHA_INVALID", "验证码已过期或失效，请点击刷新")
		}
		str, ok := res.(string)
		if !ok || str == "" {
			return apperr.BadRequest("CAPTCHA_INVALID", "验证码已过期或失效，请点击刷新")
		}
		stored = str
	} else {
		var ok bool
		stored, ok = s.popMem(id)
		if !ok {
			return apperr.BadRequest("CAPTCHA_INVALID", "验证码已过期或失效，请点击刷新")
		}
	}

	want := TokenHash(id + ":" + code)
	if subtle.ConstantTimeCompare([]byte(stored), []byte(want)) != 1 {
		return apperr.BadRequest("CAPTCHA_INVALID", "图形验证码不正确，请重新输入")
	}
	return nil
}

func captchaRedisKey(id string) string { return "captcha:" + id }

func (s *CaptchaStore) putMem(id, digest string) {
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.mem) >= captchaMaxMem {
		for k, v := range s.mem {
			if now.After(v.expiresAt) {
				delete(s.mem, k)
			}
		}
	}
	// 防止恶意高频刷码耗尽内存
	for len(s.mem) >= captchaMaxMem {
		for k := range s.mem {
			delete(s.mem, k)
			break
		}
	}
	s.mem[id] = captchaEntry{digest: digest, expiresAt: now.Add(s.ttl)}
}

func (s *CaptchaStore) popMem(id string) (string, bool) {
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	ent, ok := s.mem[id]
	delete(s.mem, id)
	if !ok || now.After(ent.expiresAt) {
		return "", false
	}
	return ent.digest, true
}

func cryptoRandInt(max int) int {
	if max <= 1 {
		return 0
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0
	}
	return int(n.Int64())
}

// ---- 纯标准库 PNG 图形渲染（无外部字体与 CGO 依赖，抗简单 OCR 且无 SVG 脚本风险） ----

const (
	capW = 240
	capH = 80
)

var inkPalette = []color.RGBA{
	{R: 20, G: 22, B: 27, A: 255},  // 深墨
	{R: 30, G: 58, B: 138, A: 255}, // 钴蓝
	{R: 30, G: 41, B: 59, A: 255},  // 石墨
	{R: 15, G: 76, B: 92, A: 255},  // 深青
}

func renderCaptchaPNG(code string) (string, error) {
	bg := image.NewRGBA(image.Rect(0, 0, capW, capH))
	// 柔和中性浅色渐变底板
	for y := 0; y < capH; y++ {
		t := float64(y) / float64(capH)
		r := uint8(246 - t*8)
		g := uint8(248 - t*6)
		b := uint8(252 - t*5)
		for x := 0; x < capW; x++ {
			bg.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}

	// 字符绘制到临时图层后做轻度波形扭曲
	layer := image.NewRGBA(image.Rect(0, 0, capW, capH))
	stepX := (capW - 36) / len(code)
	for i, ch := range code {
		glyph, ok := captchaGlyphs[byte(ch)]
		if !ok {
			continue
		}
		col := inkPalette[cryptoRandInt(len(inkPalette))]
		ox := 20 + i*stepX + cryptoRandInt(8) - 4
		oy := 18 + cryptoRandInt(12) - 6
		scale := 6 + cryptoRandInt(2) // 5*6 = 30px 宽，7*6 = 42px 高
		shear := (float64(cryptoRandInt(11)) - 5.0) * 0.045
		drawGlyph(layer, glyph, ox, oy, scale, shear, col)
	}

	// 正弦波轻度扭曲，抑制模板匹配同时保持清晰可读
	ampX := 2.2 + float64(cryptoRandInt(10))*0.15
	ampY := 1.6 + float64(cryptoRandInt(10))*0.12
	phase := float64(cryptoRandInt(314)) / 50.0
	for y := 0; y < capH; y++ {
		for x := 0; x < capW; x++ {
			sx := int(math.Round(float64(x) + ampX*math.Sin(float64(y)*0.11+phase)))
			sy := int(math.Round(float64(y) + ampY*math.Sin(float64(x)*0.07+phase)))
			if sx >= 0 && sx < capW && sy >= 0 && sy < capH {
				c := layer.RGBAAt(sx, sy)
				if c.A > 0 {
					blendPixel(bg, x, y, c)
				}
			}
		}
	}

	// 3 条贯穿弧线（细描边）
	for i := 0; i < 3; i++ {
		c := inkPalette[cryptoRandInt(len(inkPalette))]
		c.A = 140
		y0 := 14 + cryptoRandInt(capH-28)
		y1 := 14 + cryptoRandInt(capH-28)
		amp := float64(cryptoRandInt(12)) - 6
		p := float64(cryptoRandInt(628)) / 100.0
		for x := 8; x < capW-8; x++ {
			t := float64(x) / float64(capW)
			y := int(math.Round((1-t)*float64(y0) + t*float64(y1) + amp*math.Sin(t*math.Pi*2+p)))
			drawDisc(bg, x, y, 1, c)
		}
	}

	// 微量散点噪点
	for i := 0; i < 65; i++ {
		x := cryptoRandInt(capW)
		y := cryptoRandInt(capH)
		c := inkPalette[cryptoRandInt(len(inkPalette))]
		c.A = 110
		drawDisc(bg, x, y, 1, c)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, bg); err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func drawGlyph(img *image.RGBA, rows [7]string, ox, oy, scale int, shear float64, col color.RGBA) {
	halfH := float64(7*scale) / 2.0
	for r := 0; r < 7; r++ {
		row := rows[r]
		for c := 0; c < 5 && c < len(row); c++ {
			if row[c] != '#' {
				continue
			}
			for dy := 0; dy < scale; dy++ {
				py := oy + r*scale + dy
				yCentered := float64(r*scale+dy) - halfH
				shift := int(math.Round(yCentered * shear))
				for dx := 0; dx < scale; dx++ {
					px := ox + c*scale + dx + shift
					if px >= 0 && px < capW && py >= 0 && py < capH {
						img.SetRGBA(px, py, col)
					}
				}
			}
		}
	}
}

func drawDisc(img *image.RGBA, cx, cy, r int, col color.RGBA) {
	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			if dx*dx+dy*dy <= r*r {
				px, py := cx+dx, cy+dy
				if px >= 0 && px < capW && py >= 0 && py < capH {
					blendPixel(img, px, py, col)
				}
			}
		}
	}
}

func blendPixel(img *image.RGBA, x, y int, src color.RGBA) {
	if src.A == 255 {
		img.SetRGBA(x, y, src)
		return
	}
	dst := img.RGBAAt(x, y)
	a := uint16(src.A)
	inv := 255 - a
	img.SetRGBA(x, y, color.RGBA{
		R: uint8((uint16(src.R)*a + uint16(dst.R)*inv) / 255),
		G: uint8((uint16(src.G)*a + uint16(dst.G)*inv) / 255),
		B: uint8((uint16(src.B)*a + uint16(dst.B)*inv) / 255),
		A: 255,
	})
}

// 5x7 清晰点阵字模（仅含 captchaAlphabet 29 个无歧义字符）。
var captchaGlyphs = map[byte][7]string{
	'2': {".###.", "#...#", "....#", "..##.", ".#...", "#....", "#####"},
	'3': {"####.", "....#", "....#", ".###.", "....#", "....#", "####."},
	'4': {"#...#", "#...#", "#...#", "#####", "....#", "....#", "....#"},
	'6': {".###.", "#....", "#....", "####.", "#...#", "#...#", ".###."},
	'7': {"#####", "....#", "...#.", "..#..", ".#...", ".#...", ".#..."},
	'8': {".###.", "#...#", "#...#", ".###.", "#...#", "#...#", ".###."},
	'9': {".###.", "#...#", "#...#", ".####", "....#", "...#.", ".##.."},
	'A': {"..#..", ".#.#.", "#...#", "#####", "#...#", "#...#", "#...#"},
	'B': {"####.", "#...#", "#...#", "####.", "#...#", "#...#", "####."},
	'C': {".###.", "#...#", "#....", "#....", "#....", "#...#", ".###."},
	'D': {"####.", "#...#", "#...#", "#...#", "#...#", "#...#", "####."},
	'E': {"#####", "#....", "#....", "####.", "#....", "#....", "#####"},
	'F': {"#####", "#....", "#....", "####.", "#....", "#....", "#...."},
	'G': {".###.", "#...#", "#....", "#.###", "#...#", "#...#", ".###."},
	'H': {"#...#", "#...#", "#...#", "#####", "#...#", "#...#", "#...#"},
	'J': {"..###", "...#.", "...#.", "...#.", "#..#.", "#..#.", ".##.."},
	'K': {"#...#", "#..#.", "#.#..", "##...", "#.#..", "#..#.", "#...#"},
	'L': {"#....", "#....", "#....", "#....", "#....", "#....", "#####"},
	'M': {"#...#", "##.##", "#.#.#", "#...#", "#...#", "#...#", "#...#"},
	'N': {"#...#", "##..#", "#.#.#", "#..##", "#...#", "#...#", "#...#"},
	'P': {"####.", "#...#", "#...#", "####.", "#....", "#....", "#...."},
	'Q': {".###.", "#...#", "#...#", "#...#", "#.#.#", "#..#.", ".##.#"},
	'R': {"####.", "#...#", "#...#", "####.", "#.#..", "#..#.", "#...#"},
	'T': {"#####", "..#..", "..#..", "..#..", "..#..", "..#..", "..#.."},
	'U': {"#...#", "#...#", "#...#", "#...#", "#...#", "#...#", ".###."},
	'V': {"#...#", "#...#", "#...#", "#...#", ".#.#.", ".#.#.", "..#.."},
	'W': {"#...#", "#...#", "#...#", "#.#.#", "#.#.#", "##.##", "#...#"},
	'X': {"#...#", "#...#", ".#.#.", "..#..", ".#.#.", "#...#", "#...#"},
	'Y': {"#...#", "#...#", ".#.#.", "..#..", "..#..", "..#..", "..#.."},
}
