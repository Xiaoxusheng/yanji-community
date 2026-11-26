"use client";

// 用户公开主页：资料、勋章、帖子、粉丝/关注与本人多类型关注管理。
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, fmtCount, safeExternalUrl, timeAgo, type PageData } from "@/lib/api";
import type { Badge, PostItem, ProfilePublic } from "@/lib/types";
import { PostList } from "@/components/post/PostCard";
import { Avatar } from "@/components/ui/Avatar";
import { ConfirmDialog } from "@/components/ui/Dialog";
import { ListSkeleton, ErrorState, EmptyState, Pagination } from "@/components/ui/States";
import { useMe } from "@/lib/auth";
import { toast } from "@/lib/toast";
import { IconCheck, IconLink, IconLock, IconMapPin, IconSettings } from "@/components/ui/Icons";

export default function UserProfilePage() {
  const params = useParams<{ username: string }>();
  const username = params.username;
  const router = useRouter();
  const qc = useQueryClient();
  const { data: me } = useMe();
  const [tab, setTab] = useState<"posts" | "followers" | "following">("posts");
  const [followType, setFollowType] = useState<"user" | "category" | "tag">("user");
  const [page, setPage] = useState(1);
  const [confirmBlockOpen, setConfirmBlockOpen] = useState(false);

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
    queryFn: () =>
      api.get<PageData<{ id: string; username: string; display_name: string; avatar_key: string; bio: string }>>(
        `/users/${username}/followers?page=${page}&page_size=20`,
      ),
  });

  const isSelf = me?.authenticated && me.user?.username === username;

  const following = useQuery({
    enabled: tab === "following" && (!isSelf || followType === "user"),
    queryKey: ["user-following", username, page],
    queryFn: () =>
      api.get<PageData<{ id: string; username: string; display_name: string; avatar_key: string; bio: string }>>(
        `/users/${username}/following?page=${page}&page_size=20`,
      ),
  });

  const myFollowingExtra = useQuery({
    enabled: tab === "following" && !!isSelf && followType !== "user",
    queryKey: ["me-following", followType, page],
    queryFn: () =>
      api.get<PageData<{ id: number; slug: string; name: string; description?: string; post_count?: number }>>(
        `/me/following?type=${followType}&page=${page}&page_size=20`,
      ),
  });

  const badges = useQuery({
    queryKey: ["profile-badges", username],
    queryFn: () => api.get<{ items: Badge[] }>(`/users/${username}/badges`),
  });

  const toggleFollow = async () => {
    if (!profile.data) return;
    const isFollowing = !!profile.data.profile.is_following;
    try {
      if (isFollowing) {
        await api.delete(`/users/${profile.data.profile.id}/follow`);
        toast("success", "已取消关注");
      } else {
        await api.post(`/users/${profile.data.profile.id}/follow`);
        toast("success", "已关注");
      }
      qc.invalidateQueries({ queryKey: ["profile", username] });
    } catch (e) {
      toast("error", (e as Error).message);
    }
  };

  const toggleBlock = async () => {
    if (!profile.data) return;
    const isBlocked = !!profile.data.profile.is_blocked;
    try {
      if (isBlocked) {
        await api.delete(`/users/${profile.data.profile.id}/block`);
        toast("success", "已取消拉黑");
      } else {
        await api.post(`/users/${profile.data.profile.id}/block`);
        toast("success", "已拉黑该用户");
      }
      setConfirmBlockOpen(false);
      qc.invalidateQueries({ queryKey: ["profile", username] });
      qc.invalidateQueries({ queryKey: ["user-posts", username] });
    } catch (e) {
      toast("error", (e as Error).message);
    }
  };

  if (profile.isLoading) return <ListSkeleton count={2} />;
  if (profile.error || !profile.data)
    return <ErrorState message={(profile.error as Error)?.message} onRetry={() => profile.refetch()} />;
  const p = profile.data.profile;
  const safeWebsite = safeExternalUrl(p.website);

  return (
    <div style={{ maxWidth: 820, margin: "0 auto" }}>
      <div className="card" style={{ overflow: "hidden", marginBottom: 18 }}>
        {/* 顶部低调氛围头图条 */}
        <div
          aria-hidden
          style={{
            height: 88,
            background:
              "radial-gradient(60% 180% at 88% 0%, rgba(99, 102, 241, 0.24), transparent 70%), radial-gradient(50% 160% at 12% 100%, rgba(59, 91, 219, 0.14), transparent 70%), var(--bg-inset)",
            borderBottom: "1px solid var(--border)",
          }}
        />
        <div style={{ padding: "0 26px 24px" }}>
          <div style={{ display: "flex", gap: 18, alignItems: "flex-end", flexWrap: "wrap", marginTop: -36 }}>
            <Avatar name={p.display_name} avatarKey={p.avatar_key} size={72} ring />
            <div style={{ flex: 1, minWidth: 200, paddingTop: 42 }}>
              <div style={{ display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap" }}>
                <h1 style={{ fontSize: 22, fontWeight: 740, letterSpacing: "-0.025em" }}>{p.display_name}</h1>
                <span style={{ color: "var(--text-3)", fontSize: 13.5 }}>@{p.username}</span>
              </div>

              <div className="tabular-nums" style={{ color: "var(--text-3)", fontSize: 12.5, marginTop: 4, display: "flex", gap: 10, flexWrap: "wrap", alignItems: "center" }}>
                <span>{timeAgo(p.created_at)}加入</span>
                {p.location && (
                  <span style={{ display: "inline-flex", alignItems: "center", gap: 3 }}>
                    <IconMapPin size={12} /> {p.location}
                  </span>
                )}
                {!p.restricted && safeWebsite && (
                  <a href={safeWebsite} target="_blank" rel="noopener noreferrer nofollow" style={{ color: "var(--brand)", display: "inline-flex", alignItems: "center", gap: 3 }}>
                    <IconLink size={12} /> {p.website}
                  </a>
                )}
              </div>
            </div>

            <div style={{ display: "flex", gap: 8, flexWrap: "wrap", alignSelf: "center", paddingTop: 38 }}>
              {isSelf && (
                <Link href="/settings" className="btn btn-secondary btn-sm">
                  <IconSettings size={14} /> 编辑资料
                </Link>
              )}
              {!isSelf && me?.authenticated && (
                <>
                  <button className={`btn btn-sm ${p.is_following ? "btn-secondary" : "btn-brand"}`} onClick={toggleFollow}>
                    {p.is_following ? (
                      <>
                        <IconCheck size={13} /> 已关注
                      </>
                    ) : (
                      "+ 关注"
                    )}
                  </button>
                  <button
                    className="btn btn-secondary btn-sm"
                    onClick={async () => {
                      try {
                        const res = await api.post<{ conversation: { id: number } }>("/conversations", { target_id: p.id });
                        router.push(`/messages?conv=${res.conversation.id}`);
                      } catch (e) {
                        toast("error", (e as Error).message);
                      }
                    }}
                  >
                    发私信
                  </button>
                  <button
                    className="btn btn-text btn-sm"
                    style={{ color: p.is_blocked ? "var(--danger)" : "var(--text-3)" }}
                    onClick={() => (p.is_blocked ? toggleBlock() : setConfirmBlockOpen(true))}
                  >
                    {p.is_blocked ? "取消拉黑" : "拉黑"}
                  </button>
                </>
              )}
            </div>
          </div>

          {p.restricted ? (
            <p style={{ fontSize: 13.5, color: "var(--text-3)", margin: "14px 0 0", display: "flex", alignItems: "center", gap: 6 }}>
              <IconLock size={14} /> 该成员已设置主页可见范围（{p.profile_visibility === "followers" ? "仅关注者可见" : "仅自己可见"}）
            </p>
          ) : (
            p.bio && <p style={{ fontSize: 14, color: "var(--text-2)", margin: "14px 0 0", lineHeight: 1.65 }}>{p.bio}</p>
          )}

          <div
            className="tabular-nums"
            style={{
              display: "grid",
              gridTemplateColumns: "repeat(4, minmax(0, 1fr))",
              gap: 10,
              marginTop: 18,
            }}
          >
            {[
              ["帖子", p.post_count],
              ["评论", p.comment_count],
              ["粉丝", p.follower_count],
              ["关注", p.following_count],
            ].map(([label, val]) => (
              <div key={String(label)} className="stat-tile">
                <div style={{ fontSize: 16, fontWeight: 720, color: "var(--text-1)" }}>{fmtCount(Number(val))}</div>
                <div style={{ fontSize: 12, color: "var(--text-3)", marginTop: 2 }}>{label}</div>
              </div>
            ))}
          </div>

          {(badges.data?.items.length ?? 0) > 0 && (
            <div style={{ display: "flex", gap: 8, flexWrap: "wrap", marginTop: 16, paddingTop: 14, borderTop: "1px solid var(--border)" }}>
              {(badges.data?.items ?? []).map((b) => (
                <span key={b.id} className="tag tag-brand" title={b.description} style={{ padding: "3.5px 11px" }}>
                  <span>{b.icon}</span>
                  <span style={{ fontWeight: 600 }}>{b.name}</span>
                </span>
              ))}
            </div>
          )}
        </div>
      </div>

      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 10, marginBottom: 14, flexWrap: "wrap" }}>
        <div className="seg-tabs" role="tablist">
          {([["posts", "发布的帖子"], ["followers", "粉丝"], ["following", "正在关注"]] as const).map(([k, label]) => (
            <button
              key={k}
              role="tab"
              aria-selected={tab === k}
              className="seg-tab"
              onClick={() => {
                setTab(k);
                setPage(1);
              }}
            >
              {label}
            </button>
          ))}
        </div>

        {tab === "following" && isSelf && (
          <div className="seg-tabs">
            {([["user", "成员"], ["category", "版块"], ["tag", "标签"]] as const).map(([k, label]) => (
              <button
                key={k}
                type="button"
                className={`seg-tab ${followType === k ? "is-active" : ""}`}
                style={{ padding: "4px 10px", fontSize: 12.5 }}
                onClick={() => {
                  setFollowType(k);
                  setPage(1);
                }}
              >
                {label}
              </button>
            ))}
          </div>
        )}
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
          emptyTitle="还没有公开发表的帖子"
        />
      )}

      {tab === "followers" && (
        <UserMiniList
          data={followers.data}
          loading={followers.isLoading}
          error={followers.error as Error | null}
          page={page}
          onPageChange={setPage}
          emptyTitle="还没有粉丝"
        />
      )}

      {tab === "following" && (!isSelf || followType === "user") && (
        <UserMiniList
          data={following.data}
          loading={following.isLoading}
          error={following.error as Error | null}
          page={page}
          onPageChange={setPage}
          emptyTitle="还没有关注任何成员"
        />
      )}

      {tab === "following" && isSelf && followType !== "user" && (
        <>
          {myFollowingExtra.isLoading && <ListSkeleton count={3} />}
          {myFollowingExtra.error && (
            <ErrorState message={(myFollowingExtra.error as Error).message} onRetry={() => myFollowingExtra.refetch()} />
          )}
          {!myFollowingExtra.isLoading && (myFollowingExtra.data?.items.length ?? 0) === 0 && (
            <EmptyState title={followType === "category" ? "还没有关注版块" : "还没有关注标签"} />
          )}
          <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
            {(myFollowingExtra.data?.items ?? []).map((it) => (
              <div key={it.id} className="card" style={{ padding: 14, display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12 }}>
                <div>
                  <Link
                    href={followType === "category" ? `/c/${it.slug}` : `/t/${it.slug}`}
                    style={{ fontWeight: 600, fontSize: 14.5 }}
                  >
                    {followType === "tag" ? `# ${it.name}` : it.name}
                  </Link>
                  {it.description && <div style={{ fontSize: 12.5, color: "var(--text-3)", marginTop: 2 }}>{it.description}</div>}
                </div>
                <button
                  className="btn btn-secondary btn-xs"
                  onClick={async () => {
                    await api.delete(`/${followType === "category" ? "categories" : "tags"}/${it.slug}/follow`);
                    toast("success", "已取消关注");
                    qc.invalidateQueries({ queryKey: ["me-following", followType] });
                  }}
                >
                  取消关注
                </button>
              </div>
            ))}
          </div>
        </>
      )}

      <ConfirmDialog
        open={confirmBlockOpen}
        title="拉黑用户"
        description={`确定拉黑 @${p.username} 吗？拉黑后双方将互不可见彼此的帖子且无法私信。`}
        confirmText="确认拉黑"
        danger
        onConfirm={toggleBlock}
        onClose={() => setConfirmBlockOpen(false)}
      />
    </div>
  );
}

