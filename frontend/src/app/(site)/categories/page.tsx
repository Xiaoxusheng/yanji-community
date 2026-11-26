"use client";

// 版块列表：精致网格卡片展示、子版块与帖子量。
import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { api, fmtCount } from "@/lib/api";
import type { Category } from "@/lib/types";
import { PageHeader, ListSkeleton, ErrorState, EmptyState } from "@/components/ui/States";
import { IconGrid } from "@/components/ui/Icons";

export default function CategoriesPage() {
  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ["categories"],
    queryFn: () => api.get<{ items: Category[] }>("/categories"),
  });

  const roots = (data?.items ?? []).filter((c) => !c.parent_id);

  return (
    <div style={{ maxWidth: 960, margin: "0 auto" }}>
      <PageHeader eyebrow="TOPICS" title="全部版块" subtitle="按感兴趣的话题分类探索高质量讨论与专栏" />
      {isLoading && <ListSkeleton count={4} />}
      {error && <ErrorState message={(error as Error).message} onRetry={() => refetch()} />}
      {!isLoading && !error && roots.length === 0 && <EmptyState title="暂无版块" />}
      <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(300px, 1fr))", gap: 14 }}>
        {roots.map((c) => {
          const children = (data?.items ?? []).filter((x) => x.parent_id === c.id);
          return (
            <section
              key={c.id}
              className="card card-interactive"
              style={{ padding: 20, display: "flex", flexDirection: "column", gap: 14 }}
            >
              <div style={{ display: "flex", alignItems: "flex-start", gap: 13 }}>
                <div
                  style={{
                    width: 44,
                    height: 44,
                    borderRadius: 12,
                    background: "var(--brand-soft)",
                    border: "1px solid var(--border)",
                    color: "var(--brand)",
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "center",
                    flexShrink: 0,
                  }}
                  aria-hidden
                >
                  {c.icon ? <span style={{ fontSize: 19 }}>{c.icon}</span> : <IconGrid size={19} />}
                </div>
                <div style={{ flex: 1, minWidth: 0 }}>
                  <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 8 }}>
                    <Link href={`/c/${c.slug}`} className="post-card-title" style={{ fontSize: 16 }}>
                      {c.name}
                    </Link>
                    <span className="tag tag-outline tabular-nums">{fmtCount(c.post_count)} 帖</span>
                  </div>
                  <p style={{ color: "var(--text-3)", fontSize: 13, margin: "5px 0 0", lineHeight: 1.55 }}>
                    {c.description || "综合讨论版块"}
                  </p>
                </div>
              </div>
              {children.length > 0 && (
                <div style={{ display: "flex", flexWrap: "wrap", gap: 6, paddingTop: 12, borderTop: "1px solid var(--border)" }}>
                  {children.map((ch) => (
                    <Link key={ch.id} href={`/c/${ch.slug}`} className="tag tabular-nums">
                      {ch.name} · {fmtCount(ch.post_count)}
                    </Link>
                  ))}
                </div>
              )}
            </section>
          );
        })}
      </div>
    </div>
  );
}
