"use client";

// 全站头部（磨砂顶栏、实时搜索联想 & Ctrl+K、主题切换、通知/私信、用户菜单）、页脚与移动端底部导航。
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useInvalidateMe, useMe, useUnreadCount } from "@/lib/auth";
import { api, API_BASE, fmtCount, type PageData } from "@/lib/api";
import type { SearchPostResult } from "@/lib/types";
import { toast } from "@/lib/toast";
import { Avatar } from "@/components/ui/Avatar";
import {
  IconArrowUp,
  IconAward,
  IconBell,
  IconBookmark,
  IconFileText,
  IconGrid,
  IconHome,
  IconLogOut,
  IconMessage,
  IconMoon,
  IconPlus,
  IconSearch,
  IconSettings,
  IconShield,
  IconSun,
  IconUser,
  IconX,
} from "@/components/ui/Icons";

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
      } catch {
        /* 忽略解析失败 */
      }
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

function ThemeToggle() {
  const [dark, setDark] = useState(false);

  useEffect(() => {
    const sync = () => {
      const attr = document.documentElement.getAttribute("data-theme");
      if (attr === "dark") setDark(true);
      else if (attr === "light") setDark(false);
      else setDark(window.matchMedia("(prefers-color-scheme: dark)").matches);
    };
    sync();
  }, []);

  const toggle = () => {
    const next = dark ? "light" : "dark";
    document.documentElement.setAttribute("data-theme", next);
    try {
      localStorage.setItem("yanji-theme", next);
    } catch {
      /* 忽略存储异常 */
    }
    setDark(!dark);
  };

  return (
    <button
      type="button"
      className="btn btn-icon"
      onClick={toggle}
      title={dark ? "切换到浅色模式" : "切换到深色模式"}
      aria-label={dark ? "切换到浅色模式" : "切换到深色模式"}
    >
      {dark ? <IconSun size={17} /> : <IconMoon size={17} />}
    </button>
  );
}

