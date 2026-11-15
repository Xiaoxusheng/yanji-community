"use client";

// 活动列表与详情报名。
import Link from "next/link";
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { api, type PageData } from "@/lib/api";
import type { Activity } from "@/lib/types";
import { ListSkeleton, ErrorState, EmptyState, Pagination } from "@/components/ui/States";
import { fmtRange } from "@/components/activities/ActivityDetail";

export default function ActivitiesPage() {
  const [page, setPage] = useState(1);
  const list = useQuery({
    queryKey: ["activities", page],
    queryFn: () => api.get<PageData<Activity>>(`/activities?page=${page}&page_size=20`),
  });

  return (
    <div style={{ maxWidth: 820, margin: "0 auto" }}>
      <div style={{ display: "flex", alignItems: "center", marginBottom: 14 }}>
        <h1 style={{ fontSize: 20, fontWeight: 700 }}>🎉 社区活动</h1>
        <span style={{ marginLeft: "auto", fontSize: 13, color: "var(--text-3)" }}>
          参加活动可获得积分奖励
        </span>
      </div>
      {list.isLoading && <ListSkeleton count={3} />}
      {list.error && <ErrorState message={(list.error as Error).message} onRetry={() => list.refetch()} />}
      {list.data && list.data.items.length === 0 && (
        <EmptyState title="暂无进行中的活动" description="活动发布后会显示在这里" />
      )}
      <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
        {(list.data?.items ?? []).map((a) => (
          <Link key={a.id} href={`/activities/${a.id}`} className="card" style={{ padding: 16, display: "flex", gap: 14, alignItems: "center" }}>
            <div
              style={{
                width: 56, height: 56, borderRadius: 12, background: "var(--brand-soft)",
                display: "flex", alignItems: "center", justifyContent: "center", fontSize: 26, flexShrink: 0,
              }}
              aria-hidden
            >
              🎯
            </div>
            <div style={{ flex: 1, minWidth: 0 }}>
              <div style={{ fontWeight: 700, fontSize: 15.5 }}>{a.title}</div>
              <div style={{ fontSize: 13, color: "var(--text-3)", marginTop: 2 }}>
                {fmtRange(a)}{a.location && ` · ${a.location}`}
              </div>
              <div style={{ display: "flex", gap: 8, marginTop: 6, flexWrap: "wrap" }}>
                <span className="tag">{a.participant_count}/{a.capacity || "∞"} 人</span>
                {a.reward_points > 0 && <span className="tag tag-success">+{a.reward_points} 积分</span>}
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
