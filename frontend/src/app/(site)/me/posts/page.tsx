"use client";

// 我的内容：全部状态帖子管理。
import Link from "next/link";
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api, type PageData } from "@/lib/api";
import type { PostItem } from "@/lib/types";
import { PostList } from "@/components/post/PostCard";
import { PageHeader } from "@/components/ui/States";
import { AuthShell } from "@/components/auth/forms";
import { useMe } from "@/lib/auth";
import { IconPlus } from "@/components/ui/Icons";

const statusFilters = [
  { key: "", label: "全部" },
  { key: "draft", label: "草稿" },
  { key: "pending_review", label: "待审核" },
  { key: "published", label: "已发布" },
  { key: "rejected", label: "未通过" },
  { key: "hidden", label: "已隐藏" },
];

export default function MyPostsPage() {
  const { data: me, isLoading: meLoading } = useMe();
  const [status, setStatus] = useState("");
  const [page, setPage] = useState(1);

  const posts = useQuery({
    enabled: !!me?.authenticated,
    queryKey: ["me-posts", status, page],
    queryFn: () => {
      const s = status ? `&status=${status}` : "";
      return api.get<PageData<PostItem>>(`/me/posts?${s}page=${page}&page_size=20`);
    },
  });

  if (meLoading) return null;
  if (me && !me.authenticated) {
    return (
      <AuthShell title="请先登录">
        <p style={{ marginBottom: 16, color: "var(--text-2)" }}>登录后查看和管理你发布的内容。</p>
        <Link href="/login" className="btn btn-primary">去登录</Link>
      </AuthShell>
    );
  }

  return (
    <div style={{ maxWidth: 800, margin: "0 auto" }}>
      <PageHeader
        title="我的内容"
        subtitle="草稿可在详情或编辑页继续完善与发布"
        action={
          <Link href="/compose" className="btn btn-primary btn-sm">
            <IconPlus size={14} /> 发布新帖
          </Link>
        }
      />
      <div style={{ marginBottom: 14 }}>
        <div className="seg-tabs" style={{ overflowX: "auto", maxWidth: "100%" }}>
          {statusFilters.map((f) => (
            <button
              key={f.key}
              className={`seg-tab ${status === f.key ? "is-active" : ""}`}
              onClick={() => {
                setStatus(f.key);
                setPage(1);
              }}
            >
              {f.label}
            </button>
          ))}
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
        showStatus
        emptyTitle="当前状态下还没有内容"
        emptyDesc="写一篇帖子开启你的社区记录"
      />
    </div>
  );
}
