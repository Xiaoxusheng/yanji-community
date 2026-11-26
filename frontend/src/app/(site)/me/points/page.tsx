"use client";

// 积分中心：账户/等级进度/每日签到/勋章墙/积分流水。
import Link from "next/link";
import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, timeAgo, type PageData } from "@/lib/api";
import type { PointAccount, PointTransaction, Badge } from "@/lib/types";
import { ListSkeleton, ErrorState, EmptyState, PageHeader, Pagination } from "@/components/ui/States";
import { AuthShell } from "@/components/auth/forms";
import { useMe } from "@/lib/auth";
import { toast } from "@/lib/toast";
import { IconAward, IconCheck } from "@/components/ui/Icons";

const sourceLabels: Record<string, string> = {
  registration: "注册奖励",
  sign_in: "每日签到",
  post_created: "发布内容",
  post_liked: "内容获赞",
  comment_liked: "评论获赞",
  answer_accepted: "回答被采纳",
  activity_attended: "参加活动",
  badge: "勋章奖励",
  admin_adjust: "管理调整",
};

export default function PointsPage() {
  const { data: me, isLoading: meLoading } = useMe();
  const qc = useQueryClient();
  const [page, setPage] = useState(1);
  const [checkingIn, setCheckingIn] = useState(false);

  const account = useQuery({
    enabled: !!me?.authenticated,
    queryKey: ["points", "account"],
    queryFn: () => api.get<{ account: PointAccount }>("/me/points"),
  });
  const transactions = useQuery({
    enabled: !!me?.authenticated,
    queryKey: ["points", "transactions", page],
    queryFn: () => api.get<PageData<PointTransaction>>(`/me/points/transactions?page=${page}&page_size=20`),
  });
  const badges = useQuery({
    enabled: !!me?.authenticated,
    queryKey: ["my-badges"],
    queryFn: () => api.get<{ items: Badge[] }>("/badges"),
  });
  const myBadges = useQuery({
    enabled: !!me?.authenticated,
    queryKey: ["me-badges"],
    queryFn: () => api.get<{ items: Badge[] }>(`/users/${me?.user?.username ?? ""}/badges`),
  });

  if (meLoading) return null;
  if (me && !me.authenticated) {
    return (
      <AuthShell title="请先登录">
        <p style={{ marginBottom: 16, color: "var(--text-2)" }}>登录后查看积分账户与每日签到。</p>
        <Link href="/login" className="btn btn-primary">去登录</Link>
      </AuthShell>
    );
  }

  const checkIn = async () => {
    setCheckingIn(true);
    try {
      const res = await api.post<{ awarded: number; already: boolean; message: string }>("/me/check-in");
      if (res.already) toast("info", res.message);
      else toast("success", `签到成功 +${res.awarded} 积分`);
      qc.invalidateQueries({ queryKey: ["points"] });
    } catch (e) {
      toast("error", (e as Error).message);
    } finally {
      setCheckingIn(false);
    }
  };

  const acc = account.data?.account;
  const level = acc?.level;
  const next = acc?.next_level;
  const progress = level && next && next.min_points > level.min_points
    ? Math.min(100, Math.round(((acc!.total_earned - level.min_points) / (next.min_points - level.min_points)) * 100))
    : 100;
  const myBadgeCodes = new Set((myBadges.data?.items ?? []).map((b) => b.code));

  return (
    <div style={{ maxWidth: 840, margin: "0 auto" }}>
      <PageHeader title="积分中心" subtitle="通过每日签到、优质发布与互动积累社区成长" />

      {account.isLoading && <ListSkeleton count={1} />}
      {account.error && <ErrorState message={(account.error as Error).message} onRetry={() => account.refetch()} />}

      {acc && (
        <div className="layout-split-2" style={{ marginBottom: 16 }}>
          <section className="surface-hero" style={{ padding: 24 }}>
            <div style={{ fontSize: 11.5, fontWeight: 650, letterSpacing: "0.06em", color: "var(--brand)", marginBottom: 6 }}>
              POINTS BALANCE
            </div>
            <div style={{ display: "flex", alignItems: "baseline", gap: 12, flexWrap: "wrap" }}>
              <span className="tabular-nums" style={{ fontSize: 38, fontWeight: 780, letterSpacing: "-0.035em", color: "var(--text-1)" }}>
                {acc.balance}
              </span>
              <span className="tabular-nums" style={{ color: "var(--text-3)", fontSize: 13 }}>
                当前可用积分 · 累计获得 {acc.total_earned}
              </span>
            </div>
            <div style={{ marginTop: 16 }}>
              <div style={{ display: "flex", justifyContent: "space-between", fontSize: 12.5, marginBottom: 8 }}>
                <span className="tag tag-brand">
                  Lv.{level!.level} {level!.name}
                </span>
                {next ? (
                  <span className="tabular-nums" style={{ color: "var(--text-3)" }}>
                    距离 Lv.{next.level} {next.name} 还需 {Math.max(0, next.min_points - acc.total_earned)} 积分
                  </span>
                ) : (
                  <span style={{ color: "var(--text-3)" }}>已达最高等级</span>
                )}
              </div>
              <div style={{ height: 7, background: "var(--bg-inset)", borderRadius: 999, overflow: "hidden", border: "1px solid var(--border)" }}>
                <div style={{ width: `${progress}%`, height: "100%", background: "var(--brand-gradient)", borderRadius: 999, transition: "width .35s ease" }} />
              </div>
            </div>
          </section>

          <section className="surface-spotlight" style={{ padding: 22, display: "flex", flexDirection: "column", justifyContent: "center", textAlign: "center" }}>
            <div className="tabular-nums" style={{ fontSize: 13, color: "var(--text-3)", marginBottom: 10 }}>
              连续签到 <b style={{ color: "var(--text-1)", fontSize: 22 }}>{acc.streak_days}</b> 天
            </div>
            <button
              className={`btn ${acc.today_signed ? "btn-secondary" : "btn-brand"}`}
              onClick={checkIn}
              disabled={checkingIn || acc.today_signed}
              style={{ width: "100%" }}
            >
              {acc.today_signed ? (
                <>
                  <IconCheck size={14} /> 今日已签到
                </>
              ) : checkingIn ? (
                "签到中…"
              ) : (
                "每日签到 +5"
              )}
            </button>
            <div className="tabular-nums" style={{ fontSize: 12, color: "var(--text-3)", marginTop: 10 }}>
              累计签到 {acc.signin_days} 天
            </div>
          </section>
        </div>
      )}

      {/* 勋章墙 */}
      <section className="card" style={{ padding: 18, marginBottom: 14 }}>
        <h2 style={{ fontSize: 14.5, fontWeight: 650, marginBottom: 12, display: "inline-flex", alignItems: "center", gap: 6 }}>
          <IconAward size={15} /> 社区勋章
        </h2>
        <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(136px, 1fr))", gap: 10 }}>
          {(badges.data?.items ?? []).map((b) => {
            const owned = myBadgeCodes.has(b.code);
            return (
              <div
                key={b.id}
                title={b.description}
                style={{
                  display: "flex",
                  flexDirection: "column",
                  alignItems: "center",
                  gap: 4,
                  padding: "14px 10px",
                  borderRadius: 10,
                  border: `1px ${owned ? "solid var(--brand)" : "dashed var(--border-strong)"}`,
                  background: owned ? "var(--brand-soft)" : "var(--bg-inset)",
                  opacity: owned ? 1 : 0.55,
                  textAlign: "center",
                }}
              >
                <span style={{ fontSize: 22 }}>{b.icon || "•"}</span>
                <span style={{ fontSize: 13, fontWeight: 600 }}>{b.name}</span>
                <span style={{ fontSize: 11.5, color: owned ? "var(--brand)" : "var(--text-3)" }}>
                  {owned ? "已解锁" : b.description || "未解锁"}
                </span>
              </div>
            );
          })}
        </div>
      </section>

      {/* 流水 */}
      <section>
        <h2 style={{ fontSize: 15, fontWeight: 650, marginBottom: 10 }}>积分流水明细</h2>
        {transactions.isLoading && <ListSkeleton count={2} />}
        {transactions.error && <ErrorState message={(transactions.error as Error).message} onRetry={() => transactions.refetch()} />}
        {transactions.data && transactions.data.items.length === 0 && (
          <EmptyState title="还没有积分记录" description="发帖、签到或获赞都会记录在这里" />
        )}
        <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
          {(transactions.data?.items ?? []).map((t) => (
            <div key={t.id} className="card" style={{ padding: "12px 16px", display: "flex", alignItems: "center", gap: 10 }}>
              <div style={{ flex: 1 }}>
                <div style={{ fontSize: 14, fontWeight: 600 }}>{sourceLabels[t.source_type] ?? t.source_type}</div>
                <div className="tabular-nums" style={{ fontSize: 12.5, color: "var(--text-3)", marginTop: 2 }}>
                  {t.note} · {timeAgo(t.created_at)}
                </div>
              </div>
              <div className="tabular-nums" style={{ textAlign: "right" }}>
                <div style={{ fontWeight: 650, color: t.amount > 0 ? "var(--success)" : "var(--danger)" }}>
                  {t.amount > 0 ? "+" : ""}{t.amount}
                </div>
                <div style={{ fontSize: 12, color: "var(--text-3)" }}>余额 {t.balance_after}</div>
              </div>
            </div>
          ))}
        </div>
        {transactions.data && transactions.data.total > 0 && (
          <Pagination page={page} pageSize={20} total={transactions.data.total} onChange={setPage} />
        )}
      </section>
    </div>
  );
}
