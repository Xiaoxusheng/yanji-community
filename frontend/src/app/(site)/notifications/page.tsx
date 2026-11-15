"use client";

// 通知中心：全部/未读、类型筛选、已读、删除。
import Link from "next/link";
import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, timeAgo, type PageData } from "@/lib/api";
import type { NotificationItem } from "@/lib/types";
import { ListSkeleton, ErrorState, EmptyState, Pagination } from "@/components/ui/States";
import { AuthShell } from "@/components/auth/forms";
import { useMe } from "@/lib/auth";
import { toast } from "@/lib/toast";

const typeLabels: Record<string, string> = {
  comment: "评论", reply: "回复", mention: "提及",
  like_post: "点赞", like_comment: "点赞", follow: "关注",
  moderation: "管理", system: "系统",
};

export default function NotificationsPage() {
  const { data: me, isLoading: meLoading } = useMe();
  const qc = useQueryClient();
  const [unreadOnly, setUnreadOnly] = useState(false);
  const [type, setType] = useState("");
  const [page, setPage] = useState(1);

  const list = useQuery({
    enabled: !!me?.authenticated,
    queryKey: ["notifications", "list", unreadOnly, type, page],
    queryFn: () => {
      let q = `/me/notifications?page=${page}&page_size=20`;
      if (unreadOnly) q += "&unread=true";
      if (type) q += `&type=${type}`;
      return api.get<PageData<NotificationItem>>(q);
    },
  });

  if (meLoading) return <ListSkeleton count={2} />;
  if (me && !me.authenticated) {
    return (
      <AuthShell title="请先登录">
        <p style={{ marginBottom: 16 }}>登录后查看你的通知。</p>
        <Link href="/login" className="btn btn-primary">去登录</Link>
      </AuthShell>
    );
  }

  const markAll = async () => {
    try {
      await api.post("/me/notifications/read-all");
      toast("success", "全部已读");
      qc.invalidateQueries({ queryKey: ["notifications"] });
    } catch (e) {
      toast("error", (e as Error).message);
    }
  };
  const markOne = async (id: number) => {
    try {
      await api.post(`/me/notifications/${id}/read`);
      qc.invalidateQueries({ queryKey: ["notifications"] });
    } catch { /* 静默 */ }
  };
  const removeOne = async (id: number) => {
    try {
      await api.delete(`/me/notifications/${id}`);
      qc.invalidateQueries({ queryKey: ["notifications"] });
    } catch (e) {
      toast("error", (e as Error).message);
    }
  };

  const href = (n: NotificationItem) => {
    if (n.resource_type === "post" && n.resource_public_id) return `/p/${n.resource_public_id}`;
    if (n.resource_type === "comment" && n.resource_public_id) return `/p/${n.resource_public_id}`;
    return "/notifications";
  };

  return (
    <div style={{ maxWidth: 720, margin: "0 auto" }}>
      <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 14, flexWrap: "wrap" }}>
        <h1 style={{ fontSize: 20, fontWeight: 700 }}>通知中心</h1>
        <span style={{ marginLeft: "auto" }} />
        <button className={`btn btn-sm ${unreadOnly ? "btn-primary" : "btn-secondary"}`} onClick={() => { setUnreadOnly(!unreadOnly); setPage(1); }}>
          {unreadOnly ? "仅未读" : "全部"}
        </button>
        <select className="select" style={{ width: 120, padding: "6px 10px" }} value={type} onChange={(e) => { setType(e.target.value); setPage(1); }} aria-label="通知类型">
          <option value="">全部类型</option>
          {Object.entries(typeLabels).map(([k, v]) => (
            <option key={k} value={k}>{v}</option>
          ))}
        </select>
        <button className="btn btn-secondary btn-sm" onClick={markAll}>
          全部已读
        </button>
      </div>

      {list.isLoading && <ListSkeleton />}
      {list.error && <ErrorState message={(list.error as Error).message} onRetry={() => list.refetch()} />}
      {list.data && list.data.items.length === 0 && (
        <EmptyState title="暂无通知" description="有新评论、点赞或关注时会出现在这里" />
      )}
      <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
        {(list.data?.items ?? []).map((n) => (
          <div
            key={n.id}
            className="card"
            style={{
              padding: 14,
              display: "flex",
              gap: 10,
              alignItems: "flex-start",
              borderLeft: n.read_at ? "3px solid transparent" : "3px solid var(--brand)",
              background: n.read_at ? "var(--bg-card)" : "var(--brand-soft)",
            }}
          >
            <div style={{ flex: 1, minWidth: 0 }}>
              <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
                <span className="tag">{typeLabels[n.type] ?? n.type}</span>
                <span style={{ fontSize: 12.5, color: "var(--text-3)" }}>{timeAgo(n.created_at)}</span>
                {n.actor_name && <span style={{ fontSize: 12.5, color: "var(--text-3)" }}>来自 {n.actor_name}</span>}
              </div>
              <div style={{ marginTop: 6, fontSize: 14.5, fontWeight: 500 }}>
                {n.resource_public_id ? (
                  <Link href={href(n)} onClick={() => !n.read_at && markOne(n.id)}>
                    {n.title}
                  </Link>
                ) : (
                  n.title
                )}
              </div>
              {n.body && <div style={{ fontSize: 13, color: "var(--text-3)", marginTop: 2 }}>{n.body}</div>}
            </div>
            <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
              {!n.read_at && (
                <button className="btn btn-text btn-sm" onClick={() => markOne(n.id)} aria-label="标记已读">
                  ✓
                </button>
              )}
              <button className="btn btn-text btn-sm" style={{ color: "var(--danger)" }} onClick={() => removeOne(n.id)} aria-label="删除通知">
                ×
              </button>
            </div>
          </div>
        ))}
      </div>
      {list.data && list.data.total > 0 && (
        <Pagination page={page} pageSize={20} total={list.data.total} onChange={setPage} />
      )}
    </div>
  );
}
