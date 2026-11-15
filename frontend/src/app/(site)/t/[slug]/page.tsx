"use client";

// 标签详情：介绍、关注、关联帖子。
import Link from "next/link";
import { useParams } from "next/navigation";
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { Tag, PostItem } from "@/lib/types";
import type { PageData } from "@/lib/api";
import { PostList } from "@/components/post/PostCard";
import { ListSkeleton, ErrorState } from "@/components/ui/States";

export default function TagDetailPage() {
  const params = useParams<{ slug: string }>();
  const slug = params.slug;
  const [page, setPage] = useState(1);

  const tag = useQuery({
    queryKey: ["tag", slug],
    queryFn: () => api.get<{ tag: Tag }>(`/tags/${slug}`),
  });
  const posts = useQuery({
    queryKey: ["tag-posts", slug, page],
    queryFn: () => api.get<PageData<PostItem>>(`/tags/${slug}/posts?page=${page}&page_size=20`),
  });

  const follow = async () => {
    try {
      await api.post(`/tags/${slug}/follow`);
      // 标签关注状态当前未在详情返回，操作成功即反馈
    } catch {
      /* 静默 */
    }
  };

  if (tag.isLoading) return <ListSkeleton count={3} />;
  if (tag.error || !tag.data) return <ErrorState message={(tag.error as Error)?.message} onRetry={() => tag.refetch()} />;
  const t = tag.data.tag;

  return (
    <div style={{ maxWidth: 760, margin: "0 auto" }}>
      <div className="card" style={{ padding: 20, marginBottom: 16, display: "flex", alignItems: "center", gap: 14 }}>
        <div
          style={{
            width: 48, height: 48, borderRadius: 12, background: "var(--brand-soft)",
            color: "var(--brand)", display: "flex", alignItems: "center", justifyContent: "center",
            fontSize: 22, fontWeight: 700, flexShrink: 0,
          }}
          aria-hidden
        >
          #
        </div>
        <div style={{ flex: 1 }}>
          <h1 style={{ fontSize: 20, fontWeight: 700 }}>{t.name}</h1>
          <div style={{ color: "var(--text-3)", fontSize: 13.5 }}>
            {t.description || "暂无介绍"} · {t.post_count} 篇帖子
          </div>
        </div>
        <button className="btn btn-secondary btn-sm" onClick={follow}>
          关注标签
        </button>
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
        emptyTitle="该标签下还没有帖子"
      />
      <div style={{ textAlign: "center", marginTop: 16 }}>
        <Link href="/compose" className="btn btn-primary btn-sm">
          发布带该标签的帖子
        </Link>
      </div>
    </div>
  );
}
