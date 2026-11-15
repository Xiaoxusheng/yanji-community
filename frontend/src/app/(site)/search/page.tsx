"use client";

// 搜索结果页：类型筛选、排序、关键词高亮。
import { Suspense, useState } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { useQuery } from "@tanstack/react-query";
import { api, timeAgo, type PageData } from "@/lib/api";
import type { SearchPostResult } from "@/lib/types";
import { ListSkeleton, ErrorState, EmptyState, Pagination } from "@/components/ui/States";
import { Avatar } from "@/components/ui/Avatar";

interface UserRes {
  id: string;
  username: string;
  display_name: string;
  avatar_key: string;
  bio: string;
}

function SearchInner() {
  const params = useSearchParams();
  const q = params.get("q") ?? "";
  const [type, setType] = useState(params.get("type") ?? "post");
  const [sort, setSort] = useState("relevance");
  const [page, setPage] = useState(1);

  const posts = useQuery({
    enabled: type === "post" && !!q,
    queryKey: ["search", "post", q, sort, page],
    queryFn: () =>
      api.get<PageData<SearchPostResult>>(
        `/search?q=${encodeURIComponent(q)}&type=post&sort=${sort}&page=${page}&page_size=20`,
      ),
  });
  const users = useQuery({
    enabled: type === "user" && !!q,
    queryKey: ["search", "user", q, page],
    queryFn: () =>
      api.get<PageData<UserRes>>(`/search?q=${encodeURIComponent(q)}&type=user&page=${page}&page_size=20`),
  });

  const types = [
    { key: "post", label: "帖子" },
    { key: "user", label: "用户" },
    { key: "category", label: "版块" },
    { key: "tag", label: "标签" },
  ];

  return (
    <div style={{ maxWidth: 760, margin: "0 auto" }}>
      <h1 style={{ fontSize: 20, fontWeight: 700, marginBottom: 14 }}>
        {q ? <>搜索 “{q}”</> : "搜索"}
      </h1>
      <div style={{ display: "flex", gap: 6, marginBottom: 16, flexWrap: "wrap" }}>
        {types.map((t) => (
          <button
            key={t.key}
            className={`btn btn-sm ${type === t.key ? "btn-primary" : "btn-secondary"}`}
            onClick={() => { setType(t.key); setPage(1); }}
          >
            {t.label}
          </button>
        ))}
        {type === "post" && (
          <>
            <span style={{ marginLeft: "auto" }} />
            <button className={`btn btn-sm btn-text ${sort === "relevance" ? "btn-primary" : ""}`} onClick={() => setSort("relevance")}>
              相关度
            </button>
            <button className={`btn btn-sm btn-text ${sort === "latest" ? "btn-primary" : ""}`} onClick={() => setSort("latest")}>
              最新
            </button>
            <button className={`btn btn-sm btn-text ${sort === "hot" ? "btn-primary" : ""}`} onClick={() => setSort("hot")}>
              热门
            </button>
          </>
        )}
      </div>

      {!q && <EmptyState title="输入关键词开始搜索" description="支持搜索帖子标题与正文、用户名、版块和标签" />}

      {q && type === "post" && (
        <>
          {posts.isLoading && <ListSkeleton />}
          {posts.error && <ErrorState message={(posts.error as Error).message} onRetry={() => posts.refetch()} />}
          {posts.data && posts.data.items.length === 0 && (
            <EmptyState title="没有找到相关帖子" description="换个关键词试试" />
          )}
          <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
            {(posts.data?.items ?? []).map((it) => (
              <article key={it.id} className="card" style={{ padding: 16 }}>
                <Link href={`/p/${it.id}`}>
                  <h2 style={{ fontSize: 16.5, fontWeight: 600 }}>{it.title}</h2>
                </Link>
                <div
                  className="prose-community"
                  style={{ fontSize: 13.5, color: "var(--text-2)", margin: "8px 0" }}
                  dangerouslySetInnerHTML={{ __html: it.highlight }}
                />
                <div style={{ display: "flex", gap: 10, fontSize: 13, color: "var(--text-3)" }}>
                  <Link href={`/c/${it.category_slug}`} className="tag tag-info">{it.category_name}</Link>
                  <span>{it.author_name}</span>
                  <span>{timeAgo(it.published_at)}</span>
                  <span>👍 {it.like_count}</span>
                  <span>💬 {it.comment_count}</span>
                </div>
              </article>
            ))}
          </div>
          {posts.data && posts.data.total > 0 && (
            <Pagination page={page} pageSize={20} total={posts.data.total} onChange={setPage} />
          )}
        </>
      )}

      {q && type === "user" && (
        <>
          {users.isLoading && <ListSkeleton />}
          {users.error && <ErrorState message={(users.error as Error).message} onRetry={() => users.refetch()} />}
          {users.data && users.data.items.length === 0 && <EmptyState title="没有找到相关用户" />}
          <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
            {(users.data?.items ?? []).map((u) => (
              <Link key={u.id} href={`/u/${u.username}`} className="card" style={{ padding: 14, display: "flex", gap: 12, alignItems: "center" }}>
                <Avatar name={u.display_name || u.username} avatarKey={u.avatar_key} size={40} />
                <div>
                  <div style={{ fontWeight: 600 }}>{u.display_name}</div>
                  <div style={{ fontSize: 13, color: "var(--text-3)" }}>@{u.username} {u.bio && `· ${u.bio}`}</div>
                </div>
              </Link>
            ))}
          </div>
        </>
      )}

      {q && (type === "category" || type === "tag") && (
        <CategoryTagSearch q={q} type={type} />
      )}
    </div>
  );
}

function CategoryTagSearch({ q, type }: { q: string; type: string }) {
  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ["search", type, q],
    queryFn: () =>
      api.get<{ items: { name?: string; slug?: string; description?: string; post_count?: number }[]; total: number }>(
        `/search?q=${encodeURIComponent(q)}&type=${type}`,
      ),
  });
  if (isLoading) return <ListSkeleton count={2} />;
  if (error) return <ErrorState message={(error as Error).message} onRetry={() => refetch()} />;
  const items = data?.items ?? [];
  if (items.length === 0) return <EmptyState title="没有找到相关结果" />;
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
      {items.map((it, i) => (
        <Link
          key={i}
          href={type === "category" ? `/c/${it.slug}` : `/t/${it.slug}`}
          className="card"
          style={{ padding: 14, display: "flex", justifyContent: "space-between", alignItems: "center" }}
        >
          <div>
            <div style={{ fontWeight: 600 }}>{type === "category" ? it.name ?? "" : `# ${it.name ?? ""}`}</div>
            {it.description ? <div style={{ fontSize: 13, color: "var(--text-3)" }}>{it.description}</div> : null}
          </div>
          <span className="tag">{it.post_count} 帖</span>
        </Link>
      ))}
    </div>
  );
}

export default function SearchPage() {
  return (
    <Suspense>
      <SearchInner />
    </Suspense>
  );
}
