"use client";

// 用户公开主页：资料、帖子、关注/粉丝、关注与拉黑操作。
import Link from "next/link";
import { useParams } from "next/navigation";
import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, timeAgo, type PageData } from "@/lib/api";
import type { ProfilePublic, PostItem } from "@/lib/types";
import { PostList } from "@/components/post/PostCard";
import { Avatar } from "@/components/ui/Avatar";
import { ListSkeleton, ErrorState, EmptyState } from "@/components/ui/States";
import { useMe } from "@/lib/auth";
import { toast } from "@/lib/toast";

export default function UserProfilePage() {
  const params = useParams<{ username: string }>();
  const username = params.username;
  const qc = useQueryClient();
  const { data: me } = useMe();
  const [tab, setTab] = useState<"posts" | "followers" | "following">("posts");
  const [page, setPage] = useState(1);

  const profile = useQuery({
    queryKey: ["profile", username],
    queryFn: () => api.get<{ profile: ProfilePublic }>(`/users/${username}`),
  });
  const posts = useQuery({
    enabled: tab === "posts",
    queryKey: ["user-posts", username, page],
    queryFn: () => api.get<PageData<PostItem>>(`/users/${username}/posts?page=${page}&page_size=20`),
  });
  const followers = useQuery({
    enabled: tab === "followers",
    queryKey: ["user-followers", username, page],
    queryFn: () => api.get<PageData<{ id: string; username: string; display_name: string; avatar_key: string; bio: string }>>(`/users/${username}/followers?page=${page}&page_size=20`),
  });
  const following = useQuery({
    enabled: tab === "following",
    queryKey: ["user-following", username, page],
    queryFn: () => api.get<PageData<{ id: string; username: string; display_name: string; avatar_key: string; bio: string }>>(`/users/${username}/following?page=${page}&page_size=20`),
  });

  const isSelf = me?.authenticated && me.user?.username === username;

  const followUser = async () => {
    if (!profile.data) return;
    try {
      await api.post(`/users/${profile.data.profile.id}/follow`);
      toast("success", "已关注");
      qc.invalidateQueries({ queryKey: ["profile", username] });
    } catch (e) {
      toast("error", (e as Error).message);
    }
  };
  const blockUser = async () => {
    if (!profile.data) return;
    if (!window.confirm("确定拉黑该用户吗？双方将互相看不到对方的动态。")) return;
    try {
      await api.post(`/users/${profile.data.profile.id}/block`);
      toast("success", "已拉黑");
    } catch (e) {
      toast("error", (e as Error).message);
    }
  };

  if (profile.isLoading) return <ListSkeleton count={2} />;
  if (profile.error || !profile.data)
    return <ErrorState message={(profile.error as Error)?.message} onRetry={() => profile.refetch()} />;
  const p = profile.data.profile;

  return (
    <div style={{ maxWidth: 780, margin: "0 auto" }}>
      <div className="card" style={{ padding: 24, marginBottom: 16, display: "flex", gap: 18, alignItems: "flex-start" }}>
        <Avatar name={p.display_name} avatarKey={p.avatar_key} size={72} />
        <div style={{ flex: 1 }}>
          <h1 style={{ fontSize: 20, fontWeight: 700 }}>{p.display_name}</h1>
          <div style={{ color: "var(--text-3)", fontSize: 13.5, marginBottom: 6 }}>
            @{p.username} · {timeAgo(p.created_at)}加入
            {p.location && ` · ${p.location}`}
          </div>
          {p.bio && <p style={{ fontSize: 14, color: "var(--text-2)", margin: 0 }}>{p.bio}</p>}
          {p.website && (
            <a href={p.website} target="_blank" rel="noopener noreferrer nofollow" style={{ fontSize: 13.5, color: "var(--brand)" }}>
              {p.website}
            </a>
          )}
          <div style={{ display: "flex", gap: 20, marginTop: 10, fontSize: 13.5 }}>
            <span><b>{p.post_count}</b> 帖子</span>
            <span><b>{p.comment_count}</b> 评论</span>
            <span><b>{p.follower_count}</b> 粉丝</span>
            <span><b>{p.following_count}</b> 关注中</span>
          </div>
        </div>
        {!isSelf && me?.authenticated && (
          <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
            <button className="btn btn-primary btn-sm" onClick={followUser}>
              + 关注
            </button>
            <button className="btn btn-secondary btn-sm" onClick={blockUser}>
              拉黑
            </button>
          </div>
        )}
      </div>

      <div className="card" style={{ padding: "4px 8px", marginBottom: 14, display: "flex", gap: 2 }} role="tablist">
        {([["posts", "帖子"], ["followers", "粉丝"], ["following", "关注中"]] as const).map(([k, label]) => (
          <button
            key={k}
            role="tab"
            aria-selected={tab === k}
            className="btn btn-text"
            onClick={() => { setTab(k); setPage(1); }}
            style={{
              fontWeight: tab === k ? 700 : 400,
              color: tab === k ? "var(--brand)" : "var(--text-2)",
              borderBottom: tab === k ? "2px solid var(--brand)" : "2px solid transparent",
              borderRadius: 0,
            }}
          >
            {label}
          </button>
        ))}
      </div>

      {tab === "posts" && (
        <PostList
          posts={posts.data?.items}
          loading={posts.isLoading}
          error={posts.error as Error | null}
          onRetry={() => posts.refetch()}
          page={page}
          pageSize={20}
          total={posts.data?.total ?? 0}
          onPageChange={setPage}
          emptyTitle="TA 还没有公开的帖子"
        />
      )}
      {tab === "followers" && <UserMiniList data={followers.data} loading={followers.isLoading} emptyTitle="还没有粉丝" />}
      {tab === "following" && <UserMiniList data={following.data} loading={following.isLoading} emptyTitle="还没有关注任何人" />}
    </div>
  );
}

function UserMiniList({
  data,
  loading,
  emptyTitle,
}: {
  data?: PageData<{ id: string; username: string; display_name: string; avatar_key: string; bio: string }>;
  loading: boolean;
  emptyTitle: string;
}) {
  if (loading) return <ListSkeleton count={2} />;
  if (!data || data.items.length === 0) return <EmptyState title={emptyTitle} />;
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
      {data.items.map((u) => (
        <Link key={u.id} href={`/u/${u.username}`} className="card" style={{ padding: 14, display: "flex", gap: 12, alignItems: "center" }}>
          <Avatar name={u.display_name || u.username} avatarKey={u.avatar_key} size={40} />
          <div>
            <div style={{ fontWeight: 600 }}>{u.display_name}</div>
            <div style={{ fontSize: 13, color: "var(--text-3)" }}>@{u.username}</div>
          </div>
        </Link>
      ))}
    </div>
  );
}
