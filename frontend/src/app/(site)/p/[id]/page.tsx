"use client";

// 帖子详情：阅读进度条、代码块复制、图片灯箱、作者名片、收藏夹归档、修订历史与评论排序。
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, timeAgo, fmtCount } from "@/lib/api";
import type { PostDetail, CommentNode, FolderRow, ProfilePublic } from "@/lib/types";
import type { PageData } from "@/lib/api";
import { Avatar } from "@/components/ui/Avatar";
import { CardSkeleton, ErrorState } from "@/components/ui/States";
import { CommentTree, CommentForm } from "@/components/comment/CommentTree";
import { PostTypeBadge } from "@/components/post/PostCard";
import { ConfirmDialog, ImageLightbox, ModalDialog, PromptDialog } from "@/components/ui/Dialog";
import { ReportDialog } from "@/components/report/ReportDialog";
import { PollBox } from "@/components/poll/PollBox";
import { useMe } from "@/lib/auth";
import { toast } from "@/lib/toast";
import {
  IconAward,
  IconBookmark,
  IconCheck,
  IconEdit,
  IconEye,
  IconFlag,
  IconHelpCircle,
  IconHistory,
  IconLock,
  IconMessage,
  IconPin,
  IconShare,
  IconThumbUp,
  IconTrash,
} from "@/components/ui/Icons";

const statusLabels: Record<string, { text: string; cls: string }> = {
  draft: { text: "草稿", cls: "" },
  pending_review: { text: "待审核", cls: "tag-warning" },
  rejected: { text: "未通过审核", cls: "tag-danger" },
  hidden: { text: "已隐藏", cls: "tag-danger" },
  locked: { text: "已锁定", cls: "tag-info" },
};

interface RevisionItem {
  id: number;
  title: string;
  body_markdown: string;
  edit_reason: string;
  editor_name: string;
  created_at: string;
}

