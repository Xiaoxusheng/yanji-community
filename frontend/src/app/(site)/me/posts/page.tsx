"use client";

// 我的内容：全部状态帖子（含草稿/待审核/被隐藏）。
import Link from "next/link";
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api, type PageData } from "@/lib/api";
import type { PostItem } from "@/lib/types";
import { PostList } from "@/components/post/PostCard";
import { AuthShell } from "@/components/auth/forms";
import { useMe } from "@/lib/auth";

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
        <p style={{ marginBottom: 16 }}>登录后管理你的内容。</p>
        <Link href="/login" className="btn btn-primary">去登录</Link>
      </AuthShell>
    );
  }

  return (
    <div style={{ maxWidth: 780, margin: "0 auto" }}>
      <div style={{ display: "flex", alignItems: "center", marginBottom: 14, flexWrap: "wrap", gap: 6 }}>
        <h1 style={{ fontSize: 20, fontWeight: 700 }}>我的内容</h1>
        <span style={{ marginLeft: "auto" }} />
        <Link href="/compose" className="btn btn-primary btn-sm">
          写新帖
        </Link>
      </div>
      <div style={{ display: "flex", gap: 6, marginBottom: 14, flexWrap: "wrap" }}>
        {statusFilters.map((f) => (
          <button
            key={f.key}
            className={`btn btn-sm ${status === f.key ? "btn-primary" : "btn-secondary"}`}
            onClick={() => { setStatus(f.key); setPage(1); }}
          >
            {f.label}
          </button>
        ))}
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
        emptyTitle="这里还没有内容"
        emptyDesc="写一篇帖子开始你的社区之旅"
      />
      <p style={{ fontSize: 12.5, color: "var(--text-3)", marginTop: 12 }}>
        提示：草稿在编辑页继续完善；在帖子详情页可以编辑或删除自己的内容。
      </p>
    </div>
  );
}
