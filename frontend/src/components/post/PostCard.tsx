"use client";

// 帖子卡片与列表（标题优先、元信息紧凑、统一交互行）。
import Link from "next/link";
import { timeAgo, fmtCount } from "@/lib/api";
import type { PostItem } from "@/lib/types";
import { Avatar } from "@/components/ui/Avatar";
import { ListSkeleton, EmptyState, ErrorState, Pagination } from "@/components/ui/States";

const statusTags: Record<string, { text: string; cls: string }> = {
  draft: { text: "草稿", cls: "" },
  pending_review: { text: "待审核", cls: "tag-warning" },
  rejected: { text: "未通过", cls: "tag-danger" },
  hidden: { text: "已隐藏", cls: "tag-danger" },
  locked: { text: "已锁定", cls: "tag-info" },
  deleted: { text: "已删除", cls: "" },
};

export function PostCard({ post, showStatus }: { post: PostItem; showStatus?: boolean }) {
  const st = showStatus ? statusTags[post.status] : undefined;
  return (
    <article className="card" style={{ padding: 16, display: "flex", flexDirection: "column", gap: 8 }}>
      <div style={{ display: "flex", gap: 6, alignItems: "center", flexWrap: "wrap" }}>
        {post.is_pinned && <span className="tag tag-brand">置顶</span>}
        {post.is_featured && <span className="tag tag-warning">精华</span>}
        {st && <span className={`tag ${st.cls}`}>{st.text}</span>}
        <Link href={`/c/${post.category.slug}`} className="tag tag-info">
          {post.category.name}
        </Link>
      </div>
      <Link href={`/p/${post.id}`} style={{ textDecoration: "none" }}>
        <h2 style={{ fontSize: 17, fontWeight: 600, lineHeight: 1.45 }}>{post.title}</h2>
      </Link>
      {post.excerpt && (
        <p
          style={{
            color: "var(--text-2)",
            fontSize: 14,
            display: "-webkit-box",
            WebkitLineClamp: 2,
            WebkitBoxOrient: "vertical",
            overflow: "hidden",
            margin: 0,
          }}
        >
          {post.excerpt}
        </p>
      )}
      {post.tags.length > 0 && (
        <div style={{ display: "flex", gap: 6, flexWrap: "wrap" }}>
          {post.tags.map((t) => (
            <Link key={t.id} href={`/t/${t.slug}`} className="tag">
              # {t.name}
            </Link>
          ))}
        </div>
      )}
      <div style={{ display: "flex", alignItems: "center", gap: 8, color: "var(--text-3)", fontSize: 13, marginTop: 2 }}>
        <Avatar name={post.author.display_name} avatarKey={post.author.avatar_key} size={24} link={`/u/${post.author.username}`} />
        <Link href={`/u/${post.author.username}`}>{post.author.display_name}</Link>
        <span>·</span>
        <span>{timeAgo(post.published_at || post.created_at)}</span>
        <span style={{ marginLeft: "auto", display: "flex", gap: 12 }}>
          <span title="浏览">👁 {fmtCount(post.view_count)}</span>
          <span title="评论">💬 {fmtCount(post.comment_count)}</span>
          <span title="点赞">👍 {fmtCount(post.like_count)}</span>
        </span>
      </div>
    </article>
  );
}

export function PostList({
  posts,
  loading,
  error,
  onRetry,
  page,
  pageSize,
  total,
  onPageChange,
  emptyTitle = "还没有内容",
  emptyDesc = "成为第一个发帖的人吧",
  showStatus,
}: {
  posts: PostItem[] | undefined;
  loading: boolean;
  error: Error | null;
  onRetry?: () => void;
  page?: number;
  pageSize?: number;
  total?: number;
  onPageChange?: (p: number) => void;
  emptyTitle?: string;
  emptyDesc?: string;
  showStatus?: boolean;
}) {
  if (loading) return <ListSkeleton />;
  if (error) return <ErrorState message={(error as Error).message} onRetry={onRetry} />;
  if (!posts || posts.length === 0)
    return (
      <EmptyState
        title={emptyTitle}
        description={emptyDesc}
        action={
          <Link href="/compose" className="btn btn-primary btn-sm">
            去发帖
          </Link>
        }
      />
    );
  return (
    <>
      <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
        {posts.map((p) => (
          <PostCard key={p.id} post={p} showStatus={showStatus} />
        ))}
      </div>
      {page !== undefined && pageSize !== undefined && total !== undefined && onPageChange && (
        <Pagination page={page} pageSize={pageSize} total={total} onChange={onPageChange} />
      )}
    </>
  );
}
