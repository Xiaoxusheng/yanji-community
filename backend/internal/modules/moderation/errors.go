package moderation

import "github.com/yanji-community/backend/internal/shared/apperr"

func errInvalidImpl(msg string) error { return apperr.BadRequest("VALIDATION_ERROR", msg) }