export default function PostDetailPage() {
  const params = useParams<{ id: string }>();
  const router = useRouter();
  const id = params.id;
  const qc = useQueryClient();
  const { data: me } = useMe();
  const user = me?.authenticated ? me.user : null;

  const [commentPage, setCommentPage] = useState(1);
  const [commentSort, setCommentSort] = useState<"latest" | "hot">("latest");
  const [reporting, setReporting] = useState(false);
  const [sharing, setSharing] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [modPrompt, setModPrompt] = useState<{ action: string; title: string } | null>(null);
  const [modBusy, setModBusy] = useState(false);
  const [revOpen, setRevOpen] = useState(false);
  const [bookmarkModalOpen, setBookmarkModalOpen] = useState(false);
  const [lightboxSrc, setLightboxSrc] = useState<string | null>(null);
  const [readProgress, setReadProgress] = useState(0);

  const proseRef = useRef<HTMLDivElement>(null);

  const detail = useQuery({
    queryKey: ["post", id],
    queryFn: () => api.get<{ post: PostDetail }>(`/posts/${id}`),
  });
  const comments = useQuery({
    queryKey: ["post-comments", id, commentSort, commentPage],
    queryFn: () =>
      api.get<PageData<CommentNode>>(`/posts/${id}/comments?sort=${commentSort}&page=${commentPage}&page_size=20`),
  });

  const p = detail.data?.post;

  const authorProfile = useQuery({
    enabled: !!p?.author.username,
    queryKey: ["profile", p?.author.username],
    queryFn: () => api.get<{ profile: ProfilePublic }>(`/users/${p!.author.username}`),
  });

  const folders = useQuery({
    enabled: !!user && bookmarkModalOpen,
    queryKey: ["bookmark-folders"],
    queryFn: () => api.get<{ items: FolderRow[] }>("/me/bookmark-folders"),
  });

  const revisions = useQuery({
    enabled: revOpen && !!p && (p.is_author || p.can_moderate),
    queryKey: ["post-revisions", id],
    queryFn: () => api.get<{ items: RevisionItem[] }>(`/posts/${id}/revisions`),
  });

  // 浏览计数
  useEffect(() => {
    if (!detail.data) return;
    api.post(`/posts/${id}/view`).catch(() => {});
  }, [id, detail.data]);

  // 顶部阅读进度条
  useEffect(() => {
    const onScroll = () => {
      const doc = document.documentElement;
      const total = doc.scrollHeight - window.innerHeight;
      setReadProgress(total > 0 ? Math.min(100, Math.max(0, (window.scrollY / total) * 100)) : 0);
    };
    window.addEventListener("scroll", onScroll, { passive: true });
    return () => window.removeEventListener("scroll", onScroll);
  }, []);

  // 为正文 <pre> 代码块动态注入「复制」按钮
  useEffect(() => {
    const el = proseRef.current;
    if (!el) return;
    const pres = Array.from(el.querySelectorAll("pre"));
    pres.forEach((pre) => {
      if (pre.querySelector(".code-copy-btn")) return;
      const btn = document.createElement("button");
      btn.type = "button";
      btn.className = "code-copy-btn";
      btn.textContent = "复制";
      btn.onclick = async () => {
        const code = pre.querySelector("code")?.textContent ?? pre.textContent ?? "";
        try {
          await navigator.clipboard.writeText(code);
          btn.textContent = "已复制";
          setTimeout(() => {
            btn.textContent = "复制";
          }, 1500);
        } catch {
          toast("error", "复制失败");
        }
      };
      pre.appendChild(btn);
    });
  }, [p?.body_rendered]);

  if (detail.isLoading) {
    return (
      <div style={{ maxWidth: 820, margin: "0 auto" }}>
        <CardSkeleton lines={6} />
        <div style={{ height: 20 }} />
        <CardSkeleton lines={2} />
      </div>
    );
  }
  if (detail.error || !p) {
    return (
      <div style={{ maxWidth: 820, margin: "0 auto" }}>
        <ErrorState message={(detail.error as Error)?.message} onRetry={() => detail.refetch()} />
      </div>
    );
  }

  const reloadComments = () => qc.invalidateQueries({ queryKey: ["post-comments", id] });
  const st = statusLabels[p.status];
  const ap = authorProfile.data?.profile;

  // 估测阅读时长（剥除 HTML 标签）
  const plainLen = p.body_rendered.replace(/<[^>]+>/g, "").trim().length;
  const readMins = Math.max(1, Math.ceil(plainLen / 420));

  const like = async () => {
    try {
      if (p.is_liked) await api.delete(`/posts/${p.id}/like`);
      else await api.post(`/posts/${p.id}/like`);
      qc.invalidateQueries({ queryKey: ["post", id] });
    } catch (e) {
      toast("error", (e as Error).message);
    }
  };

  const bookmarkDirect = async (folderId?: number) => {
    try {
      if (p.is_bookmarked && folderId === undefined) {
        await api.delete(`/posts/${p.id}/bookmark`);
        toast("success", "已取消收藏");
      } else {
        await api.post(`/posts/${p.id}/bookmark`, folderId ? { folder_id: folderId } : {});
        toast("success", "已加入收藏");
      }
      setBookmarkModalOpen(false);
      qc.invalidateQueries({ queryKey: ["post", id] });
    } catch (e) {
      toast("error", (e as Error).message);
    }
  };

  const deletePost = async () => {
    setDeleting(true);
    try {
      await api.delete(`/posts/${p.id}`);
      toast("success", "帖子已删除");
      setConfirmDelete(false);
      router.push("/me/posts");
    } catch (e) {
      toast("error", (e as Error).message);
    } finally {
      setDeleting(false);
    }
  };

  const moderate = async (action: string, reason = "") => {
    setModBusy(true);
    try {
      await api.post(`/admin/posts/${p.id}/moderate`, { action, reason });
      toast("success", "操作成功");
      setModPrompt(null);
      qc.invalidateQueries({ queryKey: ["post", id] });
    } catch (e) {
      toast("error", (e as Error).message);
    } finally {
      setModBusy(false);
    }
  };

  const share = async () => {
    try {
      await navigator.clipboard.writeText(window.location.href);
      toast("success", "帖子链接已复制");
    } catch {
      setSharing(true);
      setTimeout(() => setSharing(false), 2000);
    }
  };

  const toggleFollowAuthor = async () => {
    if (!ap) return;
    try {
      if (ap.is_following) {
        await api.delete(`/users/${ap.id}/follow`);
        toast("success", "已取消关注");
      } else {
        await api.post(`/users/${ap.id}/follow`);
        toast("success", "已关注作者");
      }
      qc.invalidateQueries({ queryKey: ["profile", p.author.username] });
    } catch (e) {
      toast("error", (e as Error).message);
    }
  };

  return (
    <>
      {/* 顶部阅读进度条（渐变微光） */}
      <div
        aria-hidden
        style={{
          position: "fixed",
          top: 60,
          left: 0,
          height: 2.5,
          width: `${readProgress}%`,
          background: "var(--brand-gradient)",
          boxShadow: readProgress > 0 ? "0 0 10px var(--brand-ring)" : "none",
          zIndex: 99,
          transition: "width 80ms linear",
        }}
      />

      <div className="layout-main-rail">
        <article>
          {/* 面包屑 */}
          <nav aria-label="面包屑" style={{ fontSize: 13, color: "var(--text-3)", marginBottom: 14, display: "flex", gap: 6, alignItems: "center", flexWrap: "wrap" }}>
            <Link href="/categories">版块</Link>
            <span>/</span>
            <Link href={`/c/${p.category.slug}`} style={{ color: "var(--text-2)", fontWeight: 550 }}>
              {p.category.name}
            </Link>
            {st && (
              <>
                <span>/</span>
                <span className={`tag ${st.cls}`}>{st.text}</span>
              </>
            )}
          </nav>

          <div className="card" style={{ padding: "28px 30px" }}>
            <div style={{ display: "flex", gap: 6, marginBottom: 12, flexWrap: "wrap" }}>
              {p.is_pinned && (
                <span className="tag tag-brand">
                  <IconPin size={11} /> 置顶
                </span>
              )}
              {p.is_featured && (
                <span className="tag tag-warning">
                  <IconAward size={11} /> 精华
                </span>
              )}
              <PostTypeBadge type={p.post_type} />
            </div>

            <h1 style={{ fontSize: 27, fontWeight: 740, lineHeight: 1.35, letterSpacing: "-0.025em", marginBottom: 16 }}>
              {p.title}
            </h1>

            <div
              style={{
                display: "flex",
                alignItems: "center",
                gap: 11,
                fontSize: 13,
                color: "var(--text-3)",
                marginBottom: 24,
                padding: "12px 14px",
                background: "var(--bg-inset)",
                borderRadius: "var(--radius-sm)",
                border: "1px solid var(--border)",
                flexWrap: "wrap",
              }}
            >
              <Avatar name={p.author.display_name} avatarKey={p.author.avatar_key} size={34} link={`/u/${p.author.username}`} />
              <div>
                <Link href={`/u/${p.author.username}`} style={{ fontWeight: 650, color: "var(--text-1)" }}>
                  {p.author.display_name}
                </Link>
                <div className="tabular-nums" style={{ fontSize: 12, color: "var(--text-3)", marginTop: 1 }}>
                  {timeAgo(p.published_at || p.created_at)}发布
                  {p.edited_at && " · 已编辑"}
                  {` · 约 ${readMins} 分钟阅读`}
                </div>
              </div>
              <span className="tag tag-outline tabular-nums" style={{ marginLeft: "auto", background: "var(--bg-card)" }}>
                <IconEye size={13} /> {fmtCount(p.view_count)} 阅读
              </span>
            </div>

            {p.status === "rejected" && p.reject_reason && (
              <div style={{ background: "var(--danger-soft)", color: "var(--danger)", padding: "10px 14px", borderRadius: 8, marginBottom: 16, fontSize: 13.5 }}>
                未通过审核原因：{p.reject_reason}
              </div>
            )}
            {p.status === "pending_review" && (
              <div style={{ background: "var(--warning-soft)", color: "var(--warning)", padding: "10px 14px", borderRadius: 8, marginBottom: 16, fontSize: 13.5 }}>
                该内容正在等待审核，目前仅作者本人与管理团队可见
              </div>
            )}
            {p.status === "locked" && (
              <div style={{ background: "var(--info-soft)", color: "var(--info)", padding: "10px 14px", borderRadius: 8, marginBottom: 16, fontSize: 13.5, display: "flex", alignItems: "center", gap: 6 }}>
                <IconLock size={14} /> 该帖子已被锁定，当前仅可浏览，暂无法新增评论
              </div>
            )}

            {/* 正文 */}
            <div
              ref={proseRef}
              className="prose-community"
              onClick={(e) => {
                const target = e.target as HTMLElement;
                if (target.tagName === "IMG") {
                  setLightboxSrc((target as HTMLImageElement).src);
                }
              }}
              dangerouslySetInnerHTML={{ __html: p.body_rendered }}
            />

            {p.post_type === "poll" && <PollBox postId={p.id} />}
            {p.post_type === "question" && (
              <div
                style={{
                  marginTop: 18,
                  padding: "10px 14px",
                  background: "var(--info-soft)",
                  borderRadius: 8,
                  fontSize: 13.5,
                  color: "var(--info)",
                  display: "flex",
                  alignItems: "center",
                  gap: 8,
                }}
              >
                <IconHelpCircle size={16} />
                <span>问答帖：提问者可在下方挑选一条最满意的回答点击「采纳回答」，回答者将获得积分奖励与勋章。</span>
              </div>
            )}

            {p.tags.length > 0 && (
              <div style={{ display: "flex", gap: 6, flexWrap: "wrap", marginTop: 22 }}>
                {p.tags.map((t) => (
                  <Link key={t.id} href={`/t/${t.slug}`} className="tag">
                    # {t.name}
                  </Link>
                ))}
              </div>
            )}

            {/* 互动与操作栏 */}
            <div
              style={{
                display: "flex",
                gap: 8,
                marginTop: 22,
                paddingTop: 16,
                borderTop: "1px solid var(--border)",
                flexWrap: "wrap",
                alignItems: "center",
              }}
            >
              <button
                className={`btn btn-sm tabular-nums ${p.is_liked ? "btn-brand" : "btn-secondary"}`}
                onClick={like}
                disabled={!user}
              >
                <IconThumbUp size={14} /> {p.is_liked ? "已赞" : "点赞"} · {fmtCount(p.like_count)}
              </button>

              <button
                className={`btn btn-sm tabular-nums ${p.is_bookmarked ? "btn-brand" : "btn-secondary"}`}
                onClick={() => (p.is_bookmarked ? bookmarkDirect() : setBookmarkModalOpen(true))}
                disabled={!user}
              >
                <IconBookmark size={14} /> {p.is_bookmarked ? "已收藏" : "收藏"} · {fmtCount(p.bookmark_count)}
              </button>

              <button className="btn btn-secondary btn-sm" onClick={share}>
                <IconShare size={14} /> {sharing ? "已复制" : "分享"}
              </button>

              {user && !p.is_author && (
                <button className="btn btn-text btn-sm" onClick={() => setReporting(true)}>
                  <IconFlag size={14} /> 举报
                </button>
              )}

              {(p.is_author || p.can_moderate) && (
                <button className="btn btn-text btn-sm" onClick={() => setRevOpen(true)}>
                  <IconHistory size={14} /> 修订历史
                </button>
              )}

              {p.is_author && (
                <>
                  <Link href={`/compose?post=${p.id}`} className="btn btn-text btn-sm">
                    <IconEdit size={14} /> 编辑
                  </Link>
                  <button className="btn btn-text btn-sm" style={{ color: "var(--danger)" }} onClick={() => setConfirmDelete(true)}>
                    <IconTrash size={14} /> 删除
                  </button>
                </>
              )}

              {p.can_moderate && (
                <span style={{ marginLeft: "auto", display: "flex", gap: 6, flexWrap: "wrap" }}>
                  {p.status === "pending_review" && (
                    <>
                      <button className="btn btn-sm" style={{ background: "var(--success)", color: "#fff" }} onClick={() => moderate("approve")}>
                        通过
                      </button>
                      <button className="btn btn-danger btn-sm" onClick={() => setModPrompt({ action: "reject", title: "驳回帖子" })}>
                        驳回
                      </button>
                    </>
                  )}
                  {p.status === "published" && (
                    <>
                      <button className="btn btn-secondary btn-xs" onClick={() => moderate(p.is_pinned ? "unpin" : "pin")}>
                        {p.is_pinned ? "取消置顶" : "置顶"}
                      </button>
                      <button className="btn btn-secondary btn-xs" onClick={() => moderate(p.is_featured ? "unfeature" : "feature")}>
                        {p.is_featured ? "取消精华" : "加精"}
                      </button>
                      <button className="btn btn-secondary btn-xs" onClick={() => moderate("lock")}>
                        锁定
                      </button>
                      <button className="btn btn-danger btn-xs" onClick={() => setModPrompt({ action: "hide", title: "隐藏帖子" })}>
                        隐藏
                      </button>
                    </>
                  )}
                  {p.status === "locked" && (
                    <button className="btn btn-secondary btn-xs" onClick={() => moderate("unlock")}>
                      解锁
                    </button>
                  )}
                  {p.status === "hidden" && (
                    <button className="btn btn-secondary btn-xs" onClick={() => moderate("restore")}>
                      恢复
                    </button>
                  )}
                </span>
              )}
            </div>
          </div>

          {/* 评论区 */}
          <section style={{ marginTop: 24 }} aria-label="评论区">
            <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 12 }}>
              <h2 style={{ fontSize: 16.5, fontWeight: 650, display: "inline-flex", alignItems: "center", gap: 6 }}>
                <IconMessage size={16} />
                <span>全部评论</span>
                <span className="tabular-nums" style={{ color: "var(--text-3)", fontWeight: 500, fontSize: 14 }}>
                  ({p.comment_count})
                </span>
              </h2>
              <div className="seg-tabs">
                <button
                  type="button"
                  className={`seg-tab ${commentSort === "latest" ? "is-active" : ""}`}
                  style={{ padding: "4px 10px", fontSize: 12.5 }}
                  onClick={() => {
                    setCommentSort("latest");
                    setCommentPage(1);
                  }}
                >
                  时间顺序
                </button>
                <button
                  type="button"
                  className={`seg-tab ${commentSort === "hot" ? "is-active" : ""}`}
                  style={{ padding: "4px 10px", fontSize: 12.5 }}
                  onClick={() => {
                    setCommentSort("hot");
                    setCommentPage(1);
                  }}
                >
                  最热优先
                </button>
              </div>
            </div>

            <div className="card" style={{ padding: 16, marginBottom: 14 }}>
              {p.status === "locked" ? (
                <div style={{ textAlign: "center", color: "var(--text-3)", fontSize: 13.5, padding: 8 }}>
                  帖子已锁定，无法新增评论
                </div>
              ) : p.status === "published" ? (
                <CommentForm postId={p.id} onDone={reloadComments} />
              ) : (
                <div style={{ textAlign: "center", color: "var(--text-3)", fontSize: 13.5, padding: 8 }}>
                  当前状态下暂不可评论
                </div>
              )}
            </div>

            <CommentTree post={p} comments={comments.data?.items} loading={comments.isLoading} onReload={reloadComments} />

            {comments.data && comments.data.total > 20 && (
              <div style={{ display: "flex", justifyContent: "center", gap: 8, marginTop: 18 }}>
                <button className="btn btn-secondary btn-sm" disabled={commentPage <= 1} onClick={() => setCommentPage(commentPage - 1)}>
                  上一页
                </button>
                <span className="tabular-nums" style={{ alignSelf: "center", fontSize: 13, color: "var(--text-3)" }}>
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

        {/* 右侧作者信息卡 */}
        <aside className="sticky-rail">
          <section className="surface-spotlight" style={{ padding: 20 }}>
            <div style={{ display: "flex", gap: 12, alignItems: "center", marginBottom: 14 }}>
              <Avatar name={p.author.display_name} avatarKey={p.author.avatar_key} size={46} link={`/u/${p.author.username}`} ring />
              <div style={{ minWidth: 0, flex: 1 }}>
                <Link href={`/u/${p.author.username}`} style={{ fontWeight: 680, fontSize: 15.5, display: "block", letterSpacing: "-0.015em" }}>
                  {p.author.display_name}
                </Link>
                <div style={{ color: "var(--text-3)", fontSize: 12.5 }}>@{p.author.username}</div>
              </div>
            </div>
            {ap && !ap.restricted && ap.bio && (
              <p style={{ fontSize: 13, color: "var(--text-2)", lineHeight: 1.6, margin: "0 0 14px" }}>{ap.bio}</p>
            )}
            {ap && (
              <div
                className="tabular-nums"
                style={{
                  display: "grid",
                  gridTemplateColumns: "1fr 1fr 1fr",
                  gap: 7,
                  marginBottom: 14,
                  fontSize: 11.5,
                  color: "var(--text-3)",
                }}
              >
                <div className="stat-tile">
                  <div style={{ fontWeight: 700, fontSize: 14.5, color: "var(--text-1)" }}>{ap.post_count}</div>
                  <div>帖子</div>
                </div>
                <div className="stat-tile">
                  <div style={{ fontWeight: 700, fontSize: 14.5, color: "var(--text-1)" }}>{ap.follower_count}</div>
                  <div>粉丝</div>
                </div>
                <div className="stat-tile">
                  <div style={{ fontWeight: 700, fontSize: 14.5, color: "var(--text-1)" }}>{ap.comment_count}</div>
                  <div>评论</div>
                </div>
              </div>
            )}
            {user && !p.is_author && ap && (
              <div style={{ display: "flex", gap: 8 }}>
                <button
                  className={`btn btn-sm ${ap.is_following ? "btn-secondary" : "btn-brand"}`}
                  style={{ flex: 1 }}
                  onClick={toggleFollowAuthor}
                >
                  {ap.is_following ? (
                    <>
                      <IconCheck size={13} /> 已关注
                    </>
                  ) : (
                    "+ 关注"
                  )}
                </button>
                <button
                  className="btn btn-secondary btn-sm"
                  style={{ flex: 1 }}
                  onClick={async () => {
                    try {
                      const res = await api.post<{ conversation: { id: number } }>("/conversations", { target_id: ap.id });
                      router.push(`/messages?conv=${res.conversation.id}`);
                    } catch (e) {
                      toast("error", (e as Error).message);
                    }
                  }}
                >
                  发私信
                </button>
              </div>
            )}
          </section>
        </aside>
      </div>

      {/* 选择收藏夹模态框 */}
      <ModalDialog open={bookmarkModalOpen} onClose={() => setBookmarkModalOpen(false)} title="添加到收藏夹" width={380}>
        <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
          <button
            type="button"
            className="btn btn-secondary btn-sm"
            style={{ justifyContent: "space-between" }}
            onClick={() => bookmarkDirect(0)}
          >
            <span>默认收藏夹</span>
            <span style={{ color: "var(--text-3)", fontSize: 12 }}>直接收藏</span>
          </button>
          {(folders.data?.items ?? [])
            .filter((f) => f.id > 0)
            .map((f) => (
              <button
                key={f.id}
                type="button"
                className="btn btn-secondary btn-sm"
                style={{ justifyContent: "space-between" }}
                onClick={() => bookmarkDirect(f.id)}
              >
                <span>{f.name}</span>
                <span className="tabular-nums" style={{ color: "var(--text-3)", fontSize: 12 }}>{f.count} 篇</span>
              </button>
            ))}
        </div>
        <div style={{ marginTop: 12, fontSize: 12.5, color: "var(--text-3)" }}>
          可在 <Link href="/me/bookmarks" style={{ color: "var(--brand)" }}>我的收藏</Link> 中新建或管理收藏夹
        </div>
      </ModalDialog>

      {/* 修订历史模态框 */}
      <ModalDialog open={revOpen} onClose={() => setRevOpen(false)} title="帖子编辑修订历史" width={620}>
        {revisions.isLoading && <div style={{ color: "var(--text-3)", fontSize: 13 }}>加载记录中…</div>}
        {!revisions.isLoading && (revisions.data?.items.length ?? 0) === 0 && (
          <div style={{ color: "var(--text-3)", fontSize: 13.5 }}>暂无历史版本记录</div>
        )}
        <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
          {(revisions.data?.items ?? []).map((rev) => (
            <div key={rev.id} className="card" style={{ padding: 14, background: "var(--bg-inset)" }}>
              <div style={{ display: "flex", justifyContent: "space-between", fontSize: 12.5, color: "var(--text-3)", marginBottom: 6 }}>
                <span>{rev.editor_name} · {rev.edit_reason || "更新内容"}</span>
                <span className="tabular-nums">{timeAgo(rev.created_at)}</span>
              </div>
              <div style={{ fontWeight: 600, fontSize: 14, marginBottom: 6 }}>{rev.title}</div>
              <pre style={{ fontSize: 12.5, whiteSpace: "pre-wrap", margin: 0, color: "var(--text-2)", fontFamily: "inherit" }}>
                {rev.body_markdown}
              </pre>
            </div>
          ))}
        </div>
      </ModalDialog>

      <ReportDialog open={reporting} onClose={() => setReporting(false)} targetType="post" targetId={p.id} />
      <ConfirmDialog
        open={confirmDelete}
        title="删除帖子"
        description="确认删除这篇帖子吗？删除后将无法恢复。"
        confirmText="确认删除"
        danger
        loading={deleting}
        onConfirm={deletePost}
        onClose={() => setConfirmDelete(false)}
      />
      <PromptDialog
        open={!!modPrompt}
        title={modPrompt?.title ?? ""}
        label="请填写处理原因（必填，将通知作者并记入审计日志）"
        confirmText="确认提交"
        danger
        loading={modBusy}
        onConfirm={(reason) => modPrompt && moderate(modPrompt.action, reason)}
        onClose={() => setModPrompt(null)}
      />
      <ImageLightbox src={lightboxSrc} onClose={() => setLightboxSrc(null)} />
    </>
  );
}
