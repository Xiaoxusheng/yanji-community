"use client";

// 搜索页：站内即时重搜、类型/排序切换与后端高级筛选（版块、作者、日期区间）。
import { Suspense, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useQuery } from "@tanstack/react-query";
import DOMPurify from "dompurify";
import { api, fmtCount, timeAgo, type PageData } from "@/lib/api";
import type { Category, SearchPostResult, Tag } from "@/lib/types";
import { ListSkeleton, ErrorState, EmptyState, Pagination } from "@/components/ui/States";
import { Avatar } from "@/components/ui/Avatar";
import { IconMessage, IconSearch, IconSliders, IconThumbUp } from "@/components/ui/Icons";

interface UserRes {
  id: string;
  username: string;
  display_name: string;
  avatar_key: string;
  bio: string;
}

function SearchInner() {
  const router = useRouter();
  const params = useSearchParams();
  const q = params.get("q") ?? "";

  const [kw, setKw] = useState(q);
  const [type, setType] = useState(params.get("type") ?? "post");
  const [sort, setSort] = useState("relevance");
  const [page, setPage] = useState(1);
  const [showFilters, setShowFilters] = useState(false);
  const [catSlug, setCatSlug] = useState("");
  const [author, setAuthor] = useState("");
  const [fromDate, setFromDate] = useState("");
  const [toDate, setToDate] = useState("");

  const categories = useQuery({
    queryKey: ["categories"],
    queryFn: () => api.get<{ items: Category[] }>("/categories"),
    staleTime: 5 * 60_000,
  });
  const hotTags = useQuery({
    queryKey: ["tags", "hot"],
    queryFn: () => api.get<PageData<Tag>>("/tags?page_size=10"),
    staleTime: 5 * 60_000,
  });

  const posts = useQuery({
    enabled: type === "post" && !!q,
    queryKey: ["search", "post", q, sort, catSlug, author, fromDate, toDate, page],
    queryFn: () => {
      let url = `/search?q=${encodeURIComponent(q)}&type=post&sort=${sort}&page=${page}&page_size=20`;
      if (catSlug) url += `&category=${encodeURIComponent(catSlug)}`;
      if (author.trim()) url += `&author=${encodeURIComponent(author.trim())}`;
      if (fromDate) url += `&from=${encodeURIComponent(fromDate)}`;
      if (toDate) url += `&to=${encodeURIComponent(toDate)}`;
      return api.get<PageData<SearchPostResult>>(url);
    },
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

  const doSearch = (nextQ: string) => {
    setPage(1);
    router.push(`/search?q=${encodeURIComponent(nextQ.trim())}&type=${type}`);
  };

  return (
    <div style={{ maxWidth: 800, margin: "0 auto" }}>
      <form
        onSubmit={(e) => {
          e.preventDefault();
          if (kw.trim()) doSearch(kw);
        }}
        style={{ display: "flex", gap: 8, marginBottom: 16 }}
      >
        <div style={{ position: "relative", flex: 1 }}>
          <span style={{ position: "absolute", left: 12, top: "50%", transform: "translateY(-50%)", color: "var(--text-3)", display: "inline-flex" }}>
            <IconSearch size={16} />
          </span>
          <input
            className="input"
            value={kw}
            onChange={(e) => setKw(e.target.value)}
            placeholder="输入关键词搜索帖子、用户、版块或标签…"
            style={{ paddingLeft: 36 }}
          />
        </div>
        <button type="submit" className="btn btn-primary" disabled={!kw.trim()}>
          搜索
        </button>
      </form>

      <div style={{ display: "flex", gap: 10, marginBottom: 14, flexWrap: "wrap", alignItems: "center" }}>
        <div className="seg-tabs">
          {types.map((t) => (
            <button
              key={t.key}
              type="button"
              className={`seg-tab ${type === t.key ? "is-active" : ""}`}
              onClick={() => {
                setType(t.key);
                setPage(1);
              }}
            >
              {t.label}
            </button>
          ))}
        </div>

        {type === "post" && (
          <>
            <div className="seg-tabs" style={{ marginLeft: "auto" }}>
              {([["relevance", "相关度"], ["latest", "最新"], ["hot", "最热"]] as const).map(([k, label]) => (
                <button
                  key={k}
                  type="button"
                  className={`seg-tab ${sort === k ? "is-active" : ""}`}
                  style={{ padding: "5px 10px", fontSize: 12.5 }}
                  onClick={() => {
                    setSort(k);
                    setPage(1);
                  }}
                >
                  {label}
                </button>
              ))}
            </div>
            <button
              type="button"
              className={`btn btn-sm ${showFilters ? "btn-primary" : "btn-secondary"}`}
              onClick={() => setShowFilters((v) => !v)}
            >
              <IconSliders size={14} /> 筛选
            </button>
          </>
        )}
      </div>

      {type === "post" && showFilters && (
        <div className="card" style={{ padding: 14, marginBottom: 14, display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(150px, 1fr))", gap: 10 }}>
          <div>
            <label style={{ fontSize: 12, color: "var(--text-3)", display: "block", marginBottom: 4 }}>限定版块</label>
            <select
              className="select"
              style={{ padding: "6px 10px", fontSize: 13 }}
              value={catSlug}
              onChange={(e) => {
                setCatSlug(e.target.value);
                setPage(1);
              }}
            >
              <option value="">全部版块</option>
              {(categories.data?.items ?? []).map((c) => (
                <option key={c.id} value={c.slug}>{c.name}</option>
              ))}
            </select>
          </div>
          <div>
            <label style={{ fontSize: 12, color: "var(--text-3)", display: "block", marginBottom: 4 }}>作者用户名</label>
            <input
              className="input"
              style={{ padding: "6px 10px", fontSize: 13 }}
              placeholder="如 alice"
              value={author}
              onChange={(e) => {
                setAuthor(e.target.value);
                setPage(1);
              }}
            />
          </div>
          <div>
            <label style={{ fontSize: 12, color: "var(--text-3)", display: "block", marginBottom: 4 }}>起始日期</label>
            <input
              type="date"
              className="input"
              style={{ padding: "5px 10px", fontSize: 13 }}
              value={fromDate}
              onChange={(e) => {
                setFromDate(e.target.value);
                setPage(1);
              }}
            />
          </div>
          <div>
            <label style={{ fontSize: 12, color: "var(--text-3)", display: "block", marginBottom: 4 }}>结束日期</label>
            <input
              type="date"
              className="input"
              style={{ padding: "5px 10px", fontSize: 13 }}
              value={toDate}
              onChange={(e) => {
                setToDate(e.target.value);
                setPage(1);
              }}
            />
          </div>
        </div>
      )}

      {!q && (
        <div className="card" style={{ padding: 24 }}>
          <EmptyState title="输入关键词开始搜索" description="支持搜索帖子标题、正文片段、用户、版块与标签" />
          {(hotTags.data?.items.length ?? 0) > 0 && (
            <div style={{ marginTop: 16, display: "flex", gap: 6, flexWrap: "wrap", justifyContent: "center" }}>
              <span style={{ fontSize: 12.5, color: "var(--text-3)", alignSelf: "center" }}>热门标签：</span>
              {hotTags.data!.items.map((t) => (
                <button
                  key={t.id}
                  type="button"
                  className="tag"
                  style={{ cursor: "pointer" }}
                  onClick={() => {
                    setKw(t.name);
                    doSearch(t.name);
                  }}
                >
                  # {t.name}
                </button>
              ))}
            </div>
          )}
        </div>
      )}

      {q && type === "post" && (
        <>
          {posts.isLoading && <ListSkeleton />}
          {posts.error && <ErrorState message={(posts.error as Error).message} onRetry={() => posts.refetch()} />}
          {posts.data && posts.data.items.length === 0 && (
            <EmptyState title="没有找到相关帖子" description="尝试调整关键词或放宽筛选条件" />
          )}
          <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
            {(posts.data?.items ?? []).map((it) => (
              <article key={it.id} className="card card-interactive" style={{ padding: 16 }}>
                <Link href={`/p/${it.id}`}>
                  <h2 style={{ fontSize: 16, fontWeight: 650 }}>{it.title}</h2>
                </Link>
                <div
                  className="prose-community"
                  style={{ fontSize: 13.5, color: "var(--text-2)", margin: "6px 0 8px" }}
                  dangerouslySetInnerHTML={{
                    __html: DOMPurify.sanitize(it.highlight, { ALLOWED_TAGS: ["mark"], ALLOWED_ATTR: [] }),
                  }}
                />
                <div className="tabular-nums" style={{ display: "flex", gap: 10, fontSize: 12.5, color: "var(--text-3)", alignItems: "center", flexWrap: "wrap" }}>
                  <Link href={`/c/${it.category_slug}`} className="tag tag-outline">{it.category_name}</Link>
                  <span>{it.author_name}</span>
                  <span>·</span>
                  <span>{timeAgo(it.published_at)}</span>
                  <span style={{ marginLeft: "auto", display: "inline-flex", gap: 12 }}>
                    <span style={{ display: "inline-flex", alignItems: "center", gap: 4 }}>
                      <IconThumbUp size={13} /> {fmtCount(it.like_count)}
                    </span>
                    <span style={{ display: "inline-flex", alignItems: "center", gap: 4 }}>
                      <IconMessage size={13} /> {fmtCount(it.comment_count)}
                    </span>
                  </span>
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
          <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
            {(users.data?.items ?? []).map((u) => (
              <Link key={u.id} href={`/u/${u.username}`} className="card card-interactive" style={{ padding: 14, display: "flex", gap: 12, alignItems: "center" }}>
                <Avatar name={u.display_name || u.username} avatarKey={u.avatar_key} size={40} />
                <div>
                  <div style={{ fontWeight: 600 }}>{u.display_name}</div>
                  <div style={{ fontSize: 12.5, color: "var(--text-3)" }}>
                    @{u.username} {u.bio && `· ${u.bio}`}
                  </div>
                </div>
              </Link>
            ))}
          </div>
        </>
      )}

      {q && (type === "category" || type === "tag") && <CategoryTagSearch q={q} type={type} />}
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
    <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
      {items.map((it, i) => (
        <Link
          key={i}
          href={type === "category" ? `/c/${it.slug}` : `/t/${it.slug}`}
          className="card card-interactive"
          style={{ padding: 14, display: "flex", justifyContent: "space-between", alignItems: "center" }}
        >
          <div>
            <div style={{ fontWeight: 600 }}>{type === "tag" ? `# ${it.name}` : it.name}</div>
            {it.description && <div style={{ fontSize: 13, color: "var(--text-3)", marginTop: 2 }}>{it.description}</div>}
          </div>
          {it.post_count !== undefined && <span className="tag tabular-nums">{it.post_count} 帖</span>}
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
