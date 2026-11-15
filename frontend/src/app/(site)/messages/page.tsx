"use client";

// 私信中心：会话列表 + 聊天面板（桌面双栏，移动端切换）。
import Link from "next/link";
import { Suspense, useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";
import { useQuery } from "@tanstack/react-query";
import { api, timeAgo, type PageData } from "@/lib/api";
import type { Conversation } from "@/lib/types";
import { Avatar } from "@/components/ui/Avatar";
import { ListSkeleton, ErrorState, EmptyState } from "@/components/ui/States";
import { AuthShell } from "@/components/auth/forms";
import { useMe } from "@/lib/auth";
import { ChatPanel } from "@/components/dm/ChatPanel";

function MessagesInner() {
  const { data: me, isLoading: meLoading } = useMe();
  const searchParams = useSearchParams();
  const [activeId, setActiveId] = useState<number | null>(null);
  const [mobileChat, setMobileChat] = useState(false);

  // 支持 /messages?conv=<id> 直达会话
  useEffect(() => {
    const conv = searchParams.get("conv");
    if (conv) setActiveId(Number(conv) || null);
  }, [searchParams]);

  const conversations = useQuery({
    enabled: !!me?.authenticated,
    queryKey: ["dm", "conversations"],
    refetchInterval: 30_000,
    queryFn: () => api.get<PageData<Conversation>>("/me/conversations?page=1&page_size=50"),
  });

  if (meLoading) return null;
  if (me && !me.authenticated) {
    return (
      <AuthShell title="请先登录">
        <p style={{ marginBottom: 16 }}>登录后使用私信。</p>
        <Link href="/login" className="btn btn-primary">去登录</Link>
      </AuthShell>
    );
  }

  const openConv = (id: number) => {
    setActiveId(id);
    setMobileChat(true);
  };
  const active = conversations.data?.items.find((c) => c.id === activeId) ?? null;

  return (
    <div style={{ maxWidth: 980, margin: "0 auto" }}>
      <h1 style={{ fontSize: 20, fontWeight: 700, marginBottom: 14 }}>私信</h1>
      <div style={{ display: "grid", gridTemplateColumns: "300px minmax(0,1fr)", gap: 16, alignItems: "start" }}>
        {/* 会话列表 */}
        <aside style={{ display: mobileChat ? "none" : "flex", flexDirection: "column", gap: 8 }}>
          {conversations.isLoading && <ListSkeleton count={3} />}
          {conversations.error && <ErrorState message={(conversations.error as Error).message} onRetry={() => conversations.refetch()} />}
          {conversations.data && conversations.data.items.length === 0 && (
            <EmptyState title="还没有会话" description="在用户主页点击「发私信」开始对话" />
          )}
          {(conversations.data?.items ?? []).map((c) => (
            <button
              key={c.id}
              onClick={() => openConv(c.id)}
              className="card"
              style={{
                padding: 12, display: "flex", gap: 10, alignItems: "center", textAlign: "left",
                cursor: "pointer",
                borderColor: activeId === c.id ? "var(--brand)" : undefined,
                background: activeId === c.id ? "var(--brand-soft)" : undefined,
              }}
            >
              <Avatar name={c.other.display_name} avatarKey={c.other.avatar_key} size={40} />
              <div style={{ flex: 1, minWidth: 0 }}>
                <div style={{ display: "flex", gap: 6, alignItems: "center" }}>
                  <b style={{ fontSize: 14 }}>{c.other.display_name}</b>
                  {c.pinned && <span className="tag">📌</span>}
                  {c.muted && <span className="tag" title="免打扰">🔕</span>}
                  <span style={{ marginLeft: "auto", fontSize: 11.5, color: "var(--text-3)" }}>
                    {c.last_message_at ? timeAgo(c.last_message_at) : ""}
                  </span>
                </div>
                <div style={{ fontSize: 13, color: "var(--text-3)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                  {c.last_sender_me && "我："}
                  {c.last_message || "（无消息）"}
                </div>
              </div>
              {c.unread > 0 && (
                <span
                  style={{
                    background: "var(--danger)", color: "#fff", borderRadius: 999, fontSize: 11,
                    minWidth: 18, height: 18, display: "flex", alignItems: "center", justifyContent: "center",
                    padding: "0 5px", fontWeight: 700, flexShrink: 0,
                  }}
                >
                  {c.unread > 99 ? "99+" : c.unread}
                </span>
              )}
            </button>
          ))}
        </aside>

        {/* 聊天面板 */}
        <section style={{ display: mobileChat ? "block" : "block" }}>
          {active ? (
            <ChatPanel
              conv={active}
              onBack={() => setMobileChat(false)}
            />
          ) : (
            <div className="card" style={{ padding: 48, textAlign: "center", color: "var(--text-3)" }}>
              选择一个会话开始聊天
            </div>
          )}
        </section>
      </div>
      <style>{`@media (max-width: 640px){ .dm-back{display:inline-flex} }`}</style>
    </div>
  );
}

export default function MessagesPage() {
  return (
    <Suspense>
      <MessagesInner />
    </Suspense>
  );
}
