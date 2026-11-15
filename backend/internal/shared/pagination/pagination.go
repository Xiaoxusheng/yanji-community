// Package pagination 统一解析 page/page_size 分页参数。
package pagination

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/yanji-community/backend/internal/shared/apperr"
)

const (
	DefaultPage     = 1
	DefaultPageSize = 20
	MaxPageSize     = 100
)

// Query 携带解析后的分页参数与偏移量。
type Query struct {
	Page     int
	PageSize int
	Offset   int
}

// Parse 从请求解析分页参数：page >= 1，page_size ∈ [1,100]。
func Parse(c *gin.Context) (Query, error) {
	page, err := atoiDefault(c.Query("page"), DefaultPage)
	if err != nil || page < 1 {
		return Query{}, apperr.BadRequest("VALIDATION_ERROR", "page 参数不合法")
	}
	size, err := atoiDefault(c.Query("page_size"), DefaultPageSize)
	if err != nil || size < 1 || size > MaxPageSize {
		return Query{}, apperr.BadRequest("VALIDATION_ERROR", "page_size 参数不合法（1-100）")
	}
	return Query{Page: page, PageSize: size, Offset: (page - 1) * size}, nil
}

func atoiDefault(s string, def int) (int, error) {
	if s == "" {
		return def, nil
	}
	return strconv.Atoi(s)
}
