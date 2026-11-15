"use client";

// 全站头部：主导航、搜索框、发布按钮、通知铃铛、用户菜单。
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { useEffect } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useMe, useUnreadCount } from "@/lib/auth";
import { api, API_BASE } from "@/lib/api";
import { useQuery } from "@tanstack/react-query";
import { toast } from "@/lib/toast";
import { Avatar } from "@/components/ui/Avatar";
import { useInvalidateMe } from "@/lib/auth";

// SSE 实时通知（断线自动重连由 EventSource 内建；消息本体以接口补拉为准）
function useNotificationStream(enabled: boolean) {
  const qc = useQueryClient();
  useEffect(() => {
    if (!enabled || typeof window === "undefined" || !window.EventSource) return;
    const es = new EventSource(`${API_BASE}/me/notifications/stream`, { withCredentials: true });
    es.addEventListener("notification", (e) => {
      try {
        const data = JSON.parse((e as MessageEvent).data);
        toast("info", data.title || "收到新通知");
      } catch { /* 忽略解析失败 */ }
      qc.invalidateQueries({ queryKey: ["notifications"] });
      qc.invalidateQueries({ queryKey: ["dm"] });
    });
    return () => es.close();
  }, [enabled, qc]);
}

function useDmUnread(enabled: boolean) {
  return useQuery({
    queryKey: ["dm", "unread"],
    enabled,
    refetchInterval: 30_000,
    queryFn: () => api.get<{ unread: number }>("/me/conversations/unread-count"),
  });
}

