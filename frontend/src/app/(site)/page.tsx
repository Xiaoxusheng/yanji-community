"use client";

// 首页：推荐/最新/热门/精华/关注 Tab 信息流 + 公告 + 热门标签/版块侧栏。
import Link from "next/link";
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api, timeAgo, type PageData } from "@/lib/api";
import type { PostItem, Announcement, Tag, Category } from "@/lib/types";
import { PostList } from "@/components/post/PostCard";
import { useMe } from "@/lib/auth";

const tabs = [
  { key: "recommended", label: "推荐" },
  { key: "latest", label: "最新" },
  { key: "hot", label: "热门" },
  { key: "featured", label: "精华" },
  { key: "following", label: "关注" },
];

export default function HomePage() {
  const [tab, setTab] = useState("recommended");
  const [page, setPage] = useState(1);
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
    queryFn: () => api.get<PageData<Tag>>("/tags?page_size=10"),
    staleTime: 5 * 60_000,
  });
  const categories = useQuery({
    queryKey: ["categories"],
    queryFn: () => api.get<{ items: Category[] }>("/categories"),
    staleTime: 5 * 60_000,
  });

  return (
    <div style={{ display: "grid", gridTemplateColumns: "minmax(0,1fr) 300px", gap: 24, alignItems: "start" }}>
      <div>
        {/* Tab 切换 */}
        <div className="card" style={{ padding: "4px 8px", marginBottom: 14, display: "flex", gap: 2 }} role="tablist" aria-label="信息流切换">
          {tabs.map((t) => (
            <button
              key={t.key}
              role="tab"
              aria-selected={tab === t.key}
              className={`btn btn-text ${tab === t.key ? "" : ""}`}
              onClick={() => {
                setTab(t.key);
                setPage(1);
              }}
              style={{
                fontWeight: tab === t.key ? 700 : 400,
                color: tab === t.key ? "var(--brand)" : "var(--text-2)",
                borderBottom: tab === t.key ? "2px solid var(--brand)" : "2px solid transparent",
                borderRadius: 0,
              }}
            >
              {t.label}
            </button>
          ))}
        </div>

        {isFollowingTab && !me?.authenticated ? (
          <div className="card" style={{ padding: "40px 24px", textAlign: "center" }}>
            <div style={{ fontSize: 36, marginBottom: 8 }}>👀</div>
            <div style={{ fontWeight: 600, marginBottom: 4 }}>登录后查看关注动态</div>
            <p style={{ color: "var(--text-3)", fontSize: 14, marginBottom: 16 }}>
              关注你感兴趣的用户、版块和标签，在这里看到他们的最新内容
            </p>
            <Link href="/login" className="btn btn-primary btn-sm">
              去登录
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
            emptyTitle={isFollowingTab ? "关注的内容还很少" : "暂无内容"}
            emptyDesc={isFollowingTab ? "去关注一些用户、版块或标签吧" : "成为第一个发帖的人吧"}
          />
        )}
      </div>

      {/* 侧栏 */}
      <aside style={{ display: "flex", flexDirection: "column", gap: 16, position: "sticky", top: 72 }}>
        <section className="card" style={{ padding: 16 }}>
          <h3 style={{ fontSize: 14, fontWeight: 700, marginBottom: 10 }}>📢 公告</h3>
          {announcements.isLoading ? (
            <div className="skeleton" style={{ height: 40 }} />
          ) : (announcements.data?.items.length ?? 0) === 0 ? (
            <div style={{ color: "var(--text-3)", fontSize: 13 }}>暂无公告</div>
          ) : (
            <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
              {announcements.data!.items.slice(0, 3).map((a) => (
                <div key={a.id}>
                  <div style={{ fontSize: 13.5, fontWeight: 600 }}>{a.title}</div>
                  <div style={{ fontSize: 12.5, color: "var(--text-3)", marginTop: 2, display: "-webkit-box", WebkitLineClamp: 2, WebkitBoxOrient: "vertical", overflow: "hidden" }}>
                    {a.body}
                  </div>
                  <div style={{ fontSize: 12, color: "var(--text-3)", marginTop: 2 }}>{timeAgo(a.published_at)}</div>
                </div>
              ))}
            </div>
          )}
        </section>

        <section className="card" style={{ padding: 16 }}>
          <h3 style={{ fontSize: 14, fontWeight: 700, marginBottom: 10 }}>🔥 热门标签</h3>
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

        <section className="card" style={{ padding: 16 }}>
          <h3 style={{ fontSize: 14, fontWeight: 700, marginBottom: 10 }}>📚 版块导航</h3>
          <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
            {(categories.data?.items ?? []).slice(0, 6).map((c) => (
              <Link key={c.id} href={`/c/${c.slug}`} style={{ display: "flex", justifyContent: "space-between", fontSize: 13.5 }}>
                <span>{c.name}</span>
                <span style={{ color: "var(--text-3)" }}>{c.post_count} 帖</span>
              </Link>
            ))}
          </div>
          <Link href="/categories" style={{ fontSize: 13, color: "var(--brand)", marginTop: 10, display: "inline-block" }}>
            查看全部版块 →
          </Link>
        </section>

        <section className="card" style={{ padding: 16, background: "var(--brand-soft)", border: "none" }}>
          <div style={{ fontWeight: 700, marginBottom: 6 }}>问答 · 投票 · 活动（规划中）</div>
          <p style={{ fontSize: 13, color: "var(--text-2)", margin: 0 }}>
            问答、投票、积分与活动功能正在规划中，敬请期待。
          </p>
        </section>
      </aside>
    </div>
  );
}
