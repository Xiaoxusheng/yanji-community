package users

import (
	"context"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yanji-community/backend/internal/shared/apperr"
)

// Service 用户业务逻辑。
type Service struct {
	repo   *Repo
	logger *slog.Logger
}

// New 构造。
func New(pool *pgxpool.Pool, logger *slog.Logger) *Service {
	return &Service{repo: NewRepo(pool), logger: logger}
}

// UpdateProfileRequest 资料更新（指针字段：nil 表示不修改）。
type UpdateProfileRequest struct {
	DisplayName *string `json:"display_name"`
	AvatarKey   *string `json:"avatar_key"`
	Bio         *string `json:"bio"`
	Website     *string `json:"website"`
	Location    *string `json:"location"`
}

// UpdateSettingsRequest 隐私/通知设置更新。
type UpdateSettingsRequest struct {
	ProfileVisibility *string `json:"profile_visibility"`
	ShowFollowers     *bool   `json:"show_followers"`
	NotifyEmail       *bool   `json:"notify_email"`
	NotifyInapp       *bool   `json:"notify_inapp"`
}

func invalid(msg string) error { return apperr.BadRequest("VALIDATION_ERROR", msg) }

// PublicProfile 公开资料。
func (s *Service) PublicProfile(ctx context.Context, username string) (*PublicProfile, error) {
	return s.repo.GetPublicProfile(ctx, username)
}

// InternalIDByUsername 内部 ID。
func (s *Service) InternalIDByUsername(ctx context.Context, username string) (int64, error) {
	return s.repo.InternalIDByUsername(ctx, username)
}

// MyProfile 本人资料。
func (s *Service) MyProfile(ctx context.Context, userID int64) (*ProfileSetting, error) {
	return s.repo.GetMyProfile(ctx, userID)
}

// UpdateProfile 更新本人资料。
func (s *Service) UpdateProfile(ctx context.Context, userID int64, req UpdateProfileRequest) error {
	fields := map[string]any{}
	if req.DisplayName != nil {
		dn := strings.TrimSpace(*req.DisplayName)
		if len([]rune(dn)) > 30 {
			return invalid("昵称最长 30 字")
		}
		fields["display_name"] = dn
	}
	if req.AvatarKey != nil {
		fields["avatar_key"] = strings.TrimSpace(*req.AvatarKey)
	}
	if req.Bio != nil {
		if len([]rune(*req.Bio)) > 500 {
			return invalid("简介最长 500 字")
		}
		fields["bio"] = *req.Bio
	}
	if req.Website != nil {
		w := strings.TrimSpace(*req.Website)
		if w != "" && !strings.HasPrefix(w, "http://") && !strings.HasPrefix(w, "https://") {
			return invalid("个人主页链接需以 http(s):// 开头")
		}
		fields["website"] = w
	}
	if req.Location != nil {
		if len([]rune(*req.Location)) > 50 {
			return invalid("所在地最长 50 字")
		}
		fields["location"] = *req.Location
	}
	return s.repo.UpdateProfile(ctx, userID, fields)
}

// UpdateSettings 更新隐私/通知设置。
func (s *Service) UpdateSettings(ctx context.Context, userID int64, req UpdateSettingsRequest) error {
	fields := map[string]any{}
	if req.ProfileVisibility != nil {
		switch *req.ProfileVisibility {
		case "public", "followers", "private":
			fields["profile_visibility"] = *req.ProfileVisibility
		default:
			return invalid("可见性取值不合法")
		}
	}
	if req.ShowFollowers != nil {
		fields["show_followers"] = *req.ShowFollowers
	}
	if req.NotifyEmail != nil {
		fields["notify_email"] = *req.NotifyEmail
	}
	if req.NotifyInapp != nil {
		fields["notify_inapp"] = *req.NotifyInapp
	}
	return s.repo.UpdateProfile(ctx, userID, fields)
}

// Followers 粉丝列表。
func (s *Service) Followers(ctx context.Context, userID int64, limit, offset int) ([]FollowerRow, int64, error) {
	return s.repo.ListFollowers(ctx, userID, limit, offset)
}

// Following 关注列表。
func (s *Service) Following(ctx context.Context, userID int64, limit, offset int) ([]FollowerRow, int64, error) {
	return s.repo.ListFollowing(ctx, userID, limit, offset)
}
