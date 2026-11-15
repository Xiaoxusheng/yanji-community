// Package config 负责加载并校验运行时配置。
// 所有敏感凭据一律通过环境变量注入，严禁写入代码仓库。
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config 汇总应用全部配置项。
type Config struct {
	Env             string // development | production
	Port            string
	PublicURL       string
	RequestIDHeader string

	DatabaseURL      string
	DatabaseMaxConns int32
	MigrateOnStart   bool

	RedisAddr     string
	RedisPassword string
	RedisDB       int

	SessionTTL        time.Duration
	CookieDomain      string
	CookieSecure      bool
	SeedAdminEmail    string
	SeedAdminPassword string

	StorageDriver     string // local | s3
	StorageLocalDir   string
	StoragePublicBase string
	S3Endpoint        string
	S3Region          string
	S3Bucket          string
	S3AccessKey       string
	S3SecretKey       string
	UploadMaxBytes    int64

	MailDriver   string // log | smtp
	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	SMTPFrom     string

	RateLimitAuth  int
	RateLimitWrite int
	RateLimitRead  int

	CORSAllowOrigins []string
}

// Load 从环境变量读取配置；未设置时使用安全默认值。
func Load() (*Config, error) {
	cfg := &Config{
		Env:             getEnv("APP_ENV", "development"),
		Port:            getEnv("APP_PORT", "8080"),
		PublicURL:       getEnv("APP_PUBLIC_URL", "http://localhost:3000"),
		RequestIDHeader: getEnv("REQUEST_ID_HEADER", "X-Request-Id"),

		DatabaseURL:      os.Getenv("DATABASE_URL"),
		DatabaseMaxConns: int32(getEnvInt("DATABASE_MAX_CONNS", 20)),
		MigrateOnStart:   getEnvBool("MIGRATE_ON_START", true),

		RedisAddr:     getEnv("REDIS_ADDR", "127.0.0.1:6379"),
		RedisPassword: os.Getenv("REDIS_PASSWORD"),
		RedisDB:       getEnvInt("REDIS_DB", 0),

		SessionTTL:        time.Duration(getEnvInt("SESSION_TTL_HOURS", 24*7)) * time.Hour,
		CookieDomain:      os.Getenv("COOKIE_DOMAIN"),
		CookieSecure:      getEnvBool("COOKIE_SECURE", false),
		SeedAdminEmail:    os.Getenv("SEED_ADMIN_EMAIL"),
		SeedAdminPassword: os.Getenv("SEED_ADMIN_PASSWORD"),

		StorageDriver:     getEnv("STORAGE_DRIVER", "local"),
		StorageLocalDir:   getEnv("STORAGE_LOCAL_DIR", "./data/uploads"),
		StoragePublicBase: getEnv("STORAGE_PUBLIC_BASE", "/files"),
		S3Endpoint:        os.Getenv("S3_ENDPOINT"),
		S3Region:          os.Getenv("S3_REGION"),
		S3Bucket:          os.Getenv("S3_BUCKET"),
		S3AccessKey:       os.Getenv("S3_ACCESS_KEY"),
		S3SecretKey:       os.Getenv("S3_SECRET_KEY"),
		UploadMaxBytes:    int64(getEnvInt("UPLOAD_MAX_BYTES", 10*1024*1024)),

		MailDriver:   getEnv("MAIL_DRIVER", "log"),
		SMTPHost:     os.Getenv("SMTP_HOST"),
		SMTPPort:     getEnvInt("SMTP_PORT", 465),
		SMTPUsername: os.Getenv("SMTP_USERNAME"),
		SMTPPassword: os.Getenv("SMTP_PASSWORD"),
		SMTPFrom:     getEnv("SMTP_FROM", "no-reply@example.com"),

		RateLimitAuth:  getEnvInt("RATE_LIMIT_AUTH", 20),
		RateLimitWrite: getEnvInt("RATE_LIMIT_WRITE", 60),
		RateLimitRead:  getEnvInt("RATE_LIMIT_READ", 300),
	}

	if cfg.DatabaseURL == "" {
		// 允许用分散变量拼装，便于只设置 POSTGRES_* 的场景。
		host := getEnv("POSTGRES_HOST", "127.0.0.1")
		port := getEnv("POSTGRES_PORT", "5432")
		user := getEnv("POSTGRES_USER", "yanji")
		pass := os.Getenv("POSTGRES_PASSWORD")
		db := getEnv("POSTGRES_DB", "yanji_community")
		cfg.DatabaseURL = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", user, pass, host, port, db)
	}

	cfg.PublicURL = strings.TrimRight(cfg.PublicURL, "/")
	cfg.CORSAllowOrigins = []string{cfg.PublicURL}
	if cfg.Env == "development" {
		cfg.CORSAllowOrigins = append(cfg.CORSAllowOrigins, "http://localhost:3000", "http://127.0.0.1:3000")
	}

	if cfg.Env == "production" {
		cfg.CookieSecure = true // 生产环境强制 Secure Cookie
	}
	if cfg.SeedAdminEmail == "" {
		cfg.SeedAdminEmail = "admin@example.com"
	}
	if cfg.SeedAdminPassword == "" {
		cfg.SeedAdminPassword = "Admin@2024"
	}
	return cfg, nil
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); strings.TrimSpace(v) != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return n
		}
	}
	return def
}

func getEnvBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "1", "true", "yes", "on":
			return true
		case "0", "false", "no", "off":
			return false
		}
	}
	return def
}
