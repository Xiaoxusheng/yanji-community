// Package migrations 内嵌 SQL 迁移文件，供 golang-migrate 在启动时执行。
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
