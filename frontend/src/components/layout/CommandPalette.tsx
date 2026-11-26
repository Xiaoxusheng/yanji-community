"use client";

// 全局 ⌘K 命令面板：基于 Radix UI Dialog + Framer Motion + Lucide。
// 支持防抖实时搜索帖子/成员、快捷页面跳转、深浅色切换与完整键盘 ↑/↓/Enter/Esc 操控。
import { useEffect, useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { useQuery } from "@tanstack/react-query";
import * as RadixDialog from "@radix-ui/react-dialog";
import { AnimatePresence, motion } from "framer-motion";
import { api, type PageData } from "@/lib/api";
import type { SearchPostResult } from "@/lib/types";
import { Avatar } from "@/components/ui/Avatar";
import {
  IconAward,
  IconCalendar,
  IconCompass,
  IconCornerDownLeft,
  IconFileText,
  IconGrid,
  IconMessage,
  IconMoon,
  IconPlus,
  IconSearch,
  IconSettings,
  IconSun,
  IconX,
} from "@/components/ui/Icons";

interface ActionItem {
  id: string;
  group: string;
  label: string;
  sub?: string;
  icon: React.ReactNode;
  onSelect: () => void;
}

export function CommandPalette({
  open,
  onOpenChange,
  authenticated,
  canAdmin,
  isDark,
  onToggleTheme,
}: {
  open: boolean;
  onOpenChange: (v: boolean) => void;
  authenticated: boolean;
  canAdmin?: boolean;
  isDark: boolean;
  onToggleTheme: () => void;
}) {
  const router = useRouter();
  const inputRef = useRef<HTMLInputElement>(null);
  const [query, setQuery] = useState("");
  const [debouncedQ, setDebouncedQ] = useState("");
  const [activeIdx, setActiveIdx] = useState(0);

  useEffect(() => {
    if (!open) {
      setQuery("");
      setDebouncedQ("");
      setActiveIdx(0);
      return;
    }
    const t = setTimeout(() => inputRef.current?.focus(), 30);
    return () => clearTimeout(t);
  }, [open]);

  useEffect(() => {
    const t = setTimeout(() => setDebouncedQ(query.trim()), 200);
    return () => clearTimeout(t);
  }, [query]);

  const quickPosts = useQuery({
    enabled: open && debouncedQ.length >= 1,
    queryKey: ["cmdk-search", "post", debouncedQ],
    queryFn: () =>
      api.get<PageData<SearchPostResult>>(
        `/search?q=${encodeURIComponent(debouncedQ)}&type=post&page=1&page_size=5`,
      ),
    staleTime: 30_000,
  });

  const quickUsers = useQuery({
    enabled: open && debouncedQ.length >= 1,
    queryKey: ["cmdk-search", "user", debouncedQ],
    queryFn: () =>
      api.get<PageData<{ id: string; username: string; display_name: string; avatar_key: string }>>(
        `/search?q=${encodeURIComponent(debouncedQ)}&type=user&page=1&page_size=3`,
      ),
    staleTime: 30_000,
  });

  const go = (href: string) => {
    onOpenChange(false);
    router.push(href);
  };

  const items: ActionItem[] = useMemo(() => {
    if (debouncedQ.length >= 1) {
      const list: ActionItem[] = [];
      for (const p of quickPosts.data?.items ?? []) {
        list.push({
          id: `post:${p.id}`,
          group: "相关帖子",
          label: p.title,
          sub: `${p.category_name} · ${p.author_name}`,
          icon: <IconFileText size={15} style={{ color: "var(--brand)" }} />,
          onSelect: () => go(`/p/${p.id}`),
        });
      }
      for (const u of quickUsers.data?.items ?? []) {
        list.push({
          id: `user:${u.id}`,
          group: "成员",
          label: u.display_name || u.username,
          sub: `@${u.username}`,
          icon: <Avatar name={u.display_name || u.username} avatarKey={u.avatar_key} size={22} />,
          onSelect: () => go(`/u/${u.username}`),
        });
      }
      list.push({
        id: "search-all",
        group: "深度检索",
        label: `搜索「${debouncedQ}」的全部结果`,
        sub: "前往高级筛选搜索页",
        icon: <IconSearch size={15} style={{ color: "var(--brand)" }} />,
        onSelect: () => go(`/search?q=${encodeURIComponent(debouncedQ)}`),
      });
      return list;
    }

    const shortcuts: ActionItem[] = [
      {
        id: "nav-compose",
        group: "快速操作",
        label: authenticated ? "发布新话题" : "登录 / 注册账号",
        sub: authenticated ? "支持讨论、长文、问答与投票" : "加入社区参与讨论与互动",
        icon: <IconPlus size={15} style={{ color: "var(--brand)" }} />,
        onSelect: () => go(authenticated ? "/compose" : "/login"),
      },
      {
        id: "nav-theme",
        group: "快速操作",
        label: isDark ? "切换到浅色外观" : "切换到深色外观",
        sub: "即刻切换全站视觉主题",
        icon: isDark ? <IconSun size={15} /> : <IconMoon size={15} />,
        onSelect: () => {
          onToggleTheme();
          onOpenChange(false);
        },
      },
      {
        id: "nav-categories",
        group: "页面导航",
        label: "浏览全部版块",
        sub: "发现感兴趣的主题讨论区",
        icon: <IconCompass size={15} />,
        onSelect: () => go("/categories"),
      },
      {
        id: "nav-activities",
        group: "页面导航",
        label: "社区活动",
        sub: "线上分享与线下聚会报名",
        icon: <IconCalendar size={15} />,
        onSelect: () => go("/activities"),
      },
    ];

    if (authenticated) {
      shortcuts.push(
        {
          id: "nav-points",
          group: "个人中心",
          label: "积分与每日签到",
          sub: "查看等级进度与勋章成就",
          icon: <IconAward size={15} />,
          onSelect: () => go("/me/points"),
        },
        {
          id: "nav-messages",
          group: "个人中心",
          label: "私信会话",
          sub: "与社区成员一对一沟通",
          icon: <IconMessage size={15} />,
          onSelect: () => go("/messages"),
        },
        {
          id: "nav-settings",
          group: "个人中心",
          label: "账号与安全设置",
          sub: "资料、隐私、密码与登录设备",
          icon: <IconSettings size={15} />,
          onSelect: () => go("/settings"),
        },
      );
      if (canAdmin) {
        shortcuts.push({
          id: "nav-admin",
          group: "管理治理",
          label: "进入管理后台",
          sub: "审核内容、治理举报与站点配置",
          icon: <IconGrid size={15} style={{ color: "var(--brand)" }} />,
          onSelect: () => go("/admin"),
        });
      }
    }
    return shortcuts;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [debouncedQ, quickPosts.data, quickUsers.data, authenticated, canAdmin, isDark]);

  useEffect(() => {
    setActiveIdx(0);
  }, [debouncedQ, items.length]);

  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setActiveIdx((i) => (items.length ? (i + 1) % items.length : 0));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setActiveIdx((i) => (items.length ? (i - 1 + items.length) % items.length : 0));
    } else if (e.key === "Enter" && items[activeIdx]) {
      e.preventDefault();
      items[activeIdx].onSelect();
    }
  };

  // 按 group 分组展示，保持线性索引用于键盘导航
  const groups = useMemo(() => {
    const map: { name: string; entries: { item: ActionItem; flatIndex: number }[] }[] = [];
    items.forEach((item, idx) => {
      let g = map.find((x) => x.name === item.group);
      if (!g) {
        g = { name: item.group, entries: [] };
        map.push(g);
      }
      g.entries.push({ item, flatIndex: idx });
    });
    return map;
  }, [items]);

  return (
    <RadixDialog.Root open={open} onOpenChange={onOpenChange}>
      <AnimatePresence>
        {open && (
          <RadixDialog.Portal forceMount>
            <RadixDialog.Overlay asChild>
              <motion.div
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                exit={{ opacity: 0 }}
                transition={{ duration: 0.16 }}
                style={{
                  position: "fixed",
                  inset: 0,
                  background: "rgba(9, 11, 16, 0.56)",
                  backdropFilter: "blur(8px)",
                  WebkitBackdropFilter: "blur(8px)",
                  zIndex: 2200,
                }}
              />
            </RadixDialog.Overlay>
            <RadixDialog.Content asChild aria-describedby={undefined}>
              <div
                style={{
                  position: "fixed",
                  inset: 0,
                  display: "flex",
                  alignItems: "flex-start",
                  justifyContent: "center",
                  paddingTop: "12vh",
                  paddingLeft: 16,
                  paddingRight: 16,
                  zIndex: 2201,
                  pointerEvents: "none",
                }}
              >
                <motion.div
                  initial={{ opacity: 0, y: -10, scale: 0.97 }}
                  animate={{ opacity: 1, y: 0, scale: 1 }}
                  exit={{ opacity: 0, y: -6, scale: 0.98 }}
                  transition={{ type: "spring", stiffness: 460, damping: 34, mass: 0.8 }}
                  className="card"
                  onKeyDown={onKeyDown}
                  style={{
                    width: 580,
                    maxWidth: "100%",
                    overflow: "hidden",
                    borderRadius: "var(--radius-lg)",
                    boxShadow: "var(--shadow-pop)",
                    pointerEvents: "auto",
                  }}
                >
                  <RadixDialog.Title className="sr-only">全局搜索与命令面板</RadixDialog.Title>

                  {/* 顶部搜索输入栏 */}
                  <div
                    style={{
                      display: "flex",
                      alignItems: "center",
                      gap: 10,
                      padding: "13px 16px",
                      borderBottom: "1px solid var(--border)",
                    }}
                  >
                    <IconSearch size={17} style={{ color: "var(--brand)", flexShrink: 0 }} />
                    <input
                      ref={inputRef}
                      value={query}
                      onChange={(e) => setQuery(e.target.value)}
                      placeholder="搜索帖子、成员，或输入指令直达页面…"
                      aria-label="全局命令搜索"
                      style={{
                        flex: 1,
                        border: "none",
                        outline: "none",
                        background: "transparent",
                        color: "var(--text-1)",
                        fontSize: 15,
                      }}
                    />
                    {query && (
                      <button
                        type="button"
                        onClick={() => setQuery("")}
                        className="btn btn-icon"
                        style={{ width: 26, height: 26 }}
                        aria-label="清空"
                      >
                        <IconX size={14} />
                      </button>
                    )}
                    <kbd className="kbd-chip">Esc</kbd>
                  </div>

                  {/* 结果列表 */}
                  <div style={{ maxHeight: "56vh", overflowY: "auto", padding: "8px" }}>
                    {debouncedQ && quickPosts.isLoading && (
                      <div style={{ padding: "14px 12px", fontSize: 13, color: "var(--text-3)" }}>正在检索社区内容…</div>
                    )}
                    {groups.map((g) => (
                      <div key={g.name} style={{ marginBottom: 6 }}>
                        <div
                          style={{
                            fontSize: 11,
                            fontWeight: 650,
                            letterSpacing: "0.05em",
                            color: "var(--text-3)",
                            padding: "6px 10px 4px",
                          }}
                        >
                          {g.name}
                        </div>
                        {g.entries.map(({ item, flatIndex }) => {
                          const active = flatIndex === activeIdx;
                          return (
                            <button
                              key={item.id}
                              type="button"
                              onMouseEnter={() => setActiveIdx(flatIndex)}
                              onClick={item.onSelect}
                              className={`cmdk-item ${active ? "is-active" : ""}`}
                            >
                              <span
                                style={{
                                  width: 28,
                                  height: 28,
                                  borderRadius: 8,
                                  background: active ? "var(--brand-soft)" : "var(--bg-inset)",
                                  display: "inline-flex",
                                  alignItems: "center",
                                  justifyContent: "center",
                                  flexShrink: 0,
                                }}
                              >
                                {item.icon}
                              </span>
                              <span style={{ flex: 1, minWidth: 0, textAlign: "left" }}>
                                <span
                                  style={{
                                    display: "block",
                                    fontSize: 13.75,
                                    fontWeight: 560,
                                    color: "var(--text-1)",
                                    overflow: "hidden",
                                    textOverflow: "ellipsis",
                                    whiteSpace: "nowrap",
                                  }}
                                >
                                  {item.label}
                                </span>
                                {item.sub && (
                                  <span
                                    style={{
                                      display: "block",
                                      fontSize: 12,
                                      color: "var(--text-3)",
                                      overflow: "hidden",
                                      textOverflow: "ellipsis",
                                      whiteSpace: "nowrap",
                                    }}
                                  >
                                    {item.sub}
                                  </span>
                                )}
                              </span>
                              {active && <IconCornerDownLeft size={14} style={{ color: "var(--brand)", flexShrink: 0 }} />}
                            </button>
                          );
                        })}
                      </div>
                    ))}
                  </div>

                  {/* 底部快捷键指引 */}
                  <div
                    style={{
                      display: "flex",
                      alignItems: "center",
                      justifyContent: "space-between",
                      padding: "8px 14px",
                      borderTop: "1px solid var(--border)",
                      background: "var(--bg-inset)",
                      fontSize: 11.5,
                      color: "var(--text-3)",
                    }}
                  >
                    <div style={{ display: "flex", gap: 12, alignItems: "center" }}>
                      <span>
                        <kbd className="kbd-chip">↑</kbd> <kbd className="kbd-chip">↓</kbd> 选择
                      </span>
                      <span>
                        <kbd className="kbd-chip">Enter</kbd> 打开
                      </span>
                    </div>
                    <span>研记社区快捷命令</span>
                  </div>
                </motion.div>
              </div>
            </RadixDialog.Content>
          </RadixDialog.Portal>
        )}
      </AnimatePresence>
    </RadixDialog.Root>
  );
}
