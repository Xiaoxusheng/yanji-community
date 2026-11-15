"use client";

// 评论树（深度≤3）与评论表单；点赞/编辑/删除/举报。
import { useState } from "react";
import Link from "next/link";
import { api, timeAgo } from "@/lib/api";
import type { CommentNode, PostDetail } from "@/lib/types";
import { Avatar } from "@/components/ui/Avatar";
import { MarkdownEditor } from "@/components/editor/MarkdownEditor";
import { toast } from "@/lib/toast";
import { useMe } from "@/lib/auth";
import { ReportDialog } from "@/components/report/ReportDialog";

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

  if (loading) {
    return (
      <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
        {[1, 2, 3].map((i) => (
          <div key={i} className="card" style={{ padding: 16 }}>
            <div className="skeleton" style={{ height: 14, width: "30%", marginBottom: 10 }} />
            <div className="skeleton" style={{ height: 14, width: "80%" }} />
          </div>
        ))}
      </div>
    );
  }
  if (!comments || comments.length === 0) {
    return (
      <div style={{ textAlign: "center", color: "var(--text-3)", padding: 32 }}>
        还没有评论，来抢沙发 →
      </div>
    );
  }
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
      {comments.map((c) => (
        <CommentItemView
          key={c.id}
          comment={c}
          post={post}
          user={user}
          onReload={onReload}
          replyTo={replyTo}
          setReplyTo={setReplyTo}
        />
      ))}
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
}: {
  comment: CommentNode;
  post: PostDetail;
  user: { username: string; email_verified: boolean } | null;
  onReload: () => void;
  replyTo: CommentNode | null;
  setReplyTo: (c: CommentNode | null) => void;
}) {
  const [editing, setEditing] = useState(false);
  const [editBody, setEditBody] = useState(comment.body_rendered);
  const [reporting, setReporting] = useState(false);
  const isMine = user && comment.is_author;
  const canModerate = post.can_moderate;

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
    if (!window.confirm("确定删除这条评论吗？")) return;
    try {
      await api.delete(`/comments/${comment.id}`);
      toast("success", "评论已删除");
      onReload();
    } catch (e) {
      toast("error", (e as Error).message);
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

  return (
    <div
      style={{
        display: "flex",
        gap: 10,
        paddingLeft: comment.depth > 0 ? 18 * comment.depth : 0,
        borderLeft: comment.depth > 0 ? "2px solid var(--border)" : "none",
      }}
    >
      <Avatar
        name={comment.author.display_name}
        avatarKey={comment.author.avatar_key}
        size={32}
        link={`/u/${comment.author.username}`}
      />
      <div style={{ flex: 1, minWidth: 0 }}>
        <div style={{ display: "flex", gap: 8, alignItems: "center", fontSize: 13, color: "var(--text-3)", flexWrap: "wrap" }}>
          <Link href={`/u/${comment.author.username}`} style={{ fontWeight: 600, color: "var(--text-1)" }}>
            {comment.author.display_name}
          </Link>
          <span>{timeAgo(comment.created_at)}</span>
          {comment.edited_at && <span>· 已编辑</span>}
          {hidden && <span className="tag tag-danger">已隐藏</span>}
          {deleted && <span className="tag">已删除</span>}
        </div>
        {comment.quote_id && (
          <div
            style={{
              borderLeft: "3px solid var(--border-strong)",
              padding: "4px 10px",
              margin: "6px 0",
              fontSize: 13,
              color: "var(--text-3)",
              background: "var(--bg-inset)",
              borderRadius: 4,
            }}
          >
            引用 <b>{comment.quote_author}</b>：{comment.quote_body}
          </div>
        )}
        {!deleted && (
          <div
            className="prose-community"
            style={{ fontSize: 14.5, marginTop: 4 }}
            dangerouslySetInnerHTML={{ __html: comment.body_rendered }}
          />
        )}
        {deleted && <div style={{ color: "var(--text-3)", fontSize: 14, padding: "4px 0" }}>该评论已删除</div>}

        {editing ? (
          <div style={{ marginTop: 8 }}>
            <MarkdownEditor value={editBody} onChange={setEditBody} minHeight={140} />
            <div style={{ display: "flex", gap: 8, marginTop: 8 }}>
              <button className="btn btn-primary btn-sm" onClick={saveEdit}>
                保存
              </button>
              <button className="btn btn-secondary btn-sm" onClick={() => setEditing(false)}>
                取消
              </button>
            </div>
          </div>
        ) : (
          !deleted && (
            <div style={{ display: "flex", gap: 4, marginTop: 6, fontSize: 13, color: "var(--text-3)", flexWrap: "wrap" }}>
              {comment.is_liked ? (
                <button className="btn btn-text" style={{ color: "var(--brand)" }} onClick={unlike} disabled={!user}>
                  👍 {comment.like_count}
                </button>
              ) : (
                <button className="btn btn-text" onClick={like} disabled={!user}>
                  👍 {comment.like_count}
                </button>
              )}
              {user && post.status === "published" && comment.depth < 2 && (
                <button className="btn btn-text" onClick={() => setReplyTo(comment)}>
                  回复
                </button>
              )}
              {user && post.status === "published" && comment.depth >= 2 && (
                <button className="btn btn-text" onClick={() => setReplyTo(comment)}>
                  引用回复
                </button>
              )}
              {isMine && (
                <button
                  className="btn btn-text"
                  onClick={() => {
                    // 编辑框需要 Markdown 源文本；后端仅回传渲染结果，取原文需再拉修订 —— 简化：以渲染文本编辑
                    setEditBody(comment.body_rendered);
                    setEditing(true);
                  }}
                >
                  编辑
                </button>
              )}
              {(isMine || canModerate) && (
                <button className="btn btn-text" style={{ color: "var(--danger)" }} onClick={remove}>
                  删除
                </button>
              )}
              {user && !isMine && (
                <button className="btn btn-text" onClick={() => setReporting(true)}>
                  举报
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
      </div>
      <ReportDialog
        open={reporting}
        onClose={() => setReporting(false)}
        targetType="comment"
        targetId={comment.id}
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
      <div className="card" style={{ padding: 16, textAlign: "center", color: "var(--text-3)" }}>
        <Link href="/login" style={{ color: "var(--brand)" }}>
          登录
        </Link>{" "}
        后参与讨论
      </div>
    );
  }
  if (!user.email_verified) {
    return (
      <div className="card" style={{ padding: 16, textAlign: "center", color: "var(--text-3)" }}>
        完成邮箱验证后即可评论，请查收验证邮件
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
    <div style={{ marginTop: 10 }}>
      {parent && (
        <div style={{ fontSize: 13, color: "var(--text-3)", marginBottom: 6 }}>
          回复 <b>{parent.author.display_name}</b>
          {parent.depth >= 2 && "（已达最大嵌套，将以引用回复形式展示）"}
        </div>
      )}
      <MarkdownEditor value={body} onChange={setBody} minHeight={parent ? 120 : 180} placeholder="友善发言，理性讨论…" />
      <div style={{ display: "flex", gap: 8, marginTop: 8 }}>
        <button className="btn btn-primary" onClick={submit} disabled={submitting || !body.trim()}>
          {submitting ? "发布中…" : parent ? "回复" : "发布评论"}
        </button>
        {onCancel && (
          <button className="btn btn-secondary" onClick={onCancel}>
            取消
          </button>
        )}
      </div>
    </div>
  );
}