export function SiteHeader() {
  const router = useRouter();
  const pathname = usePathname();
  const { data: me } = useMe();
  const { data: unread } = useUnreadCount(!!me?.authenticated);
  const { data: dmUnread } = useDmUnread(!!me?.authenticated);
  useNotificationStream(!!me?.authenticated);
  const invalidate = useInvalidateMe();

  const [q, setQ] = useState("");
  const [debouncedQ, setDebouncedQ] = useState("");
  const [searchFocused, setSearchFocused] = useState(false);
  const [menuOpen, setMenuOpen] = useState(false);

  const searchRef = useRef<HTMLInputElement>(null);
  const searchBoxRef = useRef<HTMLDivElement>(null);
  const menuBoxRef = useRef<HTMLDivElement>(null);

  const user = me?.authenticated ? me.user : null;

  // 防抖即时联想搜索
  useEffect(() => {
    const t = setTimeout(() => setDebouncedQ(q.trim()), 220);
    return () => clearTimeout(t);
  }, [q]);

  const quickPosts = useQuery({
    enabled: searchFocused && debouncedQ.length >= 1,
    queryKey: ["quick-search", "post", debouncedQ],
    queryFn: () =>
      api.get<PageData<SearchPostResult>>(
        `/search?q=${encodeURIComponent(debouncedQ)}&type=post&page=1&page_size=5`,
      ),
    staleTime: 30_000,
  });

  const quickUsers = useQuery({
    enabled: searchFocused && debouncedQ.length >= 1,
    queryKey: ["quick-search", "user", debouncedQ],
    queryFn: () =>
      api.get<PageData<{ id: string; username: string; display_name: string; avatar_key: string }>>(
        `/search?q=${encodeURIComponent(debouncedQ)}&type=user&page=1&page_size=3`,
      ),
    staleTime: 30_000,
  });

  // 全局快捷键：Ctrl/Cmd + K 或 / 聚焦搜索；Esc 收起浮层
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        searchRef.current?.focus();
        setSearchFocused(true);
        return;
      }
      if (e.key === "/" && !["INPUT", "TEXTAREA", "SELECT"].includes((document.activeElement?.tagName ?? ""))) {
        e.preventDefault();
        searchRef.current?.focus();
        setSearchFocused(true);
        return;
      }
      if (e.key === "Escape") {
        setSearchFocused(false);
        setMenuOpen(false);
        searchRef.current?.blur();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  // 点击外部收起下拉浮层
  useEffect(() => {
    const onClick = (e: MouseEvent) => {
      const target = e.target as Node;
      if (searchBoxRef.current && !searchBoxRef.current.contains(target)) {
        setSearchFocused(false);
      }
      if (menuBoxRef.current && !menuBoxRef.current.contains(target)) {
        setMenuOpen(false);
      }
    };
    document.addEventListener("mousedown", onClick);
    return () => document.removeEventListener("mousedown", onClick);
  }, []);

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

  const navItems = [
    { href: "/", label: "首页", match: (p: string) => p === "/" },
    { href: "/categories", label: "版块", match: (p: string) => p.startsWith("/categories") || p.startsWith("/c/") },
    { href: "/activities", label: "活动", match: (p: string) => p.startsWith("/activities") },
    { href: "/help", label: "规则", match: (p: string) => p.startsWith("/help") },
  ];

  const showQuick = searchFocused && debouncedQ.length >= 1;

  return (
    <header
      className="surface-glass"
      style={{
        borderBottom: "1px solid var(--border)",
        position: "sticky",
        top: 0,
        zIndex: 100,
      }}
    >
      <div className="site-container" style={{ height: 60, display: "flex", alignItems: "center", gap: 16 }}>
        <Link
          href="/"
          style={{
            display: "inline-flex",
            alignItems: "center",
            gap: 9,
            fontWeight: 750,
            fontSize: 16.5,
            letterSpacing: "-0.025em",
            color: "var(--text-1)",
            flexShrink: 0,
          }}
        >
          <span className="brand-mark" aria-hidden>
            研
          </span>
          <span>研记社区</span>
        </Link>

        <nav className="hide-mobile" style={{ display: "flex", gap: 3, alignItems: "center" }} aria-label="主导航">
          {navItems.map((n) => {
            const active = n.match(pathname);
            return (
              <Link
                key={n.href}
                href={n.href}
                className="btn btn-text btn-sm"
                style={{
                  color: active ? "var(--text-1)" : "var(--text-2)",
                  fontWeight: active ? 620 : 500,
                  background: active ? "var(--bg-inset)" : undefined,
                  border: active ? "1px solid var(--border)" : "1px solid transparent",
                  padding: "6.5px 12px",
                }}
              >
                {n.label}
              </Link>
            );
          })}
        </nav>

        {/* 即时搜索框 */}
        <div ref={searchBoxRef} style={{ position: "relative", flex: 1, maxWidth: 356, marginLeft: "auto" }}>
          <form
            role="search"
            onSubmit={(e) => {
              e.preventDefault();
              if (q.trim()) {
                setSearchFocused(false);
                router.push(`/search?q=${encodeURIComponent(q.trim())}`);
              }
            }}
            style={{ position: "relative", display: "flex", alignItems: "center" }}
          >
            <span
              style={{
                position: "absolute",
                left: 11,
                color: searchFocused ? "var(--brand)" : "var(--text-3)",
                display: "inline-flex",
                pointerEvents: "none",
                transition: "color 0.15s ease",
              }}
            >
              <IconSearch size={15} />
            </span>
            <input
              ref={searchRef}
              className="input"
              placeholder="搜索帖子、用户、版块…"
              value={q}
              onFocus={() => setSearchFocused(true)}
              onChange={(e) => setQ(e.target.value)}
              aria-label="搜索"
              style={{
                padding: "7.5px 56px 7.5px 34px",
                fontSize: 13.5,
                background: searchFocused ? "var(--bg-card)" : "var(--bg-inset)",
                borderColor: searchFocused ? "var(--brand)" : "var(--border)",
                borderRadius: 10,
              }}
            />
            {q ? (
              <button
                type="button"
                onClick={() => setQ("")}
                aria-label="清空搜索"
                style={{
                  position: "absolute",
                  right: 9,
                  background: "none",
                  border: "none",
                  color: "var(--text-3)",
                  cursor: "pointer",
                  display: "inline-flex",
                  padding: 2,
                }}
              >
                <IconX size={14} />
              </button>
            ) : (
              <kbd
                className="hide-tablet"
                style={{
                  position: "absolute",
                  right: 8,
                  fontSize: 11,
                  fontWeight: 550,
                  color: "var(--text-3)",
                  background: "var(--bg-card)",
                  border: "1px solid var(--border)",
                  borderRadius: 6,
                  padding: "1.5px 6px",
                  fontFamily: "inherit",
                  pointerEvents: "none",
                  boxShadow: "0 1px 1px rgba(15, 23, 42, 0.04)",
                }}
              >
                ⌘K
              </kbd>
            )}
          </form>

          {showQuick && (
            <div
              className="card modal-panel"
              style={{
                position: "absolute",
                top: "calc(100% + 8px)",
                left: 0,
                right: 0,
                minWidth: 300,
                padding: 8,
                boxShadow: "var(--shadow-pop)",
                zIndex: 200,
              }}
            >
              {quickPosts.isLoading && (
                <div style={{ padding: 10, fontSize: 13, color: "var(--text-3)" }}>搜索中…</div>
              )}
              {!quickPosts.isLoading &&
                (quickPosts.data?.items.length ?? 0) === 0 &&
                (quickUsers.data?.items.length ?? 0) === 0 && (
                  <div style={{ padding: 12, fontSize: 13, color: "var(--text-3)", textAlign: "center" }}>
                    未找到直接匹配内容
                  </div>
                )}

              {(quickPosts.data?.items ?? []).length > 0 && (
                <div>
                  <div style={{ fontSize: 11, fontWeight: 650, color: "var(--text-3)", padding: "4px 10px", letterSpacing: "0.04em" }}>
                    相关帖子
                  </div>
                  {quickPosts.data!.items.map((it) => (
                    <Link
                      key={it.id}
                      href={`/p/${it.id}`}
                      onClick={() => setSearchFocused(false)}
                      className="rail-link"
                      style={{ flexDirection: "column", alignItems: "flex-start", gap: 2 }}
                    >
                      <div style={{ fontWeight: 550, maxWidth: "100%", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                        {it.title}
                      </div>
                      <div style={{ fontSize: 11.5, color: "var(--text-3)" }}>
                        {it.category_name} · {it.author_name}
                      </div>
                    </Link>
                  ))}
                </div>
              )}

              {(quickUsers.data?.items ?? []).length > 0 && (
                <div style={{ marginTop: 4 }}>
                  <div style={{ fontSize: 11, fontWeight: 650, color: "var(--text-3)", padding: "4px 10px", letterSpacing: "0.04em" }}>
                    成员
                  </div>
                  {quickUsers.data!.items.map((u) => (
                    <Link
                      key={u.id}
                      href={`/u/${u.username}`}
                      onClick={() => setSearchFocused(false)}
                      className="rail-link"
                      style={{ justifyContent: "flex-start", gap: 8 }}
                    >
                      <Avatar name={u.display_name || u.username} avatarKey={u.avatar_key} size={24} />
                      <span style={{ fontWeight: 550 }}>{u.display_name}</span>
                      <span style={{ color: "var(--text-3)", fontSize: 12 }}>@{u.username}</span>
                    </Link>
                  ))}
                </div>
              )}

              <Link
                href={`/search?q=${encodeURIComponent(debouncedQ)}`}
                onClick={() => setSearchFocused(false)}
                style={{
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "space-between",
                  padding: "8px 10px",
                  marginTop: 4,
                  borderTop: "1px solid var(--border)",
                  fontSize: 12.5,
                  color: "var(--brand)",
                  fontWeight: 580,
                }}
              >
                <span>查看「{debouncedQ}」全部结果</span>
                <span>Enter →</span>
              </Link>
            </div>
          )}
        </div>

        <ThemeToggle />

        {user ? (
          <div style={{ display: "flex", alignItems: "center", gap: 6, flexShrink: 0 }}>
            <Link href="/compose" className="btn btn-brand btn-sm hide-mobile">
              <IconPlus size={15} />
              <span>发布</span>
            </Link>

            <Link
              href="/messages"
              className="btn btn-icon"
              aria-label={`私信${dmUnread?.unread ? `（${dmUnread.unread} 条未读）` : ""}`}
              title="私信"
              style={{ position: "relative" }}
            >
              <IconMessage size={17} />
              {!!dmUnread?.unread && (
                <span
                  className="tabular-nums"
                  style={{
                    position: "absolute",
                    top: 2,
                    right: 1,
                    background: "var(--danger)",
                    color: "#fff",
                    borderRadius: 999,
                    fontSize: 10,
                    minWidth: 15,
                    height: 15,
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "center",
                    padding: "0 4px",
                    fontWeight: 700,
                    boxShadow: "0 0 0 2px var(--bg-card)",
                  }}
                >
                  {dmUnread.unread > 99 ? "99+" : fmtCount(dmUnread.unread)}
                </span>
              )}
            </Link>

            <Link
              href="/notifications"
              className="btn btn-icon"
              aria-label={`通知${unread?.unread ? `（${unread.unread} 条未读）` : ""}`}
              title="通知"
              style={{ position: "relative" }}
            >
              <IconBell size={17} />
              {!!unread?.unread && (
                <span
                  className="tabular-nums"
                  style={{
                    position: "absolute",
                    top: 2,
                    right: 1,
                    background: "var(--danger)",
                    color: "#fff",
                    borderRadius: 999,
                    fontSize: 10,
                    minWidth: 15,
                    height: 15,
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "center",
                    padding: "0 4px",
                    fontWeight: 700,
                    boxShadow: "0 0 0 2px var(--bg-card)",
                  }}
                >
                  {unread.unread > 99 ? "99+" : fmtCount(unread.unread)}
                </span>
              )}
            </Link>

            <div ref={menuBoxRef} style={{ position: "relative" }}>
              <button
                type="button"
                onClick={() => setMenuOpen((v) => !v)}
                aria-expanded={menuOpen}
                aria-haspopup="menu"
                aria-label="用户菜单"
                style={{
                  background: "none",
                  border: menuOpen ? "2px solid var(--brand)" : "2px solid transparent",
                  cursor: "pointer",
                  display: "flex",
                  padding: 1,
                  borderRadius: 999,
                  transition: "border-color 0.15s ease",
                }}
              >
                <Avatar name={user.username} size={32} />
              </button>
              {menuOpen && (
                <div
                  role="menu"
                  className="card modal-panel"
                  style={{
                    position: "absolute",
                    right: 0,
                    top: 44,
                    minWidth: 212,
                    padding: 6,
                    boxShadow: "var(--shadow-pop)",
                    zIndex: 210,
                  }}
                  onClick={() => setMenuOpen(false)}
                >
                  <div style={{ padding: "9px 10px 10px", borderBottom: "1px solid var(--border)", marginBottom: 4 }}>
                    <div style={{ fontSize: 14, fontWeight: 650 }}>{user.username}</div>
                    <div style={{ fontSize: 12, color: "var(--text-3)", overflow: "hidden", textOverflow: "ellipsis", marginTop: 1 }}>
                      {user.email}
                    </div>
                  </div>
                  <Link href={`/u/${user.username}`} role="menuitem" className="btn btn-text btn-sm" style={{ width: "100%", justifyContent: "flex-start" }}>
                    <IconUser size={15} /> 我的主页
                  </Link>
                  <Link href="/me/posts" role="menuitem" className="btn btn-text btn-sm" style={{ width: "100%", justifyContent: "flex-start" }}>
                    <IconFileText size={15} /> 我的内容
                  </Link>
                  <Link href="/me/bookmarks" role="menuitem" className="btn btn-text btn-sm" style={{ width: "100%", justifyContent: "flex-start" }}>
                    <IconBookmark size={15} /> 我的收藏
                  </Link>
                  <Link href="/me/points" role="menuitem" className="btn btn-text btn-sm" style={{ width: "100%", justifyContent: "flex-start" }}>
                    <IconAward size={15} /> 积分与签到
                  </Link>
                  <Link href="/me/appeals" role="menuitem" className="btn btn-text btn-sm" style={{ width: "100%", justifyContent: "flex-start" }}>
                    <IconShield size={15} /> 申诉中心
                  </Link>
                  <Link href="/settings" role="menuitem" className="btn btn-text btn-sm" style={{ width: "100%", justifyContent: "flex-start" }}>
                    <IconSettings size={15} /> 账号设置
                  </Link>
                  {user.can_admin && (
                    <Link href="/admin" role="menuitem" className="btn btn-text btn-sm" style={{ width: "100%", justifyContent: "flex-start", color: "var(--brand)" }}>
                      <IconGrid size={15} /> 管理后台
                    </Link>
                  )}
                  <div style={{ borderTop: "1px solid var(--border)", marginTop: 4, paddingTop: 4 }}>
                    <button
                      role="menuitem"
                      onClick={logout}
                      className="btn btn-text btn-sm"
                      style={{ width: "100%", justifyContent: "flex-start", color: "var(--danger)" }}
                    >
                      <IconLogOut size={15} /> 退出登录
                    </button>
                  </div>
                </div>
              )}
            </div>
          </div>
        ) : (
          <div style={{ display: "flex", gap: 8, flexShrink: 0 }}>
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
    <footer
      style={{
        borderTop: "1px solid var(--border)",
        marginTop: 64,
        padding: "32px 0",
        background: "var(--bg-subtle)",
        color: "var(--text-3)",
        fontSize: 13,
      }}
    >
      <div
        className="site-container"
        style={{
          display: "flex",
          gap: 24,
          flexWrap: "wrap",
          justifyContent: "space-between",
          alignItems: "center",
        }}
      >
        <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
          <span className="brand-mark" style={{ width: 24, height: 24, fontSize: 12, borderRadius: 7 }} aria-hidden>
            研
          </span>
          <div>
            <div style={{ fontWeight: 650, color: "var(--text-1)", fontSize: 13.5 }}>研记社区</div>
            <div style={{ fontSize: 12, color: "var(--text-3)" }}>理性交流 · 深度沉淀 · 共同成长</div>
          </div>
        </div>
        <div style={{ display: "flex", gap: 20, flexWrap: "wrap" }}>
          <Link href="/categories" style={{ transition: "color 0.15s" }}>版块导航</Link>
          <Link href="/activities" style={{ transition: "color 0.15s" }}>社区活动</Link>
          <Link href="/help" style={{ transition: "color 0.15s" }}>社区规则</Link>
          <Link href="/help#roadmap" style={{ transition: "color 0.15s" }}>后续规划</Link>
        </div>
      </div>
    </footer>
  );
}

/** 移动端底部导航（<640px 生效） */
export function MobileBottomNav() {
  const pathname = usePathname();
  const { data: me } = useMe();
  const { data: dmUnread } = useDmUnread(!!me?.authenticated);
  const user = me?.authenticated ? me.user : null;

  return (
    <nav className="mobile-bottom-nav" aria-label="移动端底部导航">
      <Link href="/" style={{ color: pathname === "/" ? "var(--brand)" : undefined }}>
        <IconHome size={19} />
        <span>首页</span>
      </Link>
      <Link href="/categories" style={{ color: pathname.startsWith("/categories") || pathname.startsWith("/c/") ? "var(--brand)" : undefined }}>
        <IconGrid size={19} />
        <span>版块</span>
      </Link>
      <Link href="/compose" style={{ color: "var(--brand)", fontWeight: 650 }}>
        <IconPlus size={20} />
        <span>发布</span>
      </Link>
      <Link href={user ? "/messages" : "/login"} style={{ color: pathname.startsWith("/messages") ? "var(--brand)" : undefined }}>
        <IconMessage size={19} />
        <span>消息</span>
        {!!dmUnread?.unread && (
          <span
            style={{
              position: "absolute",
              top: 4,
              right: 8,
              width: 7,
              height: 7,
              borderRadius: 999,
              background: "var(--danger)",
            }}
          />
        )}
      </Link>
      <Link href={user ? `/u/${user.username}` : "/login"} style={{ color: pathname.startsWith("/u/") ? "var(--brand)" : undefined }}>
        <IconUser size={19} />
        <span>我的</span>
      </Link>
    </nav>
  );
}

/** 返回顶部悬浮按钮 */
export function BackToTop() {
  const [visible, setVisible] = useState(false);

  useEffect(() => {
    const onScroll = () => setVisible(window.scrollY > 420);
    window.addEventListener("scroll", onScroll, { passive: true });
    return () => window.removeEventListener("scroll", onScroll);
  }, []);

  if (!visible) return null;
  return (
    <button
      type="button"
      onClick={() => window.scrollTo({ top: 0, behavior: "smooth" })}
      aria-label="返回顶部"
      title="返回顶部"
      className="card surface-glass card-interactive hide-mobile"
      style={{
        position: "fixed",
        right: 24,
        bottom: 28,
        width: 40,
        height: 40,
        borderRadius: 999,
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        color: "var(--text-2)",
        cursor: "pointer",
        zIndex: 90,
        boxShadow: "var(--shadow-pop)",
      }}
    >
      <IconArrowUp size={16} />
    </button>
  );
}
