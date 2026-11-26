"use client";

// 评论树（深度≤3）与评论表单；点赞/编辑/删除/举报/采纳最佳回答。
import { useState } from "react";
import Link from "next/link";
import { useQueryClient } from "@tanstack/react-query";
import { api, timeAgo, fmtCount } from "@/lib/api";
import type { CommentNode, PostDetail } from "@/lib/types";
import { Avatar } from "@/components/ui/Avatar";
import { MarkdownEditor } from "@/components/editor/MarkdownEditor";
import { ConfirmDialog, ImageLightbox } from "@/components/ui/Dialog";
import { ReportDialog } from "@/components/report/ReportDialog";
import { toast } from "@/lib/toast";
import { useMe } from "@/lib/auth";
import { IconCheckCircle, IconEdit, IconFlag, IconMessage, IconThumbUp, IconTrash } from "@/components/ui/Icons";

export function CommentTree({
  post,
  comments,
  loading,
  onReload,
}: {
  post: PostDetail;
  comments: CommentNode[] | undefined;
  loading: boolean;
  onReload: () => void;
}) {
  const { data: me } = useMe();
  const user = me?.authenticated ? me.user : null;
  const [replyTo, setReplyTo] = useState<CommentNode | null>(null);
  const [lightboxSrc, setLightboxSrc] = useState<string | null>(null);

  if (loading) {
    return (
      <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
        {[1, 2, 3].map((i) => (
          <div key={i} className="card" style={{ padding: 16 }}>
            <div className="skeleton" style={{ height: 13, width: "28%", marginBottom: 10 }} />
            <div className="skeleton" style={{ height: 14, width: "82%" }} />
          </div>
        ))}
      </div>
    );
  }
  if (!comments || comments.length === 0) {
    return (
      <div className="card" style={{ textAlign: "center", color: "var(--text-3)", padding: 36, fontSize: 13.5 }}>
        暂无评论，来发表第一条看法吧
      </div>
    );
  }
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
      {comments.map((c) => (
        <CommentItemView
          key={c.id}
          comment={c}
          post={post}
          user={user}
          onReload={onReload}
          replyTo={replyTo}
          setReplyTo={setReplyTo}
          onPreviewImage={setLightboxSrc}
        />
      ))}
      <ImageLightbox src={lightboxSrc} onClose={() => setLightboxSrc(null)} />
    </div>
  );
}

