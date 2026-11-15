"use client";

// 积分中心：账户/等级进度/每日签到/积分流水/勋章墙。
import Link from "next/link";
import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, timeAgo, type PageData } from "@/lib/api";
import type { PointAccount, PointTransaction, Badge } from "@/lib/types";
import { ListSkeleton, ErrorState, EmptyState, Pagination } from "@/components/ui/States";
import { AuthShell } from "@/components/auth/forms";
import { useMe } from "@/lib/auth";
import { toast } from "@/lib/toast";

const sourceLabels: Record<string, string> = {
  registration: "注册奖励", sign_in: "每日签到", post_created: "发布内容",
  post_liked: "内容获赞", comment_liked: "评论获赞", answer_accepted: "回答被采纳",
  activity_attended: "参加活动", badge: "勋章奖励", admin_adjust: "管理调整",
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
        <p style={{ marginBottom: 16 }}>登录后查看积分与签到。</p>
        <Link href="/login" className="btn btn-primary">去登录</Link>
      </AuthShell>
    );
  }

  const checkIn = async () => {
    setCheckingIn(true);
    try {
      const res = await api.post<{ awarded: number; already: boolean; message: string; streak_days?: number }>("/me/check-in");
      if (res.already) {
        toast("info", res.message);
      } else {
        toast("success", `签到成功 +${res.awarded} 积分`);
      }
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
    <div style={{ maxWidth: 820, margin: "0 auto" }}>
      <h1 style={{ fontSize: 20, fontWeight: 700, marginBottom: 14 }}>积分中心</h1>

      {account.isLoading && <ListSkeleton count={1} />}
      {account.error && <ErrorState message={(account.error as Error).message} onRetry={() => account.refetch()} />}

      {acc && (
        <div style={{ display: "grid", gridTemplateColumns: "minmax(0,1fr) 220px", gap: 14, marginBottom: 14 }}>
          <section className="card" style={{ padding: 18 }}>
            <div style={{ display: "flex", alignItems: "baseline", gap: 10 }}>
              <span style={{ fontSize: 32, fontWeight: 800, color: "var(--brand)" }}>{acc.balance}</span>
              <span style={{ color: "var(--text-3)", fontSize: 13 }}>当前积分 · 累计获得 {acc.total_earned}</span>
            </div>
            <div style={{ marginTop: 12 }}>
              <div style={{ display: "flex", justifyContent: "space-between", fontSize: 13, marginBottom: 6 }}>
                <span className="tag tag-brand">
                  Lv.{level!.level} {level!.name}
                </span>
                {next ? (
                  <span style={{ color: "var(--text-3)" }}>
                    距离 Lv.{next.level} {next.name} 还需 {Math.max(0, next.min_points - acc.total_earned)} 积分
                  </span>
                ) : (
                  <span style={{ color: "var(--text-3)" }}>已达最高等级 🎉</span>
                )}
              </div>
              <div style={{ height: 8, background: "var(--bg-inset)", borderRadius: 999, overflow: "hidden" }}>
                <div style={{ width: `${progress}%`, height: "100%", background: "var(--brand)", borderRadius: 999, transition: "width .3s ease" }} />
              </div>
            </div>
          </section>
          <section className="card" style={{ padding: 18, textAlign: "center" }}>
            <div style={{ fontSize: 13, color: "var(--text-3)", marginBottom: 6 }}>
              连续签到 <b style={{ color: "var(--text-1)", fontSize: 18 }}>{acc.streak_days}</b> 天
            </div>
            <button
              className={`btn ${acc.today_signed ? "btn-secondary" : "btn-primary"}`}
              onClick={checkIn}
              disabled={checkingIn || acc.today_signed}
              style={{ width: "100%" }}
            >
              {acc.today_signed ? "今日已签到 ✓" : checkingIn ? "签到中…" : "每日签到 +5"}
            </button>
            <div style={{ fontSize: 12, color: "var(--text-3)", marginTop: 8 }}>累计签到 {acc.signin_days} 天</div>
          </section>
        </div>
      )}

      {/* 勋章墙 */}
      <section className="card" style={{ padding: 18, marginBottom: 14 }}>
        <h2 style={{ fontSize: 15, fontWeight: 700, marginBottom: 12 }}>🏅 勋章</h2>
        <div style={{ display: "flex", flexWrap: "wrap", gap: 10 }}>
          {(badges.data?.items ?? []).map((b) => {
            const owned = myBadgeCodes.has(b.code);
            return (
              <div
                key={b.id}
                title={b.description}
                style={{
                  display: "flex", flexDirection: "column", alignItems: "center", gap: 4,
                  width: 96, padding: "12px 6px", borderRadius: 10,
                  border: owned ? "1.5px solid var(--brand)" : "1px dashed var(--border)",
                  background: owned ? "var(--brand-soft)" : "transparent",
                  opacity: owned ? 1 : 0.45,
                }}
              >
                <span style={{ fontSize: 24 }}>{b.icon || "🎖"}</span>
                <span style={{ fontSize: 12.5, fontWeight: 600 }}>{b.name}</span>
                {owned ? <span style={{ fontSize: 11, color: "var(--brand)" }}>已获得</span> : <span style={{ fontSize: 11, color: "var(--text-3)" }}>未获得</span>}
              </div>
            );
          })}
        </div>
      </section>

      {/* 流水 */}
      <section>
        <h2 style={{ fontSize: 15, fontWeight: 700, marginBottom: 10 }}>积分流水</h2>
        {transactions.isLoading && <ListSkeleton count={2} />}
        {transactions.error && <ErrorState message={(transactions.error as Error).message} onRetry={() => transactions.refetch()} />}
        {transactions.data && transactions.data.items.length === 0 && (
          <EmptyState title="还没有积分记录" description="发帖、签到、被点赞都可以获得积分" />
        )}
        <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
          {(transactions.data?.items ?? []).map((t) => (
            <div key={t.id} className="card" style={{ padding: 12, display: "flex", alignItems: "center", gap: 10 }}>
              <div style={{ flex: 1 }}>
                <div style={{ fontSize: 14, fontWeight: 600 }}>{sourceLabels[t.source_type] ?? t.source_type}</div>
                <div style={{ fontSize: 12.5, color: "var(--text-3)" }}>
                  {t.note} · {timeAgo(t.created_at)}
                </div>
              </div>
              <div style={{ textAlign: "right" }}>
                <div style={{ fontWeight: 700, color: t.amount > 0 ? "var(--success)" : "var(--danger)" }}>
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
