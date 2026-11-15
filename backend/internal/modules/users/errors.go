package users

import "github.com/yanji-community/backend/internal/shared/apperr"

var (
	errInvalidBody      = apperr.BadRequest("VALIDATION_ERROR", "请求体格式不正确")
	errFollowersPrivate = apperr.Forbidden("该用户已隐藏关注/粉丝列表")
)