function CommentItemView({
  comment,
  post,
  user,
  onReload,
  replyTo,
  setReplyTo,
  onPreviewImage,
}: {
  comment: CommentNode;
  post: PostDetail;
  user: { username: string; email_verified: boolean } | null;
  onReload: () => void;
  replyTo: CommentNode | null;
  setReplyTo: (c: CommentNode | null) => void;
  onPreviewImage: (src: string) => void;
}) {
  const qc = useQueryClient();
  const [editing, setEditing] = useState(false);
  const [editBody, setEditBody] = useState(comment.body_markdown ?? comment.body_rendered);
  const [reporting, setReporting] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [accepting, setAccepting] = useState(false);
  const isMine = user && comment.is_author;
  const canModerate = post.can_moderate;

  const toggleAccept = async () => {
    setAccepting(true);
    try {
      if (comment.is_accepted_answer) {
        await api.delete(`/comments/${comment.id}/accept`);
        toast("success", "已取消最佳回答");
      } else {
        await api.post(`/comments/${comment.id}/accept`);
        toast("success", "已采纳为最佳回答");
      }
      qc.invalidateQueries({ queryKey: ["post", post.id] });
      onReload();
    } catch (e) {
      toast("error", (e as Error).message);
    } finally {
      setAccepting(false);
    }
  };

  const like = async () => {
    try {
      await api.post(`/comments/${comment.id}/like`);
      onReload();
    } catch (e) {
      toast("error", (e as Error).message);
    }
  };
  const unlike = async () => {
    try {
      await api.delete(`/comments/${comment.id}/like`);
      onReload();
    } catch (e) {
      toast("error", (e as Error).message);
    }
  };
  const remove = async () => {
    setDeleting(true);
    try {
      await api.delete(`/comments/${comment.id}`);
      toast("success", "评论已删除");
      setConfirmDelete(false);
      onReload();
    } catch (e) {
      toast("error", (e as Error).message);
    } finally {
      setDeleting(false);
    }
  };
  const saveEdit = async () => {
    try {
      await api.patch(`/comments/${comment.id}`, { body_markdown: editBody });
      toast("success", "评论已更新");
      setEditing(false);
      onReload();
    } catch (e) {
      toast("error", (e as Error).message);
    }
  };

  const deleted = comment.status === "deleted";
  const hidden = comment.status === "hidden";
  const unavailable = deleted || hidden;
  const canAccept =
    post.post_type === "question" &&
    post.status === "published" &&
    post.is_author &&
    !isMine &&
    comment.depth === 0 &&
    !unavailable;

  const isRoot = comment.depth === 0;

  return (
    <div
      className={isRoot ? "card" : undefined}
      style={{
        display: "flex",
        gap: 11,
        padding: isRoot ? "16px 18px" : "10px 0 4px 14px",
        marginLeft: comment.depth > 0 ? 18 : 0,
        borderLeft: comment.is_accepted_answer
          ? "3px solid var(--success)"
          : comment.depth > 0
            ? "2px solid var(--border)"
            : undefined,
        background: comment.is_accepted_answer
          ? "linear-gradient(90deg, var(--success-soft) 0%, var(--bg-card) 42%)"
          : undefined,
      }}
    >
      <Avatar
        name={comment.author.display_name}
        avatarKey={comment.author.avatar_key}
        size={isRoot ? 34 : 26}
        link={`/u/${comment.author.username}`}
      />
      <div style={{ flex: 1, minWidth: 0 }}>
        <div style={{ display: "flex", gap: 8, alignItems: "center", fontSize: 12.5, color: "var(--text-3)", flexWrap: "wrap" }}>
          <Link href={`/u/${comment.author.username}`} style={{ fontWeight: 620, color: "var(--text-1)" }}>
            {comment.author.display_name}
          </Link>
          {comment.author.username === post.author.username && (
            <span className="tag tag-brand" style={{ fontSize: 10.5, padding: "0 7px", lineHeight: "17px" }}>作者</span>
          )}
          {comment.is_accepted_answer && (
            <span className="tag tag-success">
              <IconCheckCircle size={11} /> 已采纳最佳回答
            </span>
          )}
          <span className="tabular-nums">{timeAgo(comment.created_at)}</span>
          {comment.edited_at && <span>· 已编辑</span>}
          {hidden && <span className="tag tag-danger">已隐藏</span>}
          {deleted && <span className="tag">已删除</span>}
        </div>

        {comment.quote_id && comment.quote_body && (
          <div
            style={{
              borderLeft: "2px solid var(--brand)",
              padding: "6px 11px",
              margin: "8px 0 4px",
              fontSize: 12.5,
              color: "var(--text-2)",
              background: "var(--bg-inset)",
              borderRadius: "0 8px 8px 0",
            }}
          >
            回复 <span style={{ fontWeight: 620, color: "var(--text-1)" }}>{comment.quote_author}</span>：{comment.quote_body}
          </div>
        )}

        {!unavailable && (
          <div
            className="prose-community"
            style={{ fontSize: 14.5, marginTop: 6 }}
            onClick={(e) => {
              const target = e.target as HTMLElement;
              if (target.tagName === "IMG") {
                onPreviewImage((target as HTMLImageElement).src);
              }
            }}
            dangerouslySetInnerHTML={{ __html: comment.body_rendered }}
          />
        )}
        {deleted && (
          <div style={{ color: "var(--text-3)", fontSize: 13.5, fontStyle: "italic", padding: "8px 0" }}>
            该评论已被删除
          </div>
        )}
        {hidden && (
          <div style={{ color: "var(--text-3)", fontSize: 13.5, fontStyle: "italic", padding: "8px 0" }}>
            该评论因不符合社区规范已被隐藏
          </div>
        )}

        {editing ? (
          <div style={{ marginTop: 10 }}>
            <MarkdownEditor value={editBody} onChange={setEditBody} minHeight={140} onSubmitShortcut={saveEdit} />
            <div style={{ display: "flex", gap: 8, marginTop: 8 }}>
              <button className="btn btn-primary btn-sm" onClick={saveEdit}>
                保存修改
              </button>
              <button className="btn btn-secondary btn-sm" onClick={() => setEditing(false)}>
                取消
              </button>
            </div>
          </div>
        ) : (
          !unavailable && (
            <div style={{ display: "flex", gap: 4, marginTop: 6, fontSize: 12.5, color: "var(--text-3)", flexWrap: "wrap", alignItems: "center" }}>
              <button
                className="btn btn-text btn-xs tabular-nums"
                style={{ color: comment.is_liked ? "var(--brand)" : undefined }}
                onClick={comment.is_liked ? unlike : like}
                disabled={!user}
              >
                <IconThumbUp size={13} /> {fmtCount(comment.like_count)}
              </button>

              {user && post.status === "published" && (
                <button className="btn btn-text btn-xs" onClick={() => setReplyTo(comment)}>
                  <IconMessage size={13} /> {comment.depth < 2 ? "回复" : "引用回复"}
                </button>
              )}

              {canAccept && (
                <button
                  className="btn btn-text btn-xs"
                  style={{ color: comment.is_accepted_answer ? "var(--warning)" : "var(--success)" }}
                  disabled={accepting}
                  onClick={toggleAccept}
                >
                  <IconCheckCircle size={13} /> {comment.is_accepted_answer ? "取消采纳" : "采纳回答"}
                </button>
              )}

              {isMine && (
                <button
                  className="btn btn-text btn-xs"
                  onClick={() => {
                    setEditBody(comment.body_markdown ?? comment.body_rendered);
                    setEditing(true);
                  }}
                >
                  <IconEdit size={13} /> 编辑
                </button>
              )}

              {(isMine || canModerate) && (
                <button className="btn btn-text btn-xs" style={{ color: "var(--danger)" }} onClick={() => setConfirmDelete(true)}>
                  <IconTrash size={13} /> 删除
                </button>
              )}

              {user && !isMine && (
                <button className="btn btn-text btn-xs" onClick={() => setReporting(true)}>
                  <IconFlag size={13} /> 举报
                </button>
              )}
            </div>
          )
        )}

        {replyTo?.id === comment.id && (
          <CommentForm
            postId={post.id}
            parent={comment}
            onDone={() => {
              setReplyTo(null);
              onReload();
            }}
            onCancel={() => setReplyTo(null)}
          />
        )}

        {comment.replies && comment.replies.length > 0 && (
          <div style={{ marginTop: 8, display: "flex", flexDirection: "column", gap: 6 }}>
            {comment.replies.map((child) => (
              <CommentItemView
                key={child.id}
                comment={child}
                post={post}
                user={user}
                onReload={onReload}
                replyTo={replyTo}
                setReplyTo={setReplyTo}
                onPreviewImage={onPreviewImage}
              />
            ))}
          </div>
        )}
      </div>

      <ReportDialog open={reporting} onClose={() => setReporting(false)} targetType="comment" targetId={comment.id} />
      <ConfirmDialog
        open={confirmDelete}
        title="删除评论"
        description="删除后无法恢复，确认删除这条评论吗？"
        confirmText="确认删除"
        danger
        loading={deleting}
        onConfirm={remove}
        onClose={() => setConfirmDelete(false)}
      />
    </div>
  );
}

