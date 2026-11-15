"use client";

// 版块列表：分组展示、简介、帖子量。
import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { Category } from "@/lib/types";
import { PageHeader, ListSkeleton, ErrorState, EmptyState } from "@/components/ui/States";

export default function CategoriesPage() {
  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ["categories"],
    queryFn: () => api.get<{ items: Category[] }>("/categories"),
  });

  const roots = (data?.items ?? []).filter((c) => !c.parent_id);

  return (
    <div style={{ maxWidth: 860, margin: "0 auto" }}>
      <PageHeader title="版块" subtitle="按主题浏览社区内容" />
      {isLoading && <ListSkeleton count={4} />}
      {error && <ErrorState message={(error as Error).message} onRetry={() => refetch()} />}
      {!isLoading && !error && roots.length === 0 && <EmptyState title="暂无版块" />}
      <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
        {roots.map((c) => {
          const children = (data?.items ?? []).filter((x) => x.parent_id === c.id);
          return (
            <section key={c.id} className="card" style={{ padding: 16 }}>
              <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
                <div
                  style={{
                    width: 40, height: 40, borderRadius: 10, background: "var(--brand-soft)",
                    color: "var(--brand)", display: "flex", alignItems: "center", justifyContent: "center",
                    fontSize: 20, flexShrink: 0,
                  }}
                  aria-hidden
                >
                  {c.icon || "📁"}
                </div>
                <div style={{ flex: 1 }}>
                  <Link href={`/c/${c.slug}`} style={{ fontWeight: 700, fontSize: 15.5 }}>
                    {c.name}
                  </Link>
                  <div style={{ color: "var(--text-3)", fontSize: 13 }}>{c.description}</div>
                </div>
                <span className="tag">{c.post_count} 帖</span>
              </div>
              {children.length > 0 && (
                <div style={{ display: "flex", flexWrap: "wrap", gap: 8, marginTop: 12, paddingLeft: 50 }}>
                  {children.map((ch) => (
                    <Link key={ch.id} href={`/c/${ch.slug}`} className="tag">
                      {ch.name} · {ch.post_count}
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
