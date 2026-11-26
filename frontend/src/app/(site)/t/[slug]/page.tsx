"use client";

// 标签详情：关联帖子、真实关注状态检查与切换。
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, fmtCount, type PageData } from "@/lib/api";
import type { Tag, PostItem } from "@/lib/types";
import { PostList } from "@/components/post/PostCard";
import { ListSkeleton, ErrorState } from "@/components/ui/States";
import { IconCheck, IconHash, IconPlus } from "@/components/ui/Icons";
import { useMe } from "@/lib/auth";
import { toast } from "@/lib/toast";

export default function TagDetailPage() {
  const params = useParams<{ slug: string }>();
  const slug = params.slug;
  const router = useRouter();
  const qc = useQueryClient();
  const { data: me } = useMe();
  const [page, setPage] = useState(1);
  const [busy, setBusy] = useState(false);

  const tag = useQuery({
    queryKey: ["tag", slug],
    queryFn: () => api.get<{ tag: Tag }>(`/tags/${slug}`),
  });
  const posts = useQuery({
    queryKey: ["tag-posts", slug, page],
    queryFn: () => api.get<PageData<PostItem>>(`/tags/${slug}/posts?page=${page}&page_size=20`),
  });
  const myFollowedTags = useQuery({
    enabled: !!me?.authenticated,
    queryKey: ["me-following", "tag"],
    queryFn: () => api.get<PageData<{ id: number; slug: string; name: string }>>("/me/following?type=tag&page=1&page_size=100"),
  });

  const isFollowing = (myFollowedTags.data?.items ?? []).some((t) => t.slug === slug);

  const toggleFollow = async () => {
    if (!me?.authenticated) {
      toast("info", "请先登录");
      router.push("/login");
      return;
    }
    setBusy(true);
    try {
      if (isFollowing) {
        await api.delete(`/tags/${slug}/follow`);
        toast("success", "已取消关注标签");
      } else {
        await api.post(`/tags/${slug}/follow`);
        toast("success", "已关注该标签");
      }
      qc.invalidateQueries({ queryKey: ["me-following", "tag"] });
    } catch (e) {
      toast("error", (e as Error).message);
    } finally {
      setBusy(false);
    }
  };

  if (tag.isLoading) return <ListSkeleton count={3} />;
  if (tag.error || !tag.data) return <ErrorState message={(tag.error as Error)?.message} onRetry={() => tag.refetch()} />;
  const t = tag.data.tag;

  return (
    <div style={{ maxWidth: 780, margin: "0 auto" }}>
      <div className="card" style={{ padding: 20, marginBottom: 16, display: "flex", alignItems: "center", gap: 14, flexWrap: "wrap" }}>
        <div
          style={{
            width: 46,
            height: 46,
            borderRadius: 12,
            background: "var(--bg-inset)",
            border: "1px solid var(--border)",
            color: "var(--text-1)",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            flexShrink: 0,
          }}
          aria-hidden
        >
          <IconHash size={20} />
        </div>
        <div style={{ flex: 1, minWidth: 180 }}>
          <h1 style={{ fontSize: 20, fontWeight: 700, letterSpacing: "-0.02em" }}>{t.name}</h1>
          <div className="tabular-nums" style={{ color: "var(--text-3)", fontSize: 13, marginTop: 2 }}>
            {t.description || "话题标签"} · {fmtCount(t.post_count)} 篇内容
          </div>
        </div>
        <div style={{ display: "flex", gap: 8 }}>
          <button
            className={`btn btn-sm ${isFollowing ? "btn-secondary" : "btn-primary"}`}
            onClick={toggleFollow}
            disabled={busy}
          >
            {isFollowing ? (
              <>
                <IconCheck size={14} /> 已关注
              </>
            ) : (
              "关注标签"
            )}
          </button>
          <Link href="/compose" className="btn btn-secondary btn-sm">
            <IconPlus size={14} /> 发帖
          </Link>
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
        emptyTitle="该标签下还没有帖子"
      />
    </div>
  );
}
