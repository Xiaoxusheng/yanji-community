"use client";

// 帖子详情：净化正文、互动条、评论树、版主操作。
import Link from "next/link";
import { useParams } from "next/navigation";
import { useEffect, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, timeAgo, fmtCount } from "@/lib/api";
import type { PostDetail, CommentNode } from "@/lib/types";
import type { PageData } from "@/lib/api";
import { Avatar } from "@/components/ui/Avatar";
import { CardSkeleton, ErrorState } from "@/components/ui/States";
import { CommentTree, CommentForm } from "@/components/comment/CommentTree";
import { ReportDialog } from "@/components/report/ReportDialog";
import { PollBox } from "@/components/poll/PollBox";
import { useMe } from "@/lib/auth";
import { toast } from "@/lib/toast";

const statusLabels: Record<string, { text: string; cls: string }> = {
  draft: { text: "草稿", cls: "" },
  pending_review: { text: "待审核", cls: "tag-warning" },
  rejected: { text: "未通过审核", cls: "tag-danger" },
  hidden: { text: "已隐藏", cls: "tag-danger" },
  locked: { text: "已锁定", cls: "tag-info" },
};

export default function PostDetailPage() {
  const params = useParams<{ id: string }>();
  const id = params.id;
  const qc = useQueryClient();
  const { data: me } = useMe();
  const user = me?.authenticated ? me.user : null;
  const [commentPage, setCommentPage] = useState(1);
  const [reporting, setReporting] = useState(false);
  const [sharing, setSharing] = useState(false);

  const detail = useQuery({
    queryKey: ["post", id],
    queryFn: () => api.get<{ post: PostDetail }>(`/posts/${id}`),
  });
  const comments = useQuery({
    queryKey: ["post-comments", id, commentPage],
    queryFn: () =>
      api.get<PageData<CommentNode>>(`/posts/${id}/comments?page=${commentPage}&page_size=20`),
  });

  // 浏览计数（去重由后端负责）
  useEffect(() => {
    if (!detail.data) return;
    api.post(`/posts/${id}/view`).catch(() => {});
  }, [id, detail.data]);

  if (detail.isLoading) {
    return (
      <div style={{ maxWidth: 820, margin: "0 auto" }}>
        <CardSkeleton lines={6} />
        <div style={{ height: 24 }} />
        <CardSkeleton lines={2} />
      </div>
    );
  }
  if (detail.error || !detail.data)
    return (
      <div style={{ maxWidth: 820, margin: "0 auto" }}>
        <ErrorState message={(detail.error as Error)?.message} onRetry={() => detail.refetch()} />
      </div>
    );

  const p = detail.data.post;
  const reloadComments = () => qc.invalidateQueries({ queryKey: ["post-comments", id] });
  const st = statusLabels[p.status];

  const like = async () => {
    try {
      if (p.is_liked) {
        await api.delete(`/posts/${p.id}/like`);
      } else {
        await api.post(`/posts/${p.id}/like`);
      }
      qc.invalidateQueries({ queryKey: ["post", id] });
    } catch (e) {
      toast("error", (e as Error).message);
    }
  };
  const bookmark = async () => {
    try {
      if (p.is_bookmarked) {
        await api.delete(`/posts/${p.id}/bookmark`);
      } else {
        await api.post(`/posts/${p.id}/bookmark`, {});
      }
      qc.invalidateQueries({ queryKey: ["post", id] });
      toast("success", p.is_bookmarked ? "已取消收藏" : "已收藏");
    } catch (e) {
      toast("error", (e as Error).message);
    }
  };
  const moderate = async (action: string, needReason = false) => {
    let reason = "";
    if (needReason) {
      reason = window.prompt("请填写原因（将通知作者）") ?? "";
      if (!reason) return;
    }
    try {
      await api.post(`/admin/posts/${p.id}/moderate`, { action, reason });
      toast("success", "操作成功");
      qc.invalidateQueries({ queryKey: ["post", id] });
    } catch (e) {
      toast("error", (e as Error).message);
    }
  };
  const share = async () => {
    try {
      await navigator.clipboard.writeText(window.location.href);
      toast("success", "链接已复制");
    } catch {
      setSharing(true);
      setTimeout(() => setSharing(false), 2000);
    }
  };

  return (
    <div style={{ display: "grid", gridTemplateColumns: "minmax(0,1fr) 300px", gap: 24, alignItems: "start" }}>
      <article>
        {/* 面包屑 */}
        <nav aria-label="面包屑" style={{ fontSize: 13, color: "var(--text-3)", marginBottom: 12 }}>
          <Link href="/categories">版块</Link> /{" "}
          <Link href={`/c/${p.category.slug}`} style={{ color: "var(--brand)" }}>
            {p.category.name}
          </Link>
          {st && (
            <>
              {" / "}
              <span className={`tag ${st.cls}`}>{st.text}</span>
            </>
          )}
        </nav>

        <div className="card" style={{ padding: 24 }}>
          <div style={{ display: "flex", gap: 6, marginBottom: 10, flexWrap: "wrap" }}>
            {p.is_pinned && <span className="tag tag-brand">置顶</span>}
            {p.is_featured && <span className="tag tag-warning">精华</span>}
            {p.post_type === "article" && <span className="tag tag-info">文章</span>}
          </div>
          <h1 style={{ fontSize: 26, fontWeight: 700, lineHeight: 1.4, marginBottom: 12 }}>{p.title}</h1>
          <div style={{ display: "flex", alignItems: "center", gap: 10, fontSize: 13.5, color: "var(--text-3)", marginBottom: 20 }}>
            <Avatar name={p.author.display_name} avatarKey={p.author.avatar_key} size={32} link={`/u/${p.author.username}`} />
            <Link href={`/u/${p.author.username}`} style={{ fontWeight: 600, color: "var(--text-1)" }}>
              {p.author.display_name}
            </Link>
            <span>{timeAgo(p.published_at || p.created_at)}发布</span>
            {p.edited_at && <span>· 已编辑</span>}
            <span>· 👁 {fmtCount(p.view_count)}</span>
          </div>

          {p.status === "rejected" && p.reject_reason && (
            <div style={{ background: "var(--danger-soft)", color: "var(--danger)", padding: "10px 14px", borderRadius: 8, marginBottom: 16, fontSize: 14 }}>
              未通过审核原因：{p.reject_reason}
            </div>
          )}
          {p.status === "pending_review" && (
            <div style={{ background: "var(--warning-soft)", color: "var(--warning)", padding: "10px 14px", borderRadius: 8, marginBottom: 16, fontSize: 14 }}>
              该内容正在等待审核，仅你和管理人员可见
            </div>
          )}
          {p.status === "locked" && (
            <div style={{ background: "var(--info-soft)", color: "var(--info)", padding: "10px 14px", borderRadius: 8, marginBottom: 16, fontSize: 14 }}>
              该帖子已被锁定，可以浏览但无法新增评论
            </div>
          )}

          {/* 正文（服务端已净化） */}
          <div className="prose-community" dangerouslySetInnerHTML={{ __html: p.body_rendered }} />

          {p.post_type === "poll" && <PollBox postId={p.id} />}
          {p.post_type === "question" && (
            <div style={{ marginTop: 16, padding: "10px 14px", background: "var(--info-soft)", borderRadius: 8, fontSize: 13.5, color: "var(--info)" }}>
              ❓ 问答帖：在下方选择一个满意的回答点击「采纳」，被采纳者将获得积分与「首答」勋章。
            </div>
          )}

          {p.tags.length > 0 && (
            <div style={{ display: "flex", gap: 6, flexWrap: "wrap", marginTop: 20 }}>
              {p.tags.map((t) => (
                <Link key={t.id} href={`/t/${t.slug}`} className="tag">
                  # {t.name}
                </Link>
              ))}
            </div>
          )}

          {/* 互动条 */}
          <div style={{ display: "flex", gap: 6, marginTop: 20, paddingTop: 16, borderTop: "1px solid var(--border)", flexWrap: "wrap" }}>
            <button className={`btn btn-sm ${p.is_liked ? "btn-primary" : "btn-secondary"}`} onClick={like} disabled={!user}>
              👍 {p.is_liked ? "已赞" : "点赞"} · {fmtCount(p.like_count)}
            </button>
            <button className={`btn btn-sm ${p.is_bookmarked ? "btn-primary" : "btn-secondary"}`} onClick={bookmark} disabled={!user}>
              ⭐ {p.is_bookmarked ? "已收藏" : "收藏"} · {fmtCount(p.bookmark_count)}
            </button>
            <button className="btn btn-secondary btn-sm" onClick={share}>
              🔗 {sharing ? window.location.href : "分享"}
            </button>
            {user && !p.is_author && (
              <button className="btn btn-text btn-sm" onClick={() => setReporting(true)}>
                举报
              </button>
            )}
            {p.is_author && (
              <Link href={`/compose?post=${p.id}`} className="btn btn-text btn-sm">
                编辑
              </Link>
            )}
            {p.can_moderate && (
              <span style={{ marginLeft: "auto", display: "flex", gap: 4, flexWrap: "wrap" }}>
                {p.status === "pending_review" && (
                  <>
                    <button className="btn btn-sm" style={{ background: "var(--success)", color: "#fff" }} onClick={() => moderate("approve")}>
                      通过
                    </button>
                    <button className="btn btn-danger btn-sm" onClick={() => moderate("reject", true)}>
                      驳回
                    </button>
                  </>
                )}
                {p.status === "published" && (
                  <>
                    <button className="btn btn-secondary btn-sm" onClick={() => moderate(p.is_pinned ? "unpin" : "pin")}>
                      {p.is_pinned ? "取消置顶" : "置顶"}
                    </button>
                    <button className="btn btn-secondary btn-sm" onClick={() => moderate(p.is_featured ? "unfeature" : "feature")}>
                      {p.is_featured ? "取消精华" : "加精"}
                    </button>
                    <button className="btn btn-secondary btn-sm" onClick={() => moderate("lock")}>
                      锁定
                    </button>
                    <button className="btn btn-danger btn-sm" onClick={() => moderate("hide", true)}>
                      隐藏
                    </button>
                  </>
                )}
                {p.status === "locked" && (
                  <button className="btn btn-secondary btn-sm" onClick={() => moderate("unlock")}>
                    解锁
                  </button>
                )}
                {p.status === "hidden" && (
                  <button className="btn btn-secondary btn-sm" onClick={() => moderate("restore")}>
                    恢复
                  </button>
                )}
              </span>
            )}
          </div>
        </div>

        {/* 评论 */}
        <section style={{ marginTop: 20 }} aria-label="评论区">
          <h2 style={{ fontSize: 17, fontWeight: 700, marginBottom: 12 }}>
            评论 <span style={{ color: "var(--text-3)", fontWeight: 400 }}>{p.comment_count}</span>
          </h2>
          <div className="card" style={{ padding: 16, marginBottom: 16 }}>
            {p.status === "locked" ? (
              <div style={{ textAlign: "center", color: "var(--text-3)" }}>帖子已锁定，无法新增评论</div>
            ) : p.status === "published" || p.status === "locked" ? (
              <CommentForm postId={p.id} onDone={reloadComments} />
            ) : (
              <div style={{ textAlign: "center", color: "var(--text-3)" }}>当前状态下不可评论</div>
            )}
          </div>
          <CommentTree post={p} comments={comments.data?.items} loading={comments.isLoading} onReload={reloadComments} />
          {comments.data && comments.data.total > 20 && (
            <div style={{ display: "flex", justifyContent: "center", gap: 8, marginTop: 16 }}>
              <button className="btn btn-secondary btn-sm" disabled={commentPage <= 1} onClick={() => setCommentPage(commentPage - 1)}>
                上一页
              </button>
              <span style={{ alignSelf: "center", fontSize: 13, color: "var(--text-3)" }}>
                第 {commentPage} / {Math.ceil(comments.data.total / 20)} 页
              </span>
              <button
                className="btn btn-secondary btn-sm"
                disabled={commentPage >= Math.ceil(comments.data.total / 20)}
                onClick={() => setCommentPage(commentPage + 1)}
              >
                下一页
              </button>
            </div>
          )}
        </section>
      </article>

      {/* 作者卡 */}
      <aside style={{ position: "sticky", top: 72 }}>
        <section className="card" style={{ padding: 16, textAlign: "center" }}>
          <Avatar name={p.author.display_name} avatarKey={p.author.avatar_key} size={56} link={`/u/${p.author.username}`} />
          <div style={{ fontWeight: 700, marginTop: 8 }}>
            <Link href={`/u/${p.author.username}`}>{p.author.display_name}</Link>
          </div>
          <div style={{ color: "var(--text-3)", fontSize: 13 }}>@{p.author.username}</div>
        </section>
      </aside>

      <ReportDialog open={reporting} onClose={() => setReporting(false)} targetType="post" targetId={p.id} />
    </div>
  );
}
