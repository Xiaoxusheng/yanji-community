// 迁移文件完整性检查（纯静态分析，无数据库依赖）：
// 1. 每个版本号 up/down 成对；2. 无重复版本号；3. 外键引用的表必须先于当前文件创建（或自引用）。
package migrations

import (
	"regexp"
	"sort"
	"strings"
	"testing"
)

func TestMigrationPairs(t *testing.T) {
	entries, err := FS.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	ups := map[string]string{}
	downs := map[string]string{}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".sql") {
			continue
		}
		version := strings.SplitN(name, "_", 2)[0]
		switch {
		case strings.HasSuffix(name, ".up.sql"):
			if _, dup := ups[version]; dup {
				t.Fatalf("重复的迁移版本: %s", version)
			}
			ups[version] = name
		case strings.HasSuffix(name, ".down.sql"):
			if _, dup := downs[version]; dup {
				t.Fatalf("重复的回滚版本: %s", version)
			}
			downs[version] = name
		}
	}
	if len(ups) == 0 {
		t.Fatal("未找到迁移文件")
	}
	for v, up := range ups {
		if _, ok := downs[v]; !ok {
			t.Fatalf("迁移 %s(%s) 缺少对应的 down 文件", v, up)
		}
	}
	for v, down := range downs {
		if _, ok := ups[v]; !ok {
			t.Fatalf("回滚 %s(%s) 缺少对应的 up 文件", v, down)
		}
	}
	t.Logf("共 %d 组迁移，成对完整", len(ups))
}

func TestForeignKeyOrder(t *testing.T) {
	entries, err := FS.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var ups []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".up.sql") {
			ups = append(ups, e.Name())
		}
	}
	sort.Strings(ups)

	createRe := regexp.MustCompile(`(?i)CREATE TABLE (?:IF NOT EXISTS )?(\w+)`)
	refRe := regexp.MustCompile(`(?i)REFERENCES (\w+)\s*\(`)

	created := map[string]bool{}
	for _, name := range ups {
		content, err := FS.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		thisTables := map[string]bool{}
		for _, m := range createRe.FindAllStringSubmatch(string(content), -1) {
			thisTables[m[1]] = true
		}
		for _, m := range refRe.FindAllStringSubmatch(string(content), -1) {
			ref := strings.ToLower(m[1])
			if !created[ref] && !thisTables[ref] {
				t.Fatalf("%s 引用了尚未创建的表 %s（迁移顺序错误）", name, ref)
			}
		}
		for tbl := range thisTables {
			created[tbl] = true
		}
	}
}

func TestRequiredTablesExist(t *testing.T) {
	entries, err := FS.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var all strings.Builder
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".up.sql") {
			content, _ := FS.ReadFile(e.Name())
			all.Write(content)
		}
	}
	required := []string{
		"users", "user_profiles", "user_sessions", "email_verification_tokens",
		"password_reset_tokens", "roles", "permissions", "user_roles", "role_permissions",
		"categories", "category_moderators", "category_follows", "tags", "tag_follows",
		"posts", "post_revisions", "post_tags", "post_attachments", "comments",
		"post_likes", "comment_likes", "bookmarks", "bookmark_folders", "user_follows",
		"user_blocks", "notifications", "reports", "moderation_actions", "user_sanctions",
		"sensitive_words", "announcements", "featured_items", "operation_logs",
		"login_logs", "system_configs", "outbox_events",
	}
	for _, tbl := range required {
		if !strings.Contains(all.String(), "CREATE TABLE "+tbl) &&
			!strings.Contains(all.String(), "CREATE TABLE IF NOT EXISTS "+tbl) {
			t.Errorf("缺少必需表: %s", tbl)
		}
	}
}
