"use client";

// 私信中心：会话列表 + 聊天面板（桌面双栏，移动端响应式切换）。
import Link from "next/link";
import { Suspense, useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";
import { useQuery } from "@tanstack/react-query";
import { api, timeAgo, type PageData } from "@/lib/api";
import type { Conversation } from "@/lib/types";
import { Avatar } from "@/components/ui/Avatar";
import { ListSkeleton, ErrorState, EmptyState, PageHeader } from "@/components/ui/States";
import { AuthShell } from "@/components/auth/forms";
import { useMe } from "@/lib/auth";
import { ChatPanel } from "@/components/dm/ChatPanel";
import { IconBellOff, IconPin } from "@/components/ui/Icons";

function MessagesInner() {
  const { data: me, isLoading: meLoading } = useMe();
  const searchParams = useSearchParams();
  const [activeId, setActiveId] = useState<number | null>(null);
  const [mobileChat, setMobileChat] = useState(false);

  // 支持 /messages?conv=<id> 直达会话
  useEffect(() => {
    const conv = searchParams.get("conv");
    if (conv) {
      setActiveId(Number(conv) || null);
      setMobileChat(true);
    }
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
        <p style={{ marginBottom: 16, color: "var(--text-2)" }}>登录后即可使用一对一私信。</p>
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
      <PageHeader title="私信" subtitle="与社区成员一对一沟通" />
      <div className="layout-dm-grid">
        {/* 会话列表 */}
        <aside className={mobileChat ? "dm-pane-hidden-mobile" : undefined} style={{ display: "flex", flexDirection: "column", gap: 8 }}>
          {conversations.isLoading && <ListSkeleton count={3} />}
          {conversations.error && <ErrorState message={(conversations.error as Error).message} onRetry={() => conversations.refetch()} />}
          {conversations.data && conversations.data.items.length === 0 && (
            <EmptyState title="还没有会话" description="在任意成员主页点击「发私信」开启对话" />
          )}
          {(conversations.data?.items ?? []).map((c) => (
            <button
              key={c.id}
              onClick={() => openConv(c.id)}
              className="card card-interactive"
              style={{
                padding: 12,
                display: "flex",
                gap: 10,
                alignItems: "center",
                textAlign: "left",
                cursor: "pointer",
                borderColor: activeId === c.id ? "var(--brand)" : undefined,
                background: activeId === c.id ? "var(--brand-soft)" : undefined,
              }}
            >
              <Avatar name={c.other.display_name} avatarKey={c.other.avatar_key} size={38} />
              <div style={{ flex: 1, minWidth: 0 }}>
                <div style={{ display: "flex", gap: 5, alignItems: "center" }}>
                  <b style={{ fontSize: 13.5, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                    {c.other.display_name}
                  </b>
                  {c.pinned && <IconPin size={12} style={{ color: "var(--brand)", flexShrink: 0 }} />}
                  {c.muted && <IconBellOff size={12} style={{ color: "var(--text-3)", flexShrink: 0 }} />}
                  <span className="tabular-nums" style={{ marginLeft: "auto", fontSize: 11.5, color: "var(--text-3)", flexShrink: 0 }}>
                    {c.last_message_at ? timeAgo(c.last_message_at) : ""}
                  </span>
                </div>
                <div style={{ fontSize: 12.5, color: "var(--text-3)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", marginTop: 2 }}>
                  {c.last_sender_me && "我："}
                  {c.last_message || "（暂无消息）"}
                </div>
              </div>
              {c.unread > 0 && (
                <span
                  className="tabular-nums"
                  style={{
                    background: "var(--danger)",
                    color: "#fff",
                    borderRadius: 999,
                    fontSize: 11,
                    minWidth: 18,
                    height: 18,
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "center",
                    padding: "0 5px",
                    fontWeight: 700,
                    flexShrink: 0,
                  }}
                >
                  {c.unread > 99 ? "99+" : c.unread}
                </span>
              )}
            </button>
          ))}
        </aside>

        {/* 聊天面板 */}
        <section className={!mobileChat ? "dm-pane-hidden-mobile" : undefined}>
          {active ? (
            <ChatPanel conv={active} onBack={() => setMobileChat(false)} />
          ) : (
            <div className="card" style={{ padding: 64, textAlign: "center", color: "var(--text-3)", fontSize: 13.5 }}>
              从左侧选择一个会话开始聊天
            </div>
          )}
        </section>
      </div>
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