export function SiteHeader() {
  const router = useRouter();
  const { data: me } = useMe();
  const { data: unread } = useUnreadCount(!!me?.authenticated);
  const { data: dmUnread } = useDmUnread(!!me?.authenticated);
  useNotificationStream(!!me?.authenticated);
  const invalidate = useInvalidateMe();
  const [q, setQ] = useState("");
  const [menuOpen, setMenuOpen] = useState(false);
  const user = me?.authenticated ? me.user : null;

  const logout = async () => {
    try {
      await api.post("/auth/logout");
      invalidate();
      toast("success", "已退出登录");
      router.push("/");
    } catch {
      toast("error", "退出失败，请重试");
    }
  };

  return (
    <header
      style={{
        background: "var(--bg-card)",
        borderBottom: "1px solid var(--border)",
        position: "sticky",
        top: 0,
        zIndex: 100,
      }}
    >
      <div className="site-container" style={{ height: 56, display: "flex", alignItems: "center", gap: 16 }}>
        <Link href="/" style={{ fontWeight: 800, fontSize: 18, color: "var(--brand)", flexShrink: 0 }}>
          研记社区
        </Link>
        <nav style={{ display: "flex", gap: 4, alignItems: "center" }} aria-label="主导航">
          <Link href="/" className="btn btn-text">首页</Link>
          <Link href="/categories" className="btn btn-text">版块</Link>
          <Link href="/activities" className="btn btn-text">活动</Link>
          <Link href="/help" className="btn btn-text">规则</Link>
        </nav>
        <form
          role="search"
          onSubmit={(e) => {
            e.preventDefault();
            if (q.trim()) router.push(`/search?q=${encodeURIComponent(q.trim())}`);
          }}
          style={{ flex: 1, maxWidth: 320, marginLeft: "auto" }}
        >
          <input
            className="input"
            placeholder="搜索帖子 / 用户 / 版块…"
            value={q}
            onChange={(e) => setQ(e.target.value)}
            aria-label="搜索"
            style={{ padding: "7px 12px" }}
          />
        </form>
        {user ? (
          <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
            <Link href="/compose" className="btn btn-primary btn-sm">
              发布
            </Link>
            <Link
              href="/messages"
              aria-label={`私信${dmUnread?.unread ? `（${dmUnread.unread} 条未读）` : ""}`}
              style={{ position: "relative", padding: 6, borderRadius: 8, marginLeft: 2 }}
            >
              <span aria-hidden>💬</span>
              {!!dmUnread?.unread && (
                <span
                  style={{
                    position: "absolute", top: 0, right: -2, background: "var(--danger)", color: "#fff",
                    borderRadius: 999, fontSize: 10, minWidth: 16, height: 16, display: "flex",
                    alignItems: "center", justifyContent: "center", padding: "0 4px", fontWeight: 700,
                  }}
                >
                  {dmUnread.unread > 99 ? "99+" : dmUnread.unread}
                </span>
              )}
            </Link>
            <Link
              href="/notifications"
              aria-label={`通知${unread?.unread ? `（${unread.unread} 条未读）` : ""}`}
              style={{ position: "relative", padding: 6, borderRadius: 8 }}
            >
              <span aria-hidden>🔔</span>
              {!!unread?.unread && (
                <span
                  style={{
                    position: "absolute",
                    top: 0,
                    right: -2,
                    background: "var(--danger)",
                    color: "#fff",
                    borderRadius: 999,
                    fontSize: 10,
                    minWidth: 16,
                    height: 16,
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "center",
                    padding: "0 4px",
                    fontWeight: 700,
                  }}
                >
                  {unread.unread > 99 ? "99+" : unread.unread}
                </span>
              )}
            </Link>
            <div style={{ position: "relative" }}>
              <button
                onClick={() => setMenuOpen((v) => !v)}
                aria-expanded={menuOpen}
                aria-haspopup="menu"
                style={{ background: "none", border: "none", cursor: "pointer", display: "flex" }}
              >
                <Avatar name={user.username} size={32} />
              </button>
              {menuOpen && (
                <div
                  role="menu"
                  className="card"
                  style={{
                    position: "absolute",
                    right: 0,
                    top: 40,
                    minWidth: 160,
                    padding: 6,
                    boxShadow: "var(--shadow-pop)",
                  }}
                  onMouseLeave={() => setMenuOpen(false)}
                >
                  <Link href={`/u/${user.username}`} role="menuitem" className="btn btn-text" style={{ width: "100%", justifyContent: "flex-start" }}>
                    我的主页
                  </Link>
                  <Link href="/me/posts" role="menuitem" className="btn btn-text" style={{ width: "100%", justifyContent: "flex-start" }}>
                    我的内容
                  </Link>
                  <Link href="/me/bookmarks" role="menuitem" className="btn btn-text" style={{ width: "100%", justifyContent: "flex-start" }}>
                    我的收藏
                  </Link>
                  <Link href="/me/points" role="menuitem" className="btn btn-text" style={{ width: "100%", justifyContent: "flex-start" }}>
                    积分中心
                  </Link>
                  <Link href="/activities" role="menuitem" className="btn btn-text" style={{ width: "100%", justifyContent: "flex-start" }}>
                    社区活动
                  </Link>
                  <Link href="/settings" role="menuitem" className="btn btn-text" style={{ width: "100%", justifyContent: "flex-start" }}>
                    账号设置
                  </Link>
                  {user.email_verified && (
                    <Link href="/admin" role="menuitem" className="btn btn-text" style={{ width: "100%", justifyContent: "flex-start" }}>
                      管理后台
                    </Link>
                  )}
                  <button role="menuitem" onClick={logout} className="btn btn-text" style={{ width: "100%", justifyContent: "flex-start", color: "var(--danger)" }}>
                    退出登录
                  </button>
                </div>
              )}
            </div>
          </div>
        ) : (
          <div style={{ display: "flex", gap: 8 }}>
            <Link href="/login" className="btn btn-secondary btn-sm">
              登录
            </Link>
            <Link href="/register" className="btn btn-primary btn-sm">
              注册
            </Link>
          </div>
        )}
      </div>
    </header>
  );
}

export function SiteFooter() {
  return (
    <footer style={{ borderTop: "1px solid var(--border)", marginTop: 48, padding: "24px 0", color: "var(--text-3)", fontSize: 13 }}>
      <div className="site-container" style={{ display: "flex", gap: 24, flexWrap: "wrap", justifyContent: "space-between" }}>
        <div>研记社区 —— 开放注册的综合社区论坛</div>
        <div style={{ display: "flex", gap: 16 }}>
          <Link href="/help">社区规则</Link>
          <Link href="/help#roadmap">功能规划</Link>
        </div>
      </div>
    </footer>
  );
}
