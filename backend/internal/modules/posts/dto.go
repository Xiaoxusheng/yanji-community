// Package posts 帖子：状态机、草稿、发布审核、修订、Feed 与热度。
package posts

import (
	"context"
	"time"

	"github.com/yanji-community/backend/internal/modules/polls"
)

// 帖子状态。
const (
	StatusDraft         = "draft"
	StatusPendingReview = "pending_review"
	StatusPublished     = "published"
	StatusRejected      = "rejected"
	StatusHidden        = "hidden"
	StatusLocked        = "locked"
	StatusDeleted       = "deleted"
)

// 帖子类型。
const (
	TypeDiscussion = "discussion"
	TypeArticle    = "article"
	TypeQuestion   = "question"
	TypePoll       = "poll"
)

// Author 作者摘要。
type Author struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	AvatarKey   string `json:"avatar_key"`
}

// CategoryRef 版块摘要。
type CategoryRef struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// TagRef 标签摘要。
type TagRef struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// PostItem 列表项。
type PostItem struct {
	ID             string      `json:"id"`
	Title          string      `json:"title"`
	Excerpt        string      `json:"excerpt"`
	PostType       string      `json:"post_type"`
	Status         string      `json:"status"`
	Author         Author      `json:"author"`
	Category       CategoryRef `json:"category"`
	Tags           []TagRef    `json:"tags"`
	IsPinned       bool        `json:"is_pinned"`
	IsFeatured     bool        `json:"is_featured"`
	ViewCount      int64       `json:"view_count"`
	LikeCount      int64       `json:"like_count"`
	CommentCount   int64       `json:"comment_count"`
	BookmarkCount  int64       `json:"bookmark_count"`
	IsLiked        bool        `json:"is_liked"`
	IsBookmarked   bool        `json:"is_bookmarked"`
	PublishedAt    *time.Time  `json:"published_at"`
	LastActivityAt *time.Time  `json:"last_activity_at"`
	CreatedAt      time.Time   `json:"created_at"`
}

// PostDetail 详情。
type PostDetail struct {
	PostItem
	BodyRendered string     `json:"body_rendered"`
	RejectReason string     `json:"reject_reason,omitempty"`
	IsAuthor     bool       `json:"is_author"`
	CanModerate  bool       `json:"can_moderate"`
	EditedAt     *time.Time `json:"edited_at"`
}

// Revision 修订记录。
type Revision struct {
	ID           int64     `json:"id"`
	Title        string    `json:"title"`
	BodyMarkdown string    `json:"body_markdown"`
	EditReason   string    `json:"edit_reason"`
	EditorName   string    `json:"editor_name"`
	CreatedAt    time.Time `json:"created_at"`
}

// FollowsProvider 关注数据接口（由 follows 模块实现）。
type FollowsProvider interface {
	FollowedUserIDs(ctx context.Context, userID int64) ([]int64, error)
	FollowedCategoryIDs(ctx context.Context, userID int64) ([]int64, error)
	FollowedTagIDs(ctx context.Context, userID int64) ([]int64, error)
	BlockedIDs(ctx context.Context, userID int64) ([]int64, error)
}

// UsersProvider 用户查询接口（由 users 模块实现）。
type UsersProvider interface {
	InternalIDByUsername(ctx context.Context, username string) (int64, error)
}

// CreateRequest 创建/保存草稿请求。
type CreateRequest struct {
	Title        string        `json:"title"`
	BodyMarkdown string        `json:"body_markdown"`
	CategorySlug string        `json:"category_slug"`
	PostType     string        `json:"post_type"`
	TagSlugs     []string      `json:"tag_slugs"`
	Excerpt      string        `json:"excerpt"`
	IsDraft      bool          `json:"is_draft"`
	Poll         *polls.Config `json:"poll,omitempty"`
}

// UpdateRequest 编辑请求。
type UpdateRequest struct {
	Title        *string   `json:"title"`
	BodyMarkdown *string   `json:"body_markdown"`
	TagSlugs     *[]string `json:"tag_slugs"`
	EditReason   string    `json:"edit_reason"`
}

// ModerateRequest 后台管理动作。
type ModerateRequest struct {
	Action       string `json:"action"` // approve|reject|hide|restore|lock|unlock|pin|unpin|feature|unfeature|move_category
	Reason       string `json:"reason"`
	CategorySlug string `json:"category_slug"`
}
