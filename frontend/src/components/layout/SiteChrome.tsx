"use client";

// 全站头部（磨砂顶栏、Framer Motion 滑动导航胶囊、⌘K 命令面板、Radix 下拉菜单、真实头像同步）、页脚与移动端底部导航。
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import * as DropdownMenu from "@radix-ui/react-dropdown-menu";
import { AnimatePresence, motion } from "framer-motion";
import { useInvalidateMe, useMe, useUnreadCount } from "@/lib/auth";
import { api, API_BASE, fmtCount } from "@/lib/api";
import { toast } from "@/lib/toast";
import { Avatar } from "@/components/ui/Avatar";
import { CommandPalette } from "@/components/layout/CommandPalette";
import {
  IconArrowUp,
  IconAward,
  IconBell,
  IconBookmark,
  IconChevronDown,
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

function useMyMiniProfile(enabled: boolean) {
  return useQuery({
    queryKey: ["me-profile"],
    enabled,
    staleTime: 60_000,
    queryFn: () => api.get<{ profile: { display_name: string; avatar_key: string } }>("/me/profile"),
  });
}

function useThemeState() {
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

  return { dark, toggle };
}

export function SiteHeader() {
  const router = useRouter();
  const pathname = usePathname();
  const { data: me } = useMe();
  const authed = !!me?.authenticated;
  const { data: unread } = useUnreadCount(authed);
  const { data: dmUnread } = useDmUnread(authed);
  const { data: myProfile } = useMyMiniProfile(authed);
  useNotificationStream(authed);
  const invalidate = useInvalidateMe();
  const { dark, toggle: toggleTheme } = useThemeState();

  const [cmdOpen, setCmdOpen] = useState(false);

  const user = me?.authenticated ? me.user : null;
  const displayName = myProfile?.profile?.display_name || user?.username || "";
  const avatarKey = myProfile?.profile?.avatar_key || "";

  // 全局快捷键：Ctrl/Cmd + K 或 / 唤起命令面板
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        setCmdOpen((v) => !v);
        return;
      }
      if (
        e.key === "/" &&
        !["INPUT", "TEXTAREA", "SELECT"].includes(document.activeElement?.tagName ?? "") &&
        !(document.activeElement as HTMLElement | null)?.isContentEditable
      ) {
        e.preventDefault();
        setCmdOpen(true);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
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

  return (
    <>
      <header
        className="surface-glass"
        style={{
          borderBottom: "1px solid var(--border)",
          position: "sticky",
          top: 0,
          zIndex: 100,
        }}
      >
        <div className="site-container" style={{ height: 60, display: "flex", alignItems: "center", gap: 14 }}>
          <Link
            href="/"
            style={{
              display: "inline-flex",
              alignItems: "center",
              gap: 9,
              fontWeight: 760,
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

          <nav className="hide-mobile" style={{ display: "flex", gap: 2, alignItems: "center" }} aria-label="主导航">
            {navItems.map((n) => {
              const active = n.match(pathname);
              return (
                <Link
                  key={n.href}
                  href={n.href}
                  style={{
                    position: "relative",
                    padding: "6.5px 12px",
                    borderRadius: 9,
                    fontSize: 13.5,
                    fontWeight: active ? 620 : 500,
                    color: active ? "var(--text-1)" : "var(--text-2)",
                    transition: "color 0.15s ease",
                  }}
                >
                  {active && (
                    <motion.span
                      layoutId="header-nav-pill"
                      transition={{ type: "spring", stiffness: 420, damping: 32 }}
                      style={{
                        position: "absolute",
                        inset: 0,
                        borderRadius: 9,
                        background: "var(--bg-inset)",
                        border: "1px solid var(--border)",
                        zIndex: 0,
                      }}
                    />
                  )}
                  <span style={{ position: "relative", zIndex: 1 }}>{n.label}</span>
                </Link>
              );
            })}
          </nav>

          {/* 全局 ⌘K 命令面板触发胶囊 */}
          <button
            type="button"
            onClick={() => setCmdOpen(true)}
            aria-label="打开全局搜索与命令面板"
            style={{
              marginLeft: "auto",
              flex: 1,
              maxWidth: 320,
              height: 36,
              padding: "0 10px 0 12px",
              borderRadius: 10,
              border: "1px solid var(--border-strong)",
              background: "var(--bg-inset)",
              color: "var(--text-3)",
              display: "flex",
              alignItems: "center",
              gap: 8,
              cursor: "pointer",
              fontSize: 13,
              transition: "border-color 0.16s ease, background-color 0.16s ease, color 0.16s ease",
            }}
          >
            <IconSearch size={14} style={{ flexShrink: 0 }} />
            <span style={{ flex: 1, textAlign: "left", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
              搜索帖子、成员或指令…
            </span>
            <kbd className="kbd-chip hide-tablet">⌘K</kbd>
          </button>

          {/* 深浅色切换（带微旋转过渡） */}
          <button
            type="button"
            className="btn btn-icon"
            onClick={toggleTheme}
            title={dark ? "切换到浅色模式" : "切换到深色模式"}
            aria-label={dark ? "切换到浅色模式" : "切换到深色模式"}
          >
            <AnimatePresence mode="wait" initial={false}>
              <motion.span
                key={dark ? "sun" : "moon"}
                initial={{ opacity: 0, rotate: -35, scale: 0.8 }}
                animate={{ opacity: 1, rotate: 0, scale: 1 }}
                exit={{ opacity: 0, rotate: 35, scale: 0.8 }}
                transition={{ duration: 0.16 }}
                style={{ display: "inline-flex" }}
              >
                {dark ? <IconSun size={17} /> : <IconMoon size={17} />}
              </motion.span>
            </AnimatePresence>
          </button>

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

              {/* Radix UI 无障碍下拉用户菜单 */}
              <DropdownMenu.Root>
                <DropdownMenu.Trigger asChild>
                  <button
                    type="button"
                    aria-label="用户菜单"
                    style={{
                      background: "var(--bg-inset)",
                      border: "1px solid var(--border)",
                      cursor: "pointer",
                      display: "inline-flex",
                      alignItems: "center",
                      gap: 5,
                      padding: "2px 7px 2px 3px",
                      borderRadius: 999,
                      color: "var(--text-2)",
                      transition: "border-color 0.15s ease, background-color 0.15s ease",
                    }}
                  >
                    <Avatar name={displayName} avatarKey={avatarKey} size={28} />
                    <IconChevronDown size={13} className="hide-mobile" />
                  </button>
                </DropdownMenu.Trigger>
                <DropdownMenu.Portal>
                  <DropdownMenu.Content align="end" sideOffset={8} collisionPadding={12} className="radix-menu-content">
                    <div style={{ padding: "8px 10px 10px", marginBottom: 2 }}>
                      <div style={{ fontSize: 14, fontWeight: 660, color: "var(--text-1)" }}>{displayName}</div>
                      <div style={{ fontSize: 12, color: "var(--text-3)", overflow: "hidden", textOverflow: "ellipsis", marginTop: 1 }}>
                        @{user.username} · {user.email}
                      </div>
                    </div>
                    <DropdownMenu.Separator className="radix-menu-sep" />
                    <DropdownMenu.Item asChild>
                      <Link href={`/u/${user.username}`} className="radix-menu-item">
                        <IconUser size={15} /> 我的主页
                      </Link>
                    </DropdownMenu.Item>
                    <DropdownMenu.Item asChild>
                      <Link href="/me/posts" className="radix-menu-item">
                        <IconFileText size={15} /> 我的内容
                      </Link>
                    </DropdownMenu.Item>
                    <DropdownMenu.Item asChild>
                      <Link href="/me/bookmarks" className="radix-menu-item">
                        <IconBookmark size={15} /> 我的收藏
                      </Link>
                    </DropdownMenu.Item>
                    <DropdownMenu.Item asChild>
                      <Link href="/me/points" className="radix-menu-item">
                        <IconAward size={15} /> 积分与签到
                      </Link>
                    </DropdownMenu.Item>
                    <DropdownMenu.Item asChild>
                      <Link href="/me/appeals" className="radix-menu-item">
                        <IconShield size={15} /> 申诉中心
                      </Link>
                    </DropdownMenu.Item>
                    <DropdownMenu.Item asChild>
                      <Link href="/settings" className="radix-menu-item">
                        <IconSettings size={15} /> 账号设置
                      </Link>
                    </DropdownMenu.Item>
                    {user.can_admin && (
                      <>
                        <DropdownMenu.Separator className="radix-menu-sep" />
                        <DropdownMenu.Item asChild>
                          <Link href="/admin" className="radix-menu-item" style={{ color: "var(--brand)", fontWeight: 600 }}>
                            <IconGrid size={15} /> 管理后台
                          </Link>
                        </DropdownMenu.Item>
                      </>
                    )}
                    <DropdownMenu.Separator className="radix-menu-sep" />
                    <DropdownMenu.Item onSelect={logout} className="radix-menu-item radix-menu-item-danger">
                      <IconLogOut size={15} /> 退出登录
                    </DropdownMenu.Item>
                  </DropdownMenu.Content>
                </DropdownMenu.Portal>
              </DropdownMenu.Root>
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

      <CommandPalette
        open={cmdOpen}
        onOpenChange={setCmdOpen}
        authenticated={authed}
        canAdmin={user?.can_admin}
        isDark={dark}
        onToggleTheme={toggleTheme}
      />
    </>
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
        <div style={{ display: "flex", alignItems: "center", gap: 11 }}>
          <span className="brand-mark" style={{ width: 26, height: 26, fontSize: 12.5, borderRadius: 8 }} aria-hidden>
            研
          </span>
          <div>
            <div style={{ fontWeight: 680, color: "var(--text-1)", fontSize: 13.5 }}>研记社区 · Yanji Community</div>
            <div style={{ fontSize: 12, color: "var(--text-3)", marginTop: 1 }}>理性交流 · 深度沉淀 · 共同成长</div>
          </div>
        </div>
        <div style={{ display: "flex", gap: 20, flexWrap: "wrap", alignItems: "center" }}>
          <Link href="/categories" style={{ transition: "color 0.15s" }}>版块导航</Link>
          <Link href="/activities" style={{ transition: "color 0.15s" }}>社区活动</Link>
          <Link href="/search" style={{ transition: "color 0.15s" }}>高级搜索</Link>
          <Link href="/help" style={{ transition: "color 0.15s" }}>社区规则</Link>
        </div>
      </div>
    </footer>
  );
}

/** 移动端底部导航（<640px 生效，中间发布按钮使用品牌渐变悬浮胶囊） */
export function MobileBottomNav() {
  const pathname = usePathname();
  const { data: me } = useMe();
  const { data: dmUnread } = useDmUnread(!!me?.authenticated);
  const { data: notifUnread } = useUnreadCount(!!me?.authenticated);
  const user = me?.authenticated ? me.user : null;
  const msgHasUnread = !!(dmUnread?.unread || notifUnread?.unread);

  return (
    <nav className="mobile-bottom-nav" aria-label="移动端底部导航">
      <Link href="/" style={{ color: pathname === "/" ? "var(--brand)" : undefined }}>
        <IconHome size={19} />
        <span>首页</span>
      </Link>
      <Link
        href="/categories"
        style={{ color: pathname.startsWith("/categories") || pathname.startsWith("/c/") ? "var(--brand)" : undefined }}
      >
        <IconGrid size={19} />
        <span>版块</span>
      </Link>
      <Link
        href="/compose"
        aria-label="发布内容"
        style={{
          width: 44,
          height: 36,
          minWidth: 44,
          minHeight: 36,
          borderRadius: 12,
          background: "var(--brand-gradient)",
          color: "#fff",
          boxShadow: "0 4px 12px rgba(59, 91, 219, 0.34)",
        }}
      >
        <IconPlus size={20} />
      </Link>
      <Link href={user ? "/messages" : "/login"} style={{ color: pathname.startsWith("/messages") ? "var(--brand)" : undefined }}>
        <IconMessage size={19} />
        <span>消息</span>
        {msgHasUnread && (
          <span
            style={{
              position: "absolute",
              top: 4,
              right: 8,
              width: 7,
              height: 7,
              borderRadius: 999,
              background: "var(--danger)",
              boxShadow: "0 0 0 2px var(--bg-card)",
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

/** 返回顶部悬浮按钮（Framer Motion 弹性出入场） */
export function BackToTop() {
  const [visible, setVisible] = useState(false);

  useEffect(() => {
    const onScroll = () => setVisible(window.scrollY > 420);
    window.addEventListener("scroll", onScroll, { passive: true });
    return () => window.removeEventListener("scroll", onScroll);
  }, []);

  return (
    <AnimatePresence>
      {visible && (
        <motion.button
          type="button"
          initial={{ opacity: 0, y: 14, scale: 0.88 }}
          animate={{ opacity: 1, y: 0, scale: 1 }}
          exit={{ opacity: 0, y: 10, scale: 0.9 }}
          whileHover={{ y: -2 }}
          whileTap={{ scale: 0.94 }}
          transition={{ type: "spring", stiffness: 420, damping: 28 }}
          onClick={() => window.scrollTo({ top: 0, behavior: "smooth" })}
          aria-label="返回顶部"
          title="返回顶部"
          className="card surface-glass hide-mobile"
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
        </motion.button>
      )}
    </AnimatePresence>
  );
}