function UserMiniList({
  data,
  loading,
  error,
  page,
  onPageChange,
  emptyTitle,
}: {
  data: PageData<{ id: string; username: string; display_name: string; avatar_key: string; bio: string }> | undefined;
  loading: boolean;
  error: Error | null;
  page: number;
  onPageChange: (p: number) => void;
  emptyTitle: string;
}) {
  if (loading) return <ListSkeleton count={3} />;
  if (error) return <ErrorState message={error.message} />;
  if (!data || data.items.length === 0) return <EmptyState title={emptyTitle} />;
  return (
    <>
      <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
        {data.items.map((u) => (
          <Link key={u.id} href={`/u/${u.username}`} className="card card-interactive" style={{ padding: 14, display: "flex", gap: 12, alignItems: "center" }}>
            <Avatar name={u.display_name || u.username} avatarKey={u.avatar_key} size={38} />
            <div style={{ minWidth: 0, flex: 1 }}>
              <div style={{ fontWeight: 600, fontSize: 14 }}>{u.display_name}</div>
              <div style={{ fontSize: 12.5, color: "var(--text-3)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                @{u.username} {u.bio && `· ${u.bio}`}
              </div>
            </div>
          </Link>
        ))}
      </div>
      <Pagination page={page} pageSize={20} total={data.total} onChange={onPageChange} />
    </>
  );
}
