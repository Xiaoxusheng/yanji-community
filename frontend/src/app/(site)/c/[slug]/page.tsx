"use client";

// 版块详情：简介、公告、置顶、帖子列表（最新/热门排序）、关注版块。
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useParams } from "next/navigation";
import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { Category, PostItem, Announcement } from "@/lib/types";
import type { PageData } from "@/lib/api";
import { PostList } from "@/components/post/PostCard";
import { PageHeader, ListSkeleton, ErrorState } from "@/components/ui/States";
import { useMe } from "@/lib/auth";
import { toast } from "@/lib/toast";

export default function CategoryDetailPage() {
  const params = useParams<{ slug: string }>();
  const slug = params.slug;
  const qc = useQueryClient();
  const router = useRouter();
  const { data: me } = useMe();
  const [sort, setSort] = useState("latest");
  const [page, setPage] = useState(1);

  const cat = useQuery({
    queryKey: ["category", slug],
    queryFn: () =>
      api.get<{ category: Category; moderators: { username: string; display_name: string }[]; is_following: boolean }>(
        `/categories/${slug}`,
      ),
  });
  const posts = useQuery({
    queryKey: ["category-posts", slug, sort, page],
    queryFn: () => api.get<PageData<PostItem>>(`/categories/${slug}/posts?sort=${sort}&page=${page}&page_size=20`),
  });
  const ann = useQuery({
    queryKey: ["announcements"],
    queryFn: () => api.get<{ items: Announcement[] }>("/announcements"),
    staleTime: 5 * 60_000,
  });

  const toggleFollow = async () => {
    if (!me?.authenticated) {
      toast("info", "请先登录");
      router.push("/login");
      return;
    }
    try {
      if (cat.data?.is_following) {
        await api.delete(`/categories/${slug}/follow`);
        toast("success", "已取消关注");
      } else {
        await api.post(`/categories/${slug}/follow`);
        toast("success", "已关注版块");
      }
      qc.invalidateQueries({ queryKey: ["category", slug] });
    } catch (e) {
      toast("error", (e as Error).message);
    }
  };

  if (cat.isLoading) return <ListSkeleton count={3} />;
  if (cat.error || !cat.data) return <ErrorState message={(cat.error as Error)?.message} onRetry={() => cat.refetch()} />;
  const c = cat.data.category;

  return (
    <div style={{ display: "grid", gridTemplateColumns: "minmax(0,1fr) 300px", gap: 24, alignItems: "start" }}>
      <div>
        <PageHeader
          title={c.name}
          subtitle={c.description}
        />
        <div style={{ display: "flex", gap: 8, marginBottom: 14, alignItems: "center" }}>
          <button className={`btn btn-sm ${sort === "latest" ? "btn-primary" : "btn-secondary"}`} onClick={() => { setSort("latest"); setPage(1); }}>
            最新
          </button>
          <button className={`btn btn-sm ${sort === "hot" ? "btn-primary" : "btn-secondary"}`} onClick={() => { setSort("hot"); setPage(1); }}>
            热门
          </button>
          <span style={{ marginLeft: "auto" }} />
          {c.status === "archived" ? (
            <span className="tag tag-warning">版块已归档，仅供浏览</span>
          ) : (
            <Link href={`/compose?category=${c.slug}`} className="btn btn-primary btn-sm">
              发帖
            </Link>
          )}
        </div>
        <PostList
          posts={posts.data?.items}
          loading={posts.isLoading}
          error={posts.error as Error | null}
          onRetry={() => posts.refetch()}
          page={page}
          pageSize={20}
          total={posts.data?.total ?? 0}
          onPageChange={setPage}
        />
      </div>
      <aside style={{ display: "flex", flexDirection: "column", gap: 16, position: "sticky", top: 72 }}>
        <section className="card" style={{ padding: 16 }}>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 8 }}>
            <h3 style={{ fontSize: 14, fontWeight: 700 }}>版块信息</h3>
            <button className="btn btn-secondary btn-sm" onClick={toggleFollow}>
              {cat.data?.is_following ? "已关注 ✓" : "关注"}
            </button>
          </div>
          <div style={{ fontSize: 13.5, color: "var(--text-2)", display: "flex", flexDirection: "column", gap: 6 }}>
            <div>📝 帖子数：{c.post_count}</div>
            <div>📋 发帖权限：{c.posting_policy === "all" ? "所有人" : c.posting_policy === "verified" ? "已验证用户" : "版块管理团队"}</div>
            {cat.data.moderators.length > 0 && (
              <div>
                👮 版主：{cat.data.moderators.map((m) => m.display_name).join("、")}
              </div>
            )}
          </div>
        </section>
        {(ann.data?.items.length ?? 0) > 0 && (
          <section className="card" style={{ padding: 16 }}>
            <h3 style={{ fontSize: 14, fontWeight: 700, marginBottom: 10 }}>📢 公告</h3>
            {ann.data!.items.slice(0, 3).map((a) => (
              <div key={a.id} style={{ fontSize: 13.5, marginBottom: 8 }}>
                <b>{a.title}</b>
                <div style={{ color: "var(--text-3)", fontSize: 12.5 }}>{a.body}</div>
              </div>
            ))}
          </section>
        )}
      </aside>
    </div>
  );
}
