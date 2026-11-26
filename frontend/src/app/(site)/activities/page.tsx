"use client";

// 社区活动列表。
import Link from "next/link";
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api, type PageData } from "@/lib/api";
import type { Activity } from "@/lib/types";
import { ListSkeleton, ErrorState, EmptyState, PageHeader, Pagination } from "@/components/ui/States";
import { fmtRange } from "@/components/activities/ActivityDetail";
import { IconCalendar, IconMapPin } from "@/components/ui/Icons";

export default function ActivitiesPage() {
  const [page, setPage] = useState(1);
  const list = useQuery({
    queryKey: ["activities", page],
    queryFn: () => api.get<PageData<Activity>>(`/activities?page=${page}&page_size=20`),
  });

  return (
    <div style={{ maxWidth: 840, margin: "0 auto" }}>
      <PageHeader title="社区活动" subtitle="线上交流与线下聚会，出席签到可获得额外积分" />
      {list.isLoading && <ListSkeleton count={3} />}
      {list.error && <ErrorState message={(list.error as Error).message} onRetry={() => list.refetch()} />}
      {list.data && list.data.items.length === 0 && (
        <EmptyState title="暂无进行中的活动" description="有新活动发布时会展示在这里" />
      )}
      <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
        {(list.data?.items ?? []).map((a) => (
          <Link key={a.id} href={`/activities/${a.id}`} className="card card-interactive" style={{ padding: 18, display: "flex", gap: 14, alignItems: "center" }}>
            <div
              style={{
                width: 48,
                height: 48,
                borderRadius: 12,
                background: "var(--bg-inset)",
                border: "1px solid var(--border)",
                color: "var(--text-2)",
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                flexShrink: 0,
              }}
              aria-hidden
            >
              <IconCalendar size={20} />
            </div>
            <div style={{ flex: 1, minWidth: 0 }}>
              <div style={{ fontWeight: 650, fontSize: 15.5 }}>{a.title}</div>
              <div style={{ fontSize: 13, color: "var(--text-3)", marginTop: 3, display: "flex", gap: 10, flexWrap: "wrap", alignItems: "center" }}>
                <span>{fmtRange(a)}</span>
                {a.location && (
                  <span style={{ display: "inline-flex", alignItems: "center", gap: 3 }}>
                    <IconMapPin size={12} /> {a.location}
                  </span>
                )}
              </div>
              <div style={{ display: "flex", gap: 8, marginTop: 8, flexWrap: "wrap" }}>
                <span className="tag tabular-nums">{a.participant_count}/{a.capacity || "∞"} 人报名</span>
                {a.reward_points > 0 && <span className="tag tag-success tabular-nums">+{a.reward_points} 积分</span>}
                {a.capacity > 0 && a.participant_count >= a.capacity && <span className="tag tag-warning">名额已满</span>}
              </div>
            </div>
          </Link>
        ))}
      </div>
      {list.data && list.data.total > 0 && (
        <Pagination page={page} pageSize={20} total={list.data.total} onChange={setPage} />
      )}
    </div>
  );
}
