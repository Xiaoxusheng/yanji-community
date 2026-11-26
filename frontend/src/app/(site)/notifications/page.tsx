"use client";

// 通知中心：全部/未读筛选、类型过滤、一键已读与快捷跳转。
import Link from "next/link";
import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, timeAgo, type PageData } from "@/lib/api";
import type { NotificationItem } from "@/lib/types";
import { ListSkeleton, ErrorState, EmptyState, PageHeader, Pagination } from "@/components/ui/States";
import { AuthShell } from "@/components/auth/forms";
import { useMe } from "@/lib/auth";
import { toast } from "@/lib/toast";
import { IconCheck, IconTrash } from "@/components/ui/Icons";

const typeLabels: Record<string, string> = {
  comment: "评论",
  reply: "回复",
  mention: "提及",
  like_post: "点赞",
  like_comment: "点赞",
  follow: "关注",
  message: "私信",
  moderation: "管理",
  system: "系统",
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
        <p style={{ marginBottom: 16, color: "var(--text-2)" }}>登录后查看通知提醒。</p>
        <Link href="/login" className="btn btn-primary">去登录</Link>
      </AuthShell>
    );
  }

  const markAll = async () => {
    try {
      await api.post("/me/notifications/read-all");
      toast("success", "已全部标记为已读");
      qc.invalidateQueries({ queryKey: ["notifications"] });
    } catch (e) {
      toast("error", (e as Error).message);
    }
  };
  const markOne = async (id: number) => {
    try {
      await api.post(`/me/notifications/${id}/read`);
      qc.invalidateQueries({ queryKey: ["notifications"] });
    } catch {
      /* 静默 */
    }
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
    if (n.type === "message") return "/messages";
    if (n.resource_type === "post" && n.resource_public_id) return `/p/${n.resource_public_id}`;
    if (n.resource_type === "comment" && n.resource_public_id) return `/p/${n.resource_public_id}`;
    return "/notifications";
  };

  return (
    <div style={{ maxWidth: 740, margin: "0 auto" }}>
      <PageHeader
        title="通知中心"
        subtitle="新回复、赞同、私信与管理消息汇总"
        action={
          <div style={{ display: "flex", gap: 8, flexWrap: "wrap", alignItems: "center" }}>
            <div className="seg-tabs">
              <button
                type="button"
                className={`seg-tab ${!unreadOnly ? "is-active" : ""}`}
                style={{ padding: "5px 10px", fontSize: 12.5 }}
                onClick={() => {
                  setUnreadOnly(false);
                  setPage(1);
                }}
              >
                全部
              </button>
              <button
                type="button"
                className={`seg-tab ${unreadOnly ? "is-active" : ""}`}
                style={{ padding: "5px 10px", fontSize: 12.5 }}
                onClick={() => {
                  setUnreadOnly(true);
                  setPage(1);
                }}
              >
                未读
              </button>
            </div>
            <select
              className="select"
              style={{ width: 116, padding: "6px 10px", fontSize: 13 }}
              value={type}
              onChange={(e) => {
                setType(e.target.value);
                setPage(1);
              }}
              aria-label="通知类型"
            >
              <option value="">全部类型</option>
              {Object.entries(typeLabels).map(([k, v]) => (
                <option key={k} value={k}>{v}</option>
              ))}
            </select>
            <button className="btn btn-secondary btn-sm" onClick={markAll}>
              全部已读
            </button>
          </div>
        }
      />

      {list.isLoading && <ListSkeleton />}
      {list.error && <ErrorState message={(list.error as Error).message} onRetry={() => list.refetch()} />}
      {list.data && list.data.items.length === 0 && (
        <EmptyState title="暂无通知" description="收到评论、点赞、关注或私信时会提醒你" />
      )}
      <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
        {(list.data?.items ?? []).map((n) => (
          <div
            key={n.id}
            className="card card-interactive"
            style={{
              padding: "14px 16px",
              display: "flex",
              gap: 12,
              alignItems: "flex-start",
              borderLeft: n.read_at ? undefined : "3px solid var(--brand)",
            }}
          >
            <div style={{ flex: 1, minWidth: 0 }}>
              <div style={{ display: "flex", gap: 8, alignItems: "center", flexWrap: "wrap" }}>
                {!n.read_at && (
                  <span style={{ width: 6, height: 6, borderRadius: 999, background: "var(--brand)" }} aria-label="未读" />
                )}
                <span className="tag tag-outline">{typeLabels[n.type] ?? n.type}</span>
                <span className="tabular-nums" style={{ fontSize: 12, color: "var(--text-3)" }}>
                  {timeAgo(n.created_at)}
                </span>
                {n.actor_name && <span style={{ fontSize: 12, color: "var(--text-3)" }}>· {n.actor_name}</span>}
              </div>
              <div style={{ marginTop: 6, fontSize: 14.5, fontWeight: n.read_at ? 500 : 600 }}>
                {n.resource_public_id || n.type === "message" ? (
                  <Link href={href(n)} onClick={() => !n.read_at && markOne(n.id)}>
                    {n.title}
                  </Link>
                ) : (
                  n.title
                )}
              </div>
              {n.body && <div style={{ fontSize: 13, color: "var(--text-2)", marginTop: 3 }}>{n.body}</div>}
            </div>
            <div style={{ display: "flex", gap: 2 }}>
              {!n.read_at && (
                <button className="btn btn-icon" style={{ width: 30, height: 30 }} onClick={() => markOne(n.id)} aria-label="标记已读" title="标记已读">
                  <IconCheck size={14} />
                </button>
              )}
              <button className="btn btn-icon" style={{ width: 30, height: 30, color: "var(--danger)" }} onClick={() => removeOne(n.id)} aria-label="删除通知" title="删除通知">
                <IconTrash size={14} />
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
