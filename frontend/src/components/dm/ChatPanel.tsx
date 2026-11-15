"use client";

import { useEffect, useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { api, fileURL, timeAgo, type PageData } from "@/lib/api";
import type { Conversation, DmMessage } from "@/lib/types";
import { Avatar } from "@/components/ui/Avatar";
import { ListSkeleton } from "@/components/ui/States";
import { toast } from "@/lib/toast";

export function ChatPanel({ conv, onBack }: { conv: Conversation; onBack?: () => void }) {
  const qc = useQueryClient();
  const [text, setText] = useState("");
  const [sending, setSending] = useState(false);
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
    mutationFn: (body: string) => api.post(`/conversations/${conv.id}/messages`, { body_text: body }),
    onSuccess: () => {
      setText("");
      qc.invalidateQueries({ queryKey: ["dm", "messages", conv.id] });
      qc.invalidateQueries({ queryKey: ["dm", "conversations"] });
    },
    onError: (e) => toast("error", (e as Error).message),
  });

  const submit = () => {
    if (!text.trim() || sending) return;
    setSending(true);
    send.mutate(text.trim());
    setSending(false);
  };

  const recall = async (id: number) => {
    if (!window.confirm("撤回这条消息？")) return;
    try {
      await api.delete(`/messages/${id}`);
      qc.invalidateQueries({ queryKey: ["dm", "messages", conv.id] });
    } catch (e) {
      toast("error", (e as Error).message);
    }
  };

  const list = [...(messages.data?.items ?? [])].reverse();

  return (
    <div className="card" style={{ display: "flex", flexDirection: "column", height: 560 }}>
      {/* 头部 */}
      <div style={{ display: "flex", alignItems: "center", gap: 10, padding: "10px 14px", borderBottom: "1px solid var(--border)" }}>
        <button className="btn btn-text btn-sm dm-back" onClick={onBack} aria-label="返回会话列表" style={{ display: "none" }}>
          ←
        </button>
        <Avatar name={conv.other.display_name} avatarKey={conv.other.avatar_key} size={32} link={`/u/${conv.other.username}`} />
        <Link href={`/u/${conv.other.username}`} style={{ fontWeight: 700, fontSize: 14.5 }}>
          {conv.other.display_name}
        </Link>
        <div style={{ marginLeft: "auto", display: "flex", gap: 4 }}>
          <button
            className="btn btn-text btn-sm"
            title={conv.muted ? "取消免打扰" : "免打扰"}
            aria-label="免打扰切换"
            onClick={async () => {
              try {
                await api.patch(`/conversations/${conv.id}/settings`, { muted: !conv.muted });
                qc.invalidateQueries({ queryKey: ["dm", "conversations"] });
              } catch (e) {
                toast("error", (e as Error).message);
              }
            }}
          >
            {conv.muted ? "🔕" : "🔔"}
          </button>
          <button
            className="btn btn-text btn-sm"
            title={conv.pinned ? "取消置顶" : "置顶会话"}
            aria-label="置顶切换"
            onClick={async () => {
              try {
                await api.patch(`/conversations/${conv.id}/settings`, { pinned: !conv.pinned });
                qc.invalidateQueries({ queryKey: ["dm", "conversations"] });
              } catch (e) {
                toast("error", (e as Error).message);
              }
            }}
          >
            📌
          </button>
        </div>
      </div>

      {/* 消息列表 */}
      <div style={{ flex: 1, overflowY: "auto", padding: 14, display: "flex", flexDirection: "column", gap: 10 }}>
        {messages.isLoading && <ListSkeleton count={2} />}
        {!messages.isLoading && list.length === 0 && (
          <div style={{ textAlign: "center", color: "var(--text-3)", margin: "auto" }}>
            和 {conv.other.display_name} 打个招呼吧 👋
          </div>
        )}
        {list.map((m) => (
          <div key={m.id} style={{ display: "flex", justifyContent: m.mine ? "flex-end" : "flex-start" }}>
            <div
              style={{
                maxWidth: "72%",
                background: m.mine ? "var(--brand)" : "var(--bg-inset)",
                color: m.mine ? "#fff" : "var(--text-1)",
                borderRadius: 12,
                padding: "8px 12px",
                fontSize: 14,
                lineHeight: 1.55,
                wordBreak: "break-word",
              }}
            >
              {m.reply_body && (
                <div style={{ fontSize: 12, opacity: 0.75, borderLeft: "2px solid currentColor", paddingLeft: 8, marginBottom: 4 }}>
                  {m.reply_body}
                </div>
              )}
              {m.status === "recalled" ? (
                <i style={{ opacity: 0.6 }}>消息已撤回</i>
              ) : (
                <>
                  {m.body_text}
                  {m.image_key && (
                    <img src={fileURL(`/files/${m.image_key}`) ?? ""} alt="图片消息" style={{ maxWidth: 240, borderRadius: 8, display: "block", marginTop: m.body_text ? 6 : 0 }} />
                  )}
                </>
              )}
              <div style={{ fontSize: 11, opacity: 0.65, marginTop: 4, display: "flex", gap: 8, alignItems: "center" }}>
                {timeAgo(m.created_at)}
                {m.mine && m.status === "sent" && (
                  <button onClick={() => recall(m.id)} style={{ background: "none", border: "none", color: "inherit", cursor: "pointer", fontSize: 11, textDecoration: "underline" }}>
                    撤回
                  </button>
                )}
              </div>
            </div>
          </div>
        ))}
        <div ref={bottomRef} />
      </div>

      {/* 输入区 */}
      <div style={{ display: "flex", gap: 8, padding: 12, borderTop: "1px solid var(--border)" }}>
        <input
          className="input"
          value={text}
          maxLength={5000}
          placeholder="输入消息…（Enter 发送）"
          aria-label="消息输入框"
          onChange={(e) => setText(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter" && !e.shiftKey) {
              e.preventDefault();
              submit();
            }
          }}
        />
        <button className="btn btn-primary" onClick={submit} disabled={sending || !text.trim()}>
          发送
        </button>
      </div>
    </div>
  );
}
