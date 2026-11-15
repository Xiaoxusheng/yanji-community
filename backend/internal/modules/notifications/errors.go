package notifications

import "github.com/yanji-community/backend/internal/shared/apperr"

var (
	errInvalidID   = apperr.BadRequest("VALIDATION_ERROR", "ID 不合法")
	errInvalidBody = apperr.BadRequest("VALIDATION_ERROR", "请求体格式不正确")
)