export function CommentForm({
  postId,
  parent,
  onDone,
  onCancel,
}: {
  postId: string;
  parent?: CommentNode | null;
  onDone: () => void;
  onCancel?: () => void;
}) {
  const [body, setBody] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const { data: me } = useMe();
  const user = me?.authenticated ? me.user : null;

  if (!user) {
    return (
      <div style={{ padding: "14px 8px", textAlign: "center", color: "var(--text-3)", fontSize: 13.5 }}>
        <Link href="/login" style={{ color: "var(--brand)", fontWeight: 500 }}>
          登录
        </Link>{" "}
        后即可参与评论讨论
      </div>
    );
  }
  if (!user.email_verified) {
    return (
      <div style={{ padding: "14px 8px", textAlign: "center", color: "var(--text-3)", fontSize: 13.5 }}>
        完成<Link href="/settings" style={{ color: "var(--brand)" }}>邮箱验证</Link>后即可发表评论
      </div>
    );
  }

  const submit = async () => {
    if (!body.trim()) {
      toast("error", "评论内容不能为空");
      return;
    }
    setSubmitting(true);
    try {
      const res = await api.post<{ status: string }>(`/posts/${postId}/comments`, {
        body_markdown: body,
        parent_id: parent?.id,
      });
      if (res.status === "pending_review") {
        toast("info", "评论已提交，等待审核");
      } else {
        toast("success", "评论已发布");
      }
      setBody("");
      onDone();
    } catch (e) {
      toast("error", (e as Error).message);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <div style={{ marginTop: parent ? 10 : 0 }}>
      {parent && (
        <div style={{ fontSize: 12.5, color: "var(--text-3)", marginBottom: 6 }}>
          回复 <b>{parent.author.display_name}</b>
          {parent.depth >= 2 && "（将自动转为引用回复）"}
        </div>
      )}
      <MarkdownEditor
        value={body}
        onChange={setBody}
        minHeight={parent ? 110 : 140}
        placeholder="理性发言，支持 Markdown 与快捷键 Ctrl+Enter 发送…"
        onSubmitShortcut={submit}
      />
      <div style={{ display: "flex", gap: 8, marginTop: 8, alignItems: "center" }}>
        <button className="btn btn-primary btn-sm" onClick={submit} disabled={submitting || !body.trim()}>
          {submitting ? "发布中…" : parent ? "发送回复" : "发布评论"}
        </button>
        {onCancel && (
          <button className="btn btn-secondary btn-sm" onClick={onCancel}>
            取消
          </button>
        )}
      </div>
    </div>
  );
}
