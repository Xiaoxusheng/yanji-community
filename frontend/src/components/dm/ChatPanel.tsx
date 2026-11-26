"use client";

// 一对一私信聊天面板：品牌渐变气泡、居中撤回微胶囊、悬停操作栏、图片灯箱与 Framer Motion 入场动效。
import { useEffect, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { motion } from "framer-motion";
import { api, fileURL, timeAgo, type PageData } from "@/lib/api";
import type { Conversation, DmMessage } from "@/lib/types";
import { Avatar } from "@/components/ui/Avatar";
import { ListSkeleton } from "@/components/ui/States";
import { ConfirmDialog, ImageLightbox } from "@/components/ui/Dialog";
import { ReportDialog } from "@/components/report/ReportDialog";
import { toast } from "@/lib/toast";
import { IconArrowLeft, IconBell, IconBellOff, IconImage, IconPin, IconSend, IconX } from "@/components/ui/Icons";

export function ChatPanel({ conv, onBack }: { conv: Conversation; onBack?: () => void }) {
  const qc = useQueryClient();
  const [text, setText] = useState("");
  const [replyTo, setReplyTo] = useState<DmMessage | null>(null);
  const [uploadingImg, setUploadingImg] = useState(false);
  const [recallTarget, setRecallTarget] = useState<number | null>(null);
  const [reportTargetId, setReportTargetId] = useState<string | null>(null);
  const [lightboxSrc, setLightboxSrc] = useState<string | null>(null);
  const bottomRef = useRef<HTMLDivElement>(null);

  const messages = useQuery({
    queryKey: ["dm", "messages", conv.id],
    refetchInterval: 8_000,
    queryFn: () => api.get<PageData<DmMessage>>(`/conversations/${conv.id}/messages?page=1&page_size=100`),
  });

  // 打开会话即已读
  useEffect(() => {
    api.post(`/conversations/${conv.id}/read`).catch(() => {});
    qc.invalidateQueries({ queryKey: ["dm", "conversations"] });
  }, [conv.id, qc]);

  // 新消息滚动到底
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages.data?.items.length]);

  const send = useMutation({
    mutationFn: (payload: { body_text: string; image_key?: string; reply_to_id?: number }) =>
      api.post(`/conversations/${conv.id}/messages`, payload),
    onSuccess: () => {
      setText("");
      setReplyTo(null);
      qc.invalidateQueries({ queryKey: ["dm", "messages", conv.id] });
      qc.invalidateQueries({ queryKey: ["dm", "conversations"] });
    },
    onError: (e) => toast("error", (e as Error).message),
  });

  const submit = () => {
    if (!text.trim() || send.isPending) return;
    send.mutate({
      body_text: text.trim(),
      ...(replyTo ? { reply_to_id: replyTo.id } : {}),
    });
  };

  const sendImage = async (file: File) => {
    const fd = new FormData();
    fd.append("file", file);
    setUploadingImg(true);
    try {
      const res = await api.upload<{ key: string }>("/uploads", fd);
      send.mutate({
        body_text: text.trim() || "[图片]",
        image_key: res.key,
        ...(replyTo ? { reply_to_id: replyTo.id } : {}),
      });
    } catch (e) {
      toast("error", (e as Error).message);
    } finally {
      setUploadingImg(false);
    }
  };

  const recall = async () => {
    if (!recallTarget) return;
    try {
      await api.delete(`/messages/${recallTarget}`);
      setRecallTarget(null);
      qc.invalidateQueries({ queryKey: ["dm", "messages", conv.id] });
      toast("success", "已撤回消息");
    } catch (e) {
      toast("error", (e as Error).message);
    }
  };

  const list = [...(messages.data?.items ?? [])].reverse();

  return (
    <div className="card" style={{ display: "flex", flexDirection: "column", height: 600, overflow: "hidden" }}>
      {/* 头部 */}
      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: 10,
          padding: "12px 16px",
          borderBottom: "1px solid var(--border)",
          background: "var(--bg-card)",
        }}
      >
        <button className="btn btn-icon dm-back-btn" onClick={onBack} aria-label="返回会话列表" style={{ display: "none" }}>
          <IconArrowLeft size={16} />
        </button>
        <Avatar name={conv.other.display_name} avatarKey={conv.other.avatar_key} size={34} link={`/u/${conv.other.username}`} />
        <div style={{ minWidth: 0 }}>
          <Link href={`/u/${conv.other.username}`} style={{ fontWeight: 660, fontSize: 14.5 }}>
            {conv.other.display_name}
          </Link>
          <div style={{ fontSize: 11.5, color: "var(--text-3)" }}>@{conv.other.username}</div>
        </div>
        <div style={{ marginLeft: "auto", display: "flex", gap: 4 }}>
          <button
            className="btn btn-icon"
            title={conv.muted ? "取消免打扰" : "免打扰"}
            aria-label="免打扰切换"
            style={{ color: conv.muted ? "var(--brand)" : undefined, background: conv.muted ? "var(--brand-soft)" : undefined }}
            onClick={async () => {
              try {
                await api.patch(`/conversations/${conv.id}/settings`, { muted: !conv.muted });
                qc.invalidateQueries({ queryKey: ["dm", "conversations"] });
              } catch (e) {
                toast("error", (e as Error).message);
              }
            }}
          >
            {conv.muted ? <IconBellOff size={16} /> : <IconBell size={16} />}
          </button>
          <button
            className="btn btn-icon"
            title={conv.pinned ? "取消置顶" : "置顶会话"}
            aria-label="置顶切换"
            style={{ color: conv.pinned ? "var(--brand)" : undefined, background: conv.pinned ? "var(--brand-soft)" : undefined }}
            onClick={async () => {
              try {
                await api.patch(`/conversations/${conv.id}/settings`, { pinned: !conv.pinned });
                qc.invalidateQueries({ queryKey: ["dm", "conversations"] });
              } catch (e) {
                toast("error", (e as Error).message);
              }
            }}
          >
            <IconPin size={16} />
          </button>
        </div>
      </div>

      {/* 消息列表 */}
      <div
        style={{
          flex: 1,
          overflowY: "auto",
          padding: 16,
          display: "flex",
          flexDirection: "column",
          gap: 11,
          background: "var(--bg-subtle)",
        }}
      >
        {messages.isLoading && <ListSkeleton count={2} />}
        {!messages.isLoading && list.length === 0 && (
          <div style={{ textAlign: "center", color: "var(--text-3)", margin: "auto", fontSize: 13.5 }}>
            和 {conv.other.display_name} 打个招呼，开始对话吧
          </div>
        )}
        {list.map((m) =>
          m.status === "recalled" ? (
            <div key={m.id} style={{ display: "flex", justifyContent: "center" }}>
              <span
                style={{
                  fontSize: 12,
                  color: "var(--text-3)",
                  background: "var(--bg-inset)",
                  border: "1px solid var(--border)",
                  padding: "3px 12px",
                  borderRadius: 999,
                }}
              >
                {m.mine ? "你撤回了一条消息" : "对方撤回了一条消息"}
              </span>
            </div>
          ) : (
            <motion.div
              key={m.id}
              initial={{ opacity: 0, y: 6 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.18 }}
              className="dm-bubble-wrap"
              style={{ display: "flex", flexDirection: "column", alignItems: m.mine ? "flex-end" : "flex-start" }}
            >
              <div
                style={{
                  maxWidth: "74%",
                  background: m.mine ? "var(--brand-gradient)" : "var(--bg-card)",
                  color: m.mine ? "#ffffff" : "var(--text-1)",
                  border: m.mine ? "none" : "1px solid var(--border)",
                  boxShadow: m.mine ? "0 3px 10px rgba(59, 91, 219, 0.22)" : "var(--shadow-card)",
                  borderRadius: m.mine ? "15px 15px 4px 15px" : "15px 15px 15px 4px",
                  padding: "10px 14px",
                  fontSize: 14,
                  lineHeight: 1.58,
                  wordBreak: "break-word",
                }}
              >
                {m.reply_body && (
                  <div
                    style={{
                      fontSize: 12,
                      opacity: 0.82,
                      borderLeft: `2px solid ${m.mine ? "rgba(255,255,255,0.65)" : "var(--brand)"}`,
                      paddingLeft: 8,
                      marginBottom: 6,
                    }}
                  >
                    {m.reply_body}
                  </div>
                )}
                {m.body_text !== "[图片]" && <div>{m.body_text}</div>}
                {m.image_key && (
                  <img
                    src={fileURL(m.image_key) ?? ""}
                    alt="图片消息"
                    onClick={() => setLightboxSrc(fileURL(m.image_key ?? ""))}
                    style={{
                      maxWidth: 240,
                      borderRadius: 9,
                      display: "block",
                      marginTop: m.body_text && m.body_text !== "[图片]" ? 6 : 0,
                      cursor: "zoom-in",
                    }}
                  />
                )}
              </div>
              <div
                className="tabular-nums dm-bubble-actions"
                style={{
                  fontSize: 11,
                  color: "var(--text-3)",
                  marginTop: 4,
                  padding: "0 4px",
                  display: "flex",
                  gap: 8,
                  alignItems: "center",
                }}
              >
                <span>{timeAgo(m.created_at)}</span>
                <button
                  type="button"
                  onClick={() => setReplyTo(m)}
                  style={{ background: "none", border: "none", color: "inherit", cursor: "pointer", fontSize: 11, padding: 0 }}
                >
                  引用
                </button>
                {m.mine ? (
                  <button
                    type="button"
                    onClick={() => setRecallTarget(m.id)}
                    style={{ background: "none", border: "none", color: "inherit", cursor: "pointer", fontSize: 11, padding: 0 }}
                  >
                    撤回
                  </button>
                ) : (
                  <button
                    type="button"
                    onClick={() => setReportTargetId(String(m.id))}
                    style={{ background: "none", border: "none", color: "inherit", cursor: "pointer", fontSize: 11, padding: 0 }}
                  >
                    举报
                  </button>
                )}
              </div>
            </motion.div>
          ),
        )}
        <div ref={bottomRef} />
      </div>

      {/* 引用预览 */}
      {replyTo && (
        <div style={{ padding: "7px 14px", background: "var(--bg-inset)", borderTop: "1px solid var(--border)", display: "flex", alignItems: "center", gap: 8, fontSize: 12.5 }}>
          <span style={{ color: "var(--text-2)", flex: 1, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
            引用：{replyTo.body_text || "[图片]"}
          </span>
          <button className="btn btn-icon" style={{ width: 24, height: 24 }} onClick={() => setReplyTo(null)} aria-label="取消引用">
            <IconX size={13} />
          </button>
        </div>
      )}

      {/* 输入区 */}
      <div style={{ display: "flex", gap: 8, padding: 12, borderTop: "1px solid var(--border)", alignItems: "center", background: "var(--bg-card)" }}>
        <label
          className="btn btn-icon"
          title="发送图片"
          aria-label="发送图片"
          style={{
            border: "1px solid var(--border-strong)",
            cursor: uploadingImg ? "wait" : "pointer",
            opacity: uploadingImg ? 0.5 : 1,
          }}
        >
          <IconImage size={16} />
          <input
            type="file"
            accept="image/jpeg,image/png,image/gif,image/webp"
            hidden
            disabled={uploadingImg || send.isPending}
            onChange={(e) => {
              const f = e.target.files?.[0];
              if (f) sendImage(f);
              e.target.value = "";
            }}
          />
        </label>
        <input
          className="input"
          value={text}
          maxLength={5000}
          placeholder="输入消息…（按 Enter 快速发送）"
          aria-label="消息输入框"
          onChange={(e) => setText(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter" && !e.shiftKey) {
              e.preventDefault();
              submit();
            }
          }}
        />
        <button className="btn btn-brand btn-sm" onClick={submit} disabled={send.isPending || !text.trim()}>
          <IconSend size={14} />
          <span>{send.isPending ? "发送中" : "发送"}</span>
        </button>
      </div>

      <ConfirmDialog
        open={recallTarget !== null}
        title="撤回消息"
        description="撤回后对方将看到「消息已撤回」，仅发送 5 分钟内的消息可撤回。"
        confirmText="撤回"
        danger
        onConfirm={recall}
        onClose={() => setRecallTarget(null)}
      />
      <ReportDialog
        open={reportTargetId !== null}
        onClose={() => setReportTargetId(null)}
        targetType="message"
        targetId={reportTargetId ?? ""}
      />
      <ImageLightbox src={lightboxSrc} onClose={() => setLightboxSrc(null)} />
    </div>
  );
}
