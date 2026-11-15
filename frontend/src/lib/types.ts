// 后端 API 领域类型（与 /api/v1 契约对应）。

export interface Author {
  id: string;
  username: string;
  display_name: string;
  avatar_key: string;
}

export interface CategoryRef {
  id: number;
  name: string;
  slug: string;
}

export interface TagRef {
  id: number;
  name: string;
  slug: string;
}

export interface PostItem {
  id: string;
  title: string;
  excerpt: string;
  post_type: string;
  status: string;
  author: Author;
  category: CategoryRef;
  tags: TagRef[];
  is_pinned: boolean;
  is_featured: boolean;
  view_count: number;
  like_count: number;
  comment_count: number;
  bookmark_count: number;
  is_liked: boolean;
  is_bookmarked: boolean;
  published_at: string | null;
  last_activity_at: string | null;
  created_at: string;
}

export interface PostDetail extends PostItem {
  body_rendered: string;
  reject_reason?: string;
  is_author: boolean;
  can_moderate: boolean;
  edited_at: string | null;
}

export interface CommentNode {
  id: string;
  is_accepted_answer?: boolean;
  post_id?: string;
  parent_id?: string;
  quote_id?: string;
  quote_author?: string;
  quote_body?: string;
  depth: number;
  author: Author;
  body_rendered: string;
  status: string;
  like_count: number;
  reply_count: number;
  is_liked: boolean;
  is_author: boolean;
  edited_at: string | null;
  created_at: string;
  replies?: CommentNode[];
}

export interface Category {
  id: number;
  parent_id: number | null;
  name: string;
  slug: string;
  description: string;
  icon: string;
  sort_order: number;
  status: string;
  visibility: string;
  posting_policy: string;
  review_policy: string;
  post_count: number;
}

export interface Tag {
  id: number;
  name: string;
  slug: string;
  description: string;
  status: string;
  post_count: number;
}

export interface Me {
  authenticated: boolean;
  user: {
    id: string;
    username: string;
    email: string;
    email_verified: boolean;
    status: string;
    sanctioned: boolean;
    sanction_type: string;
  } | null;
}

export interface NotificationItem {
  id: number;
  type: string;
  resource_type: string;
  resource_id: number;
  resource_public_id: string;
  title: string;
  body: string;
  actor_name: string;
  actor_avatar: string;
  read_at: string | null;
  created_at: string;
}

export interface Announcement {
  id: number;
  title: string;
  body: string;
  published_at: string | null;
}

export interface SessionRow {
  id: number;
  device_name: string;
  ip_address: string;
  created_at: string;
  last_seen_at: string;
  expires_at: string;
  current: boolean;
}

export interface ProfilePublic {
  id: string;
  username: string;
  display_name: string;
  avatar_key: string;
  bio: string;
  website: string;
  location: string;
  created_at: string;
  post_count: number;
  comment_count: number;
  follower_count: number;
  following_count: number;
  show_followers: boolean;
}

export interface SearchPostResult {
  id: string;
  title: string;
  highlight: string;
  author_name: string;
  category_slug: string;
  category_name: string;
  like_count: number;
  comment_count: number;
  published_at: string;
}

export interface CommentItem {
  id: number;
  public_id: string;
  author: string;
  post_title: string;
  post_id: string;
  body: string;
  status: string;
  like_count: number;
  created_at: string;
}

export interface ReportRow {
  id: number;
  reporter: string;
  target_type: string;
  target_label: string;
  reason_code: string;
  description: string;
  status: string;
  assignee: string;
  resolution_note: string;
  resolved_at: string | null;
  created_at: string;
}

export interface UserRow {
  id: number;
  public_id: string;
  username: string;
  email: string;
  status: string;
  email_verified: boolean;
  display_name: string;
  post_count: number;
  created_at: string;
  last_login_at: string | null;
}

export interface SanctionRow {
  id: number;
  username?: string;
  sanction_type: string;
  reason: string;
  starts_at: string;
  ends_at: string | null;
  status: string;
  operator?: string;
  created_at: string;
  lifted_at: string | null;
}

export interface RoleRow {
  id: number;
  code: string;
  name: string;
  description: string;
  is_system: boolean;
  permissions: string[];
}

export interface OpLogRow {
  id: number;
  actor: string;
  action: string;
  target_type: string;
  target_id: number;
  request_id: string;
  note: string;
  ip: string;
  created_at: string;
}

export interface DashboardData {
  totals: {
    users: number;
    users_today: number;
    posts: number;
    posts_today: number;
    comments: number;
    comments_today: number;
    online_sessions: number;
  };
  queues: {
    pending_posts: number;
    pending_reports: number;
    active_sanctions: number;
  };
  trends: { day: string; posts: number; users: number; comments: number }[];
}

export interface BookmarkItem {
  post_id: string;
  title: string;
  excerpt: string;
  folder_id: number;
  folder_name: string;
  author_name: string;
  created_at: string;
}

export interface FolderRow {
  id: number;
  name: string;
  sort_order: number;
  count: number;
}

// ---------- 阶段 2 ----------

export interface LevelInfo {
  level: number;
  name: string;
  min_points: number;
  color: string;
}

export interface PointAccount {
  balance: number;
  total_earned: number;
  level: LevelInfo;
  next_level?: LevelInfo | null;
  signin_days: number;
  streak_days: number;
  today_signed: boolean;
}

export interface PointTransaction {
  id: number;
  amount: number;
  balance_after: number;
  source_type: string;
  note: string;
  created_at: string;
}

export interface Badge {
  id: number;
  code: string;
  name: string;
  description: string;
  icon: string;
  condition_type: "manual" | "auto";
  is_enabled: boolean;
  holder_count?: number;
  created_at: string;
}

export interface PollOption {
  id: number;
  content: string;
  votes: number;
  percent: number;
}

export interface Poll {
  id: number;
  post_id: number;
  multiple_choice: boolean;
  max_choices: number;
  allow_revote: boolean;
  ends_at: string | null;
  expired: boolean;
  total_voters: number;
  my_votes: number[];
  options: PollOption[];
}

export interface Activity {
  id: number;
  title: string;
  description: string;
  cover_key: string;
  location: string;
  starts_at: string;
  ends_at: string | null;
  signup_starts_at: string | null;
  signup_ends_at: string | null;
  capacity: number;
  participant_count: number;
  status: string;
  reward_points: number;
  created_by_name: string;
  is_joined: boolean;
  created_at: string;
}

export interface Sanction {
  id: number;
  sanction_type: string;
  reason: string;
  starts_at: string;
  ends_at: string | null;
  status: string;
  created_at: string;
  lifted_at: string | null;
}

export interface Appeal {
  id: number;
  user_id: number;
  username: string;
  target_type: string;
  target_id: number;
  target_label: string;
  reason: string;
  status: string;
  handled_by: string;
  handle_note: string;
  handled_at: string | null;
  created_at: string;
}
