"use client";

// 社区活动列表：撕页日历日期卡、容量进度条与微交互反馈。
import Link from "next/link";
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { motion } from "framer-motion";
import { api, type PageData } from "@/lib/api";
import type { Activity } from "@/lib/types";
import { ListSkeleton, ErrorState, EmptyState, PageHeader, Pagination } from "@/components/ui/States";
import { fmtRange } from "@/components/activities/ActivityDetail";
import { IconAward, IconChevronRight, IconClock, IconMapPin } from "@/components/ui/Icons";

function parseDateBadge(iso: string) {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return { month: "活动", day: "--" };
  return { month: `${d.getMonth() + 1}月`, day: String(d.getDate()).padStart(2, "0") };
}

export default function ActivitiesPage() {
  const [page, setPage] = useState(1);
  const list = useQuery({
    queryKey: ["activities", page],
    queryFn: () => api.get<PageData<Activity>>(`/activities?page=${page}&page_size=20`),
  });

  return (
    <div style={{ maxWidth: 860, margin: "0 auto" }}>
      <PageHeader
        eyebrow="Community Events"
        title="社区活动"
        subtitle="线上深度分享与线下交流聚会，报名出席还可领取社区积分奖励"
      />
      {list.isLoading && <ListSkeleton count={3} />}
      {list.error && <ErrorState message={(list.error as Error).message} onRetry={() => list.refetch()} />}
      {list.data && list.data.items.length === 0 && (
        <EmptyState title="暂无进行中的活动" description="有新活动发布时会第一时间展示在这里" />
      )}
      <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
        {(list.data?.items ?? []).map((a, idx) => {
          const badge = parseDateBadge(a.starts_at);
          const full = a.capacity > 0 && a.participant_count >= a.capacity;
          const pct = a.capacity > 0 ? Math.min(100, Math.round((a.participant_count / a.capacity) * 100)) : 0;
          return (
            <motion.div
              key={a.id}
              initial={{ opacity: 0, y: 8 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.24, delay: Math.min(idx, 6) * 0.04 }}
            >
              <Link
                href={`/activities/${a.id}`}
                className="card card-interactive"
                style={{ padding: "18px 20px", display: "flex", gap: 16, alignItems: "center" }}
              >
                {/* 撕页日历日期卡 */}
                <div
                  style={{
                    width: 56,
                    borderRadius: 12,
                    border: "1px solid var(--border-strong)",
                    overflow: "hidden",
                    textAlign: "center",
                    flexShrink: 0,
                    background: "var(--bg-card)",
                    boxShadow: "var(--shadow-card)",
                  }}
                  aria-hidden
                >
                  <div
                    style={{
                      background: "var(--brand-gradient)",
                      color: "#fff",
                      fontSize: 11,
                      fontWeight: 650,
                      padding: "2px 0",
                    }}
                  >
                    {badge.month}
                  </div>
                  <div className="tabular-nums" style={{ fontSize: 20, fontWeight: 780, lineHeight: 1.45, color: "var(--text-1)" }}>
                    {badge.day}
                  </div>
                </div>

                <div style={{ flex: 1, minWidth: 0 }}>
                  <div style={{ fontWeight: 680, fontSize: 16, letterSpacing: "-0.015em", color: "var(--text-1)" }}>
                    {a.title}
                  </div>
                  <div
                    style={{
                      fontSize: 13,
                      color: "var(--text-3)",
                      marginTop: 5,
                      display: "flex",
                      gap: 12,
                      flexWrap: "wrap",
                      alignItems: "center",
                    }}
                  >
                    <span style={{ display: "inline-flex", alignItems: "center", gap: 4 }}>
                      <IconClock size={13} /> {fmtRange(a)}
                    </span>
                    {a.location && (
                      <span style={{ display: "inline-flex", alignItems: "center", gap: 4 }}>
                        <IconMapPin size={13} /> {a.location}
                      </span>
                    )}
                  </div>
                  <div style={{ display: "flex", gap: 8, marginTop: 10, flexWrap: "wrap", alignItems: "center" }}>
                    <span className="tag tag-outline tabular-nums">
                      {a.participant_count} / {a.capacity || "∞"} 人报名
                    </span>
                    {a.reward_points > 0 && (
                      <span className="tag tag-success tabular-nums">
                        <IconAward size={11} /> +{a.reward_points} 积分
                      </span>
                    )}
                    {full && <span className="tag tag-warning">名额已满</span>}
                    {a.capacity > 0 && (
                      <div
                        className="hide-mobile"
                        style={{
                          width: 84,
                          height: 5,
                          borderRadius: 999,
                          background: "var(--bg-inset)",
                          overflow: "hidden",
                        }}
                      >
                        <div
                          style={{
                            width: `${pct}%`,
                            height: "100%",
                            borderRadius: 999,
                            background: full ? "var(--warning)" : "var(--brand-gradient)",
                          }}
                        />
                      </div>
                    )}
                  </div>
                </div>

                <IconChevronRight size={17} className="hide-mobile" style={{ color: "var(--text-3)", flexShrink: 0 }} />
              </Link>
            </motion.div>
          );
        })}
      </div>
      {list.data && list.data.total > 0 && (
        <Pagination page={page} pageSize={20} total={list.data.total} onChange={setPage} />
      )}
    </div>
  );
}
