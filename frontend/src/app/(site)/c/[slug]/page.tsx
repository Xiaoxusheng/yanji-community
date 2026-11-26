"use client";

// 版块详情：简介、排序切换、关注、版主列表与公告。
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, fmtCount } from "@/lib/api";
import type { Category, PostItem, Announcement } from "@/lib/types";
import type { PageData } from "@/lib/api";
import { PostList } from "@/components/post/PostCard";
import { PageHeader, ListSkeleton, ErrorState } from "@/components/ui/States";
import { useMe } from "@/lib/auth";
import { toast } from "@/lib/toast";
import { IconCheck, IconFileText, IconPlus, IconShield } from "@/components/ui/Icons";

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
        toast("success", "已取消关注版块");
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
    <div className="layout-main-rail">
      <div>
        <PageHeader
          title={c.name}
          subtitle={c.description}
          action={
            c.status === "archived" ? (
              <span className="tag tag-warning">版块已归档</span>
            ) : (
              <Link href={`/compose?category=${c.slug}`} className="btn btn-primary btn-sm">
                <IconPlus size={14} /> 发布到此版块
              </Link>
            )
          }
        />

        <div style={{ display: "flex", gap: 8, marginBottom: 14, alignItems: "center" }}>
          <div className="seg-tabs">
            <button
              type="button"
              className={`seg-tab ${sort === "latest" ? "is-active" : ""}`}
              onClick={() => {
                setSort("latest");
                setPage(1);
              }}
            >
              最新回复
            </button>
            <button
              type="button"
              className={`seg-tab ${sort === "hot" ? "is-active" : ""}`}
              onClick={() => {
                setSort("hot");
                setPage(1);
              }}
            >
              最热讨论
            </button>
          </div>
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

      <aside className="sticky-rail">
        <section className="card" style={{ padding: 16 }}>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 12 }}>
            <h3 style={{ fontSize: 13.5, fontWeight: 650 }}>版块信息</h3>
            <button
              className={`btn btn-xs ${cat.data.is_following ? "btn-secondary" : "btn-primary"}`}
              onClick={toggleFollow}
            >
              {cat.data.is_following ? (
                <>
                  <IconCheck size={12} /> 已关注
                </>
              ) : (
                "+ 关注"
              )}
            </button>
          </div>
          <div style={{ fontSize: 13, color: "var(--text-2)", display: "flex", flexDirection: "column", gap: 8 }}>
            <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
              <IconFileText size={14} style={{ color: "var(--text-3)" }} />
              <span className="tabular-nums">累计帖子：{fmtCount(c.post_count)} 篇</span>
            </div>
            <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
              <IconShield size={14} style={{ color: "var(--text-3)" }} />
              <span>发帖权限：{c.posting_policy === "all" ? "所有成员" : c.posting_policy === "verified" ? "已验证成员" : "仅限管理团队"}</span>
            </div>
            {cat.data.moderators.length > 0 && (
              <div style={{ paddingTop: 8, borderTop: "1px solid var(--border)", fontSize: 12.5, color: "var(--text-3)" }}>
                版主：{cat.data.moderators.map((m) => m.display_name).join("、")}
              </div>
            )}
          </div>
        </section>

        {(ann.data?.items.length ?? 0) > 0 && (
          <section className="card" style={{ padding: 16 }}>
            <h3 style={{ fontSize: 13.5, fontWeight: 650, marginBottom: 10 }}>全站公告</h3>
            <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
              {ann.data!.items.slice(0, 3).map((a) => (
                <div key={a.id} style={{ fontSize: 13 }}>
                  <div style={{ fontWeight: 600 }}>{a.title}</div>
                  {a.body && <div style={{ color: "var(--text-3)", fontSize: 12.5, marginTop: 2 }}>{a.body}</div>}
                </div>
              ))}
            </div>
          </section>
        )}
      </aside>
    </div>
  );
}
