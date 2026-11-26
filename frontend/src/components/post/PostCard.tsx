"use client";

// 帖子卡片与列表（精致编辑感排版、微层次悬浮与数值对齐）。
import Link from "next/link";
import { timeAgo, fmtCount } from "@/lib/api";
import type { PostItem } from "@/lib/types";
import { Avatar } from "@/components/ui/Avatar";
import { ListSkeleton, EmptyState, ErrorState, Pagination } from "@/components/ui/States";
import {
  IconAward,
  IconBarChart,
  IconEye,
  IconFileText,
  IconHelpCircle,
  IconLock,
  IconMessage,
  IconPin,
  IconThumbUp,
} from "@/components/ui/Icons";

const statusTags: Record<string, { text: string; cls: string }> = {
  draft: { text: "草稿", cls: "" },
  pending_review: { text: "待审核", cls: "tag-warning" },
  rejected: { text: "未通过", cls: "tag-danger" },
  hidden: { text: "已隐藏", cls: "tag-danger" },
  locked: { text: "已锁定", cls: "tag-info" },
  deleted: { text: "已删除", cls: "" },
};

export function PostTypeBadge({ type }: { type: string }) {
  if (type === "article") {
    return (
      <span className="tag tag-outline">
        <IconFileText size={11} /> 长文章
      </span>
    );
  }
  if (type === "question") {
    return (
      <span className="tag tag-info">
        <IconHelpCircle size={11} /> 问答
      </span>
    );
  }
  if (type === "poll") {
    return (
      <span className="tag tag-outline">
        <IconBarChart size={11} /> 投票
      </span>
    );
  }
  return null;
}

export function PostCard({ post, showStatus }: { post: PostItem; showStatus?: boolean }) {
  const st = showStatus ? statusTags[post.status] : undefined;
  return (
    <article
      className={`card card-interactive ${post.is_pinned ? "post-card-pinned" : ""}`}
      style={{
        padding: "18px 20px",
        display: "flex",
        flexDirection: "column",
        gap: 10,
      }}
    >
      {/* 顶部作者与属性行 */}
      <div style={{ display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap", fontSize: 12.5, color: "var(--text-3)" }}>
        <Avatar name={post.author.display_name} avatarKey={post.author.avatar_key} size={24} link={`/u/${post.author.username}`} />
        <Link href={`/u/${post.author.username}`} style={{ fontWeight: 600, color: "var(--text-2)" }}>
          {post.author.display_name}
        </Link>
        <span style={{ opacity: 0.55 }}>·</span>
        <span className="tabular-nums">{timeAgo(post.published_at || post.created_at)}</span>

        <div style={{ marginLeft: "auto", display: "flex", gap: 5, alignItems: "center", flexWrap: "wrap" }}>
          {post.is_pinned && (
            <span className="tag tag-brand">
              <IconPin size={11} /> 置顶
            </span>
          )}
          {post.is_featured && (
            <span className="tag tag-warning">
              <IconAward size={11} /> 精华
            </span>
          )}
          {post.status === "locked" && !showStatus && (
            <span className="tag">
              <IconLock size={11} /> 已锁定
            </span>
          )}
          <PostTypeBadge type={post.post_type} />
          {st && <span className={`tag ${st.cls}`}>{st.text}</span>}
          <Link href={`/c/${post.category.slug}`} className="tag tag-outline" style={{ color: "var(--text-2)" }}>
            {post.category.name}
          </Link>
        </div>
      </div>

      {/* 标题 */}
      <Link href={`/p/${post.id}`} style={{ textDecoration: "none" }}>
        <h2 className="post-card-title">{post.title}</h2>
      </Link>

      {/* 摘要 */}
      {post.excerpt && (
        <p
          style={{
            color: "var(--text-2)",
            fontSize: 13.75,
            lineHeight: 1.65,
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

      {/* 底部标签与数据指标 */}
      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: 8,
          flexWrap: "wrap",
          paddingTop: 6,
          marginTop: 2,
          borderTop: "1px solid var(--border)",
        }}
      >
        {post.tags.length > 0 && (
          <div style={{ display: "flex", gap: 5, flexWrap: "wrap" }}>
            {post.tags.map((t) => (
              <Link key={t.id} href={`/t/${t.slug}`} className="tag">
                # {t.name}
              </Link>
            ))}
          </div>
        )}
        <div
          className="tabular-nums"
          style={{
            marginLeft: "auto",
            display: "flex",
            gap: 14,
            alignItems: "center",
            color: "var(--text-3)",
            fontSize: 12.5,
          }}
        >
          <span title="浏览" style={{ display: "inline-flex", alignItems: "center", gap: 4 }}>
            <IconEye size={14} /> {fmtCount(post.view_count)}
          </span>
          <span title="评论" style={{ display: "inline-flex", alignItems: "center", gap: 4 }}>
            <IconMessage size={14} /> {fmtCount(post.comment_count)}
          </span>
          <span
            title="点赞"
            style={{
              display: "inline-flex",
              alignItems: "center",
              gap: 4,
              color: post.is_liked ? "var(--brand)" : undefined,
              fontWeight: post.is_liked ? 620 : undefined,
            }}
          >
            <IconThumbUp size={14} /> {fmtCount(post.like_count)}
          </span>
        </div>
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
  emptyDesc = "成为第一个发起讨论的人吧",
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
            去发布
          </Link>
        }
      />
    );
  return (
    <>
      <div style={{ display: "flex", flexDirection: "column", gap: 11 }}>
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
