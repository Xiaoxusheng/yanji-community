"use client";

// 首页：网格纹理编辑风导览横幅 + Framer Motion 弹簧滑动信息流 Tab + 可展开公告 + 实时积分签到/活跃版块/热签侧栏。
import Link from "next/link";
import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { motion } from "framer-motion";
import { api, fmtCount, timeAgo, type PageData } from "@/lib/api";
import type { PostItem, Announcement, Tag, Category, PointAccount } from "@/lib/types";
import { PostList } from "@/components/post/PostCard";
import { useMe } from "@/lib/auth";
import { toast } from "@/lib/toast";
import {
  IconAward,
  IconCheck,
  IconClock,
  IconCompass,
  IconFlame,
  IconGrid,
  IconHash,
  IconPlus,
  IconSparkles,
} from "@/components/ui/Icons";

const tabs = [
  { key: "recommended", label: "推荐", icon: IconSparkles },
  { key: "latest", label: "最新", icon: IconClock },
  { key: "hot", label: "热门", icon: IconFlame },
  { key: "featured", label: "精华", icon: IconAward },
  { key: "following", label: "关注", icon: IconCompass },
];

export default function HomePage() {
  const [tab, setTab] = useState("recommended");
  const [page, setPage] = useState(1);
  const [checkingIn, setCheckingIn] = useState(false);
  const [expandedAnnId, setExpandedAnnId] = useState<number | null>(null);
  const qc = useQueryClient();
  const { data: me } = useMe();
  const isFollowingTab = tab === "following";

  const feed = useQuery({
    queryKey: ["feed", tab, page],
    queryFn: () => api.get<PageData<PostItem>>(`/feeds?tab=${tab}&page=${page}&page_size=20`),
  });

  const announcements = useQuery({
    queryKey: ["announcements"],
    queryFn: () => api.get<{ items: Announcement[] }>("/announcements"),
    staleTime: 5 * 60_000,
  });
  const hotTags = useQuery({
    queryKey: ["tags", "hot"],
    queryFn: () => api.get<PageData<Tag>>("/tags?page_size=12"),
    staleTime: 5 * 60_000,
  });
  const categories = useQuery({
    queryKey: ["categories"],
    queryFn: () => api.get<{ items: Category[] }>("/categories"),
    staleTime: 5 * 60_000,
  });
  const pointsAcc = useQuery({
    enabled: !!me?.authenticated,
    queryKey: ["points", "account"],
    queryFn: () => api.get<{ account: PointAccount }>("/me/points"),
    staleTime: 60_000,
  });

  const checkIn = async () => {
    setCheckingIn(true);
    try {
      const res = await api.post<{ awarded: number; already: boolean; message: string }>("/me/check-in");
      if (res.already) toast("info", res.message);
      else toast("success", `签到成功 +${res.awarded} 积分`);
      qc.invalidateQueries({ queryKey: ["points"] });
    } catch (e) {
      toast("error", (e as Error).message);
    } finally {
      setCheckingIn(false);
    }
  };

  const acc = pointsAcc.data?.account;
  const allCats = categories.data?.items ?? [];
  const topCats = allCats.slice(0, 5);
  const streakPct = acc ? Math.min(100, Math.round(((acc.streak_days % 7 || (acc.streak_days >= 7 ? 7 : 0)) / 7) * 100)) : 0;

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 20 }}>
      {/* 顶部克制编辑风导览横幅（带极浅点阵纹理） */}
      <motion.section
        initial={{ opacity: 0, y: 8 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.28, ease: [0.22, 1, 0.36, 1] }}
        className="surface-hero"
        style={{ padding: "22px 24px" }}
      >
        <div
          aria-hidden
          className="bg-grid-subtle"
          style={{
            position: "absolute",
            inset: 0,
            opacity: 0.45,
            maskImage: "linear-gradient(120deg, transparent 25%, black 100%)",
            WebkitMaskImage: "linear-gradient(120deg, transparent 25%, black 100%)",
            pointerEvents: "none",
          }}
        />
        <div
          style={{
            position: "relative",
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            gap: 16,
            flexWrap: "wrap",
          }}
        >
          <div style={{ maxWidth: 640 }}>
            <div
              style={{
                display: "inline-flex",
                alignItems: "center",
                gap: 6,
                fontSize: 11.5,
                fontWeight: 650,
                letterSpacing: "0.06em",
                textTransform: "uppercase",
                color: "var(--brand)",
                background: "var(--brand-soft)",
                padding: "3px 10px",
                borderRadius: 999,
                marginBottom: 8,
              }}
            >
              <IconSparkles size={12} />
              <span>YANJI COMMUNITY</span>
            </div>
            <h1 style={{ fontSize: 22.5, fontWeight: 750, letterSpacing: "-0.028em", lineHeight: 1.3, margin: 0 }}>
              {me?.authenticated && me.user
                ? `欢迎回来，${me.user.username}`
                : "理性交流、深度思考与高价值内容沉淀"}
            </h1>
            <p style={{ fontSize: 13.75, color: "var(--text-2)", margin: "6px 0 0", lineHeight: 1.6 }}>
              涵盖技术研讨、长文专栏、问答采纳、社区投票与线下活动。在这里与同频的思考者展开对话。
            </p>
          </div>
          <div style={{ display: "flex", gap: 8, flexShrink: 0, flexWrap: "wrap" }}>
            <Link href={me?.authenticated ? "/compose" : "/register"} className="btn btn-brand btn-sm">
              <IconPlus size={14} />
              <span>{me?.authenticated ? "发起新话题" : "免费加入社区"}</span>
            </Link>
            <Link href="/categories" className="btn btn-secondary btn-sm">
              浏览全部版块
            </Link>
          </div>
        </div>

        {topCats.length > 0 && (
          <div
            style={{
              position: "relative",
              display: "flex",
              alignItems: "center",
              gap: 7,
              marginTop: 16,
              paddingTop: 14,
              borderTop: "1px solid var(--border)",
              flexWrap: "wrap",
            }}
          >
            <span style={{ fontSize: 12, color: "var(--text-3)", fontWeight: 560 }}>精选版块：</span>
            {topCats.map((c) => (
              <Link key={c.id} href={`/c/${c.slug}`} className="tag tag-outline" style={{ background: "var(--bg-card)" }}>
                <span>{c.name}</span>
                <span className="tabular-nums" style={{ color: "var(--text-3)", fontSize: 11 }}>
                  {fmtCount(c.post_count)}
                </span>
              </Link>
            ))}
            <span className="tabular-nums hide-mobile" style={{ marginLeft: "auto", fontSize: 12, color: "var(--text-3)" }}>
              {allCats.length} 个主题版块 · 按 <kbd className="kbd-chip" style={{ margin: "0 2px" }}>⌘K</kbd> 随时检索
            </span>
          </div>
        )}
      </motion.section>

      <div className="layout-main-rail">
        <div>
          <div
            style={{
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
              gap: 10,
              marginBottom: 14,
              flexWrap: "wrap",
            }}
          >
            {/* Framer Motion 弹簧滑动分段切换器 */}
            <div className="seg-tabs" role="tablist" aria-label="信息流切换" style={{ overflowX: "auto", maxWidth: "100%" }}>
              {tabs.map((t) => {
                const active = tab === t.key;
                const Icon = t.icon;
                return (
                  <button
                    key={t.key}
                    type="button"
                    role="tab"
                    aria-selected={active}
                    onClick={() => {
                      setTab(t.key);
                      setPage(1);
                    }}
                    style={{
                      position: "relative",
                      display: "inline-flex",
                      alignItems: "center",
                      gap: 5,
                      padding: "6.5px 13px",
                      borderRadius: 7,
                      border: "none",
                      background: "transparent",
                      color: active ? "var(--text-1)" : "var(--text-2)",
                      fontSize: 13,
                      fontWeight: active ? 620 : 500,
                      cursor: "pointer",
                      whiteSpace: "nowrap",
                      transition: "color 0.15s ease",
                    }}
                  >
                    {active && (
                      <motion.span
                        layoutId="home-feed-tab-pill"
                        transition={{ type: "spring", stiffness: 460, damping: 34 }}
                        style={{
                          position: "absolute",
                          inset: 0,
                          borderRadius: 7,
                          background: "var(--bg-card)",
                          boxShadow: "0 1px 3px rgba(15, 23, 42, 0.08), 0 0 0 1px var(--border)",
                          zIndex: 0,
                        }}
                      />
                    )}
                    <span style={{ position: "relative", zIndex: 1, display: "inline-flex", alignItems: "center", gap: 5 }}>
                      <Icon size={13.5} style={{ color: active ? "var(--brand)" : "var(--text-3)" }} />
                      {t.label}
                    </span>
                  </button>
                );
              })}
            </div>
            {feed.data && (
              <span className="tabular-nums hide-mobile" style={{ fontSize: 12.5, color: "var(--text-3)" }}>
                共 {fmtCount(feed.data.total)} 篇内容
              </span>
            )}
          </div>

          {isFollowingTab && !me?.authenticated ? (
            <div className="card" style={{ padding: "52px 24px", textAlign: "center" }}>
              <div style={{ fontWeight: 680, fontSize: 16, marginBottom: 6 }}>登录后查看关注动态</div>
              <p style={{ color: "var(--text-3)", fontSize: 13.5, marginBottom: 18 }}>
                关注感兴趣的成员、版块与标签，你的专属信息流将汇总在这里。
              </p>
              <Link href="/login" className="btn btn-primary btn-sm">
                立即登录
              </Link>
            </div>
          ) : (
            <PostList
              posts={feed.data?.items}
              loading={feed.isLoading}
              error={feed.error as Error | null}
              onRetry={() => feed.refetch()}
              page={page}
              pageSize={20}
              total={feed.data?.total ?? 0}
              onPageChange={setPage}
              emptyTitle={isFollowingTab ? "关注的动态还很少" : "还没有内容"}
              emptyDesc={isFollowingTab ? "去关注一些活跃成员、版块或标签吧" : "发表第一篇讨论开启话题"}
            />
          )}
        </div>

        <aside className="sticky-rail">
          {me?.authenticated ? (
            <section className="surface-spotlight" style={{ padding: 18 }}>
              <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 12 }}>
                <span style={{ fontSize: 13.5, fontWeight: 680, display: "inline-flex", alignItems: "center", gap: 6 }}>
                  <IconAward size={15} style={{ color: "var(--brand)" }} /> 每日签到
                </span>
                {acc && (
                  <Link href="/me/points" className="tabular-nums" style={{ fontSize: 12.5, color: "var(--brand)", fontWeight: 600 }}>
                    {acc.balance} 积分 →
                  </Link>
                )}
              </div>
              {acc ? (
                <>
                  <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 8, marginBottom: 10 }}>
                    <div className="stat-tile">
                      <div style={{ fontSize: 14, fontWeight: 720, color: "var(--text-1)" }}>
                        Lv.{acc.level.level}
                      </div>
                      <div style={{ fontSize: 11.5, color: "var(--text-3)", marginTop: 2 }}>{acc.level.name}</div>
                    </div>
                    <div className="stat-tile">
                      <div className="tabular-nums" style={{ fontSize: 14, fontWeight: 720, color: "var(--text-1)" }}>
                        {acc.streak_days} 天
                      </div>
                      <div style={{ fontSize: 11.5, color: "var(--text-3)", marginTop: 2 }}>连续签到</div>
                    </div>
                  </div>
                  {/* 7 日连签周期进度微条 */}
                  <div style={{ marginBottom: 12 }}>
                    <div style={{ display: "flex", justifyContent: "space-between", fontSize: 11, color: "var(--text-3)", marginBottom: 4 }}>
                      <span>七日之约勋章进度</span>
                      <span className="tabular-nums">{Math.min(acc.streak_days, 7)} / 7 天</span>
                    </div>
                    <div style={{ height: 5, borderRadius: 999, background: "var(--bg-hover)", overflow: "hidden" }}>
                      <motion.div
                        initial={{ width: 0 }}
                        animate={{ width: `${streakPct}%` }}
                        transition={{ type: "spring", stiffness: 200, damping: 24 }}
                        style={{ height: "100%", borderRadius: 999, background: "var(--brand-gradient)" }}
                      />
                    </div>
                  </div>
                  <button
                    className={`btn btn-sm ${acc.today_signed ? "btn-secondary" : "btn-brand"}`}
                    style={{ width: "100%" }}
                    disabled={checkingIn || acc.today_signed}
                    onClick={checkIn}
                  >
                    {acc.today_signed ? (
                      <>
                        <IconCheck size={14} /> 今日已签到
                      </>
                    ) : checkingIn ? (
                      "签到中…"
                    ) : (
                      "立即签到 +5 积分"
                    )}
                  </button>
                </>
              ) : (
                <div className="skeleton" style={{ height: 84 }} />
              )}
            </section>
          ) : (
            <section className="surface-spotlight" style={{ padding: 18 }}>
              <div style={{ fontSize: 15, fontWeight: 700, letterSpacing: "-0.015em", marginBottom: 6 }}>
                加入研记社区
              </div>
              <p style={{ fontSize: 13, color: "var(--text-2)", lineHeight: 1.62, margin: "0 0 14px" }}>
                参与深度讨论、问答采纳、社区投票，记录并沉淀你的每一次思考。
              </p>
              <div style={{ display: "flex", gap: 8 }}>
                <Link href="/register" className="btn btn-brand btn-sm" style={{ flex: 1 }}>
                  注册账号
                </Link>
                <Link href="/login" className="btn btn-secondary btn-sm" style={{ flex: 1 }}>
                  登录
                </Link>
              </div>
            </section>
          )}

          {/* 站点公告：支持点击展开/收起全文 */}
          <section className="card" style={{ padding: 18 }}>
            <h3 style={{ fontSize: 13.5, fontWeight: 680, marginBottom: 12, color: "var(--text-1)", display: "flex", alignItems: "center", gap: 7 }}>
              <span style={{ width: 6, height: 6, borderRadius: 999, background: "var(--brand)" }} aria-hidden />
              站点公告
            </h3>
            {announcements.isLoading ? (
              <div className="skeleton" style={{ height: 48 }} />
            ) : (announcements.data?.items.length ?? 0) === 0 ? (
              <div style={{ color: "var(--text-3)", fontSize: 13 }}>暂无新公告</div>
            ) : (
              <div style={{ display: "flex", flexDirection: "column", gap: 11 }}>
                {announcements.data!.items.slice(0, 3).map((a, idx, arr) => {
                  const expanded = expandedAnnId === a.id;
                  return (
                    <div
                      key={a.id}
                      style={{
                        paddingBottom: idx < arr.length - 1 ? 10 : 0,
                        borderBottom: idx < arr.length - 1 ? "1px solid var(--border)" : "none",
                      }}
                    >
                      <button
                        type="button"
                        onClick={() => setExpandedAnnId(expanded ? null : a.id)}
                        style={{
                          width: "100%",
                          textAlign: "left",
                          background: "none",
                          border: "none",
                          padding: 0,
                          cursor: a.body ? "pointer" : "default",
                          color: "inherit",
                        }}
                      >
                        <div style={{ fontSize: 13.5, fontWeight: 620, lineHeight: 1.4 }}>{a.title}</div>
                        {a.body && (
                          <div
                            style={{
                              fontSize: 12.5,
                              color: "var(--text-2)",
                              marginTop: 4,
                              lineHeight: 1.58,
                              whiteSpace: expanded ? "pre-wrap" : undefined,
                              ...(expanded
                                ? {}
                                : {
                                    display: "-webkit-box",
                                    WebkitLineClamp: 2,
                                    WebkitBoxOrient: "vertical",
                                    overflow: "hidden",
                                  }),
                            }}
                          >
                            {a.body}
                          </div>
                        )}
                      </button>
                      <div
                        className="tabular-nums"
                        style={{
                          fontSize: 11.5,
                          color: "var(--text-3)",
                          marginTop: 4,
                          display: "flex",
                          justifyContent: "space-between",
                        }}
                      >
                        <span>{timeAgo(a.published_at)}</span>
                        {a.body && a.body.length > 48 && (
                          <button
                            type="button"
                            onClick={() => setExpandedAnnId(expanded ? null : a.id)}
                            style={{ background: "none", border: "none", padding: 0, color: "var(--brand)", fontSize: 11.5, cursor: "pointer" }}
                          >
                            {expanded ? "收起" : "展开"}
                          </button>
                        )}
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </section>

          <section className="card" style={{ padding: 18 }}>
            <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 10 }}>
              <h3 style={{ fontSize: 13.5, fontWeight: 680, display: "inline-flex", alignItems: "center", gap: 6 }}>
                <IconGrid size={14} style={{ color: "var(--text-3)" }} /> 活跃版块
              </h3>
              <Link href="/categories" style={{ fontSize: 12.5, color: "var(--brand)", fontWeight: 560 }}>
                全部 →
              </Link>
            </div>
            <div style={{ display: "flex", flexDirection: "column", gap: 3 }}>
              {allCats.slice(0, 6).map((c) => (
                <Link key={c.id} href={`/c/${c.slug}`} className="rail-link">
                  <span style={{ fontWeight: 550 }}>{c.name}</span>
                  <span className="tag tabular-nums" style={{ fontSize: 11.5, padding: "1px 8px" }}>
                    {fmtCount(c.post_count)}
                  </span>
                </Link>
              ))}
            </div>
          </section>

          <section className="card" style={{ padding: 18 }}>
            <h3 style={{ fontSize: 13.5, fontWeight: 680, marginBottom: 11, display: "inline-flex", alignItems: "center", gap: 6 }}>
              <IconHash size={14} style={{ color: "var(--text-3)" }} /> 热门标签
            </h3>
            <div style={{ display: "flex", flexWrap: "wrap", gap: 6 }}>
              {(hotTags.data?.items ?? []).map((t) => (
                <Link key={t.id} href={`/t/${t.slug}`} className="tag">
                  # {t.name}
                </Link>
              ))}
              {(hotTags.data?.items.length ?? 0) === 0 && !hotTags.isLoading && (
                <span style={{ color: "var(--text-3)", fontSize: 13 }}>暂无标签</span>
              )}
            </div>
          </section>
        </aside>
      </div>
    </div>
  );
}
