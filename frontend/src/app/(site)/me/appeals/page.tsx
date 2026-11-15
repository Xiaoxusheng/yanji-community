"use client";

// 申诉中心：我的处罚（申诉入口）+ 申诉记录。
import Link from "next/link";
import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, timeAgo } from "@/lib/api";
import type { Sanction, Appeal } from "@/lib/types";
import { ListSkeleton, EmptyState } from "@/components/ui/States";
import { AuthShell } from "@/components/auth/forms";
import { useMe } from "@/lib/auth";
import { toast } from "@/lib/toast";

const sanctionLabels: Record<string, string> = {
  warn: "警告", mute: "禁言", post_ban: "禁止发帖", temp_ban: "临时封禁", perma_ban: "永久封禁",
};

export default function AppealsPage() {
  const { data: me, isLoading: meLoading } = useMe();
  const qc = useQueryClient();
  const [appealTarget, setAppealTarget] = useState<Sanction | null>(null);
  const [reason, setReason] = useState("");
  const [submitting, setSubmitting] = useState(false);

  const sanctions = useQuery({
    enabled: !!me?.authenticated,
    queryKey: ["me-sanctions"],
    queryFn: () => api.get<{ items: Sanction[] }>("/me/sanctions"),
  });
  const appeals = useQuery({
    enabled: !!me?.authenticated,
    queryKey: ["me-appeals"],
    queryFn: () => api.get<{ items: Appeal[] }>("/me/appeals"),
  });

  if (meLoading) return null;
  if (me && !me.authenticated) {
    return (
      <AuthShell title="请先登录">
        <p style={{ marginBottom: 16 }}>登录后查看申诉。</p>
        <Link href="/login" className="btn btn-primary">去登录</Link>
      </AuthShell>
    );
  }

  const submitAppeal = async () => {
    if (!appealTarget) return;
    if (reason.trim().length < 5) {
      toast("error", "申诉理由至少 5 个字");
      return;
    }
    setSubmitting(true);
    try {
      await api.post("/appeals", { target_type: "sanction", target_id: appealTarget.id, reason });
      toast("success", "申诉已提交");
      setAppealTarget(null);
      setReason("");
      qc.invalidateQueries({ queryKey: ["me-appeals"] });
    } catch (e) {
      toast("error", (e as Error).message);
    } finally {
      setSubmitting(false);
    }
  };

  const appealedSanctionIds = new Set(
    (appeals.data?.items ?? []).filter((a) => a.status === "pending").map((a) => a.target_id),
  );

  return (
    <div style={{ maxWidth: 760, margin: "0 auto" }}>
      <h1 style={{ fontSize: 20, fontWeight: 700, marginBottom: 6 }}>申诉中心</h1>
      <p style={{ fontSize: 13.5, color: "var(--text-3)", marginBottom: 16 }}>
        对处罚或内容处理有异议可在此提交申诉；申诉由管理团队独立处理，不会覆盖原始处理记录。
      </p>

      <h2 style={{ fontSize: 15, fontWeight: 700, marginBottom: 10 }}>我的处罚记录</h2>
      {sanctions.isLoading && <ListSkeleton count={1} />}
      {sanctions.data && sanctions.data.items.length === 0 && (
        <EmptyState title="没有处罚记录" description="保持良好的社区行为 👍" />
      )}
      <div style={{ display: "flex", flexDirection: "column", gap: 8, marginBottom: 24 }}>
        {(sanctions.data?.items ?? []).map((s) => {
          const hasPending = appealedSanctionIds.has(s.id);
          return (
            <div key={s.id} className="card" style={{ padding: 14, display: "flex", gap: 10, alignItems: "center" }}>
              <span className={`tag ${s.status === "active" ? "tag-danger" : ""}`}>{sanctionLabels[s.sanction_type] ?? s.sanction_type}</span>
              <div style={{ flex: 1, minWidth: 0 }}>
                <div style={{ fontSize: 14 }}>{s.reason}</div>
                <div style={{ fontSize: 12, color: "var(--text-3)" }}>
                  {timeAgo(s.created_at)}
                  {s.ends_at && ` · 至 ${new Date(s.ends_at).toLocaleString("zh-CN")}`}
                  {` · 状态：${s.status}`}
                </div>
              </div>
              {s.status === "active" && !hasPending && (
                <button className="btn btn-secondary btn-sm" onClick={() => setAppealTarget(s)}>
                  发起申诉
                </button>
              )}
              {hasPending && <span className="tag tag-warning">申诉处理中</span>}
            </div>
          );
        })}
      </div>

      <h2 style={{ fontSize: 15, fontWeight: 700, marginBottom: 10 }}>我的申诉记录</h2>
      {appeals.isLoading && <ListSkeleton count={1} />}
      {appeals.data && appeals.data.items.length === 0 && (
        <EmptyState title="还没有申诉记录" />
      )}
      <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
        {(appeals.data?.items ?? []).map((a) => (
          <div key={a.id} className="card" style={{ padding: 14 }}>
            <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
              <span className={`tag ${a.status === "accepted" ? "tag-success" : a.status === "rejected" ? "tag-danger" : "tag-warning"}`}>
                {a.status === "accepted" ? "已通过" : a.status === "rejected" ? "未通过" : "处理中"}
              </span>
              <span style={{ fontSize: 13, color: "var(--text-3)" }}>{a.target_label}</span>
              <span style={{ marginLeft: "auto", fontSize: 12, color: "var(--text-3)" }}>{timeAgo(a.created_at)}</span>
            </div>
            <div style={{ fontSize: 13.5, marginTop: 6 }}>{a.reason}</div>
            {a.status !== "pending" && a.handle_note && (
              <div style={{ fontSize: 13, color: "var(--text-2)", marginTop: 6, background: "var(--bg-inset)", padding: "8px 10px", borderRadius: 6 }}>
                处理意见：{a.handle_note}
              </div>
            )}
          </div>
        ))}
      </div>

      {appealTarget && (
        <div
          role="dialog"
          aria-modal="true"
          aria-label="发起申诉"
          style={{ position: "fixed", inset: 0, background: "rgba(0,0,0,.45)", display: "flex", alignItems: "center", justifyContent: "center", zIndex: 2000, padding: 16 }}
          onClick={(e) => e.target === e.currentTarget && setAppealTarget(null)}
        >
          <div className="card" style={{ width: 460, maxWidth: "100%", padding: 20 }}>
            <h3 style={{ fontWeight: 700, marginBottom: 8 }}>
              申诉：{sanctionLabels[appealTarget.sanction_type] ?? appealTarget.sanction_type}
            </h3>
            <p style={{ fontSize: 13, color: "var(--text-3)", marginBottom: 10 }}>处罚原因：{appealTarget.reason}</p>
            <textarea
              className="textarea"
              rows={5}
              maxLength={2000}
              placeholder="请说明申诉理由（5-2000 字）…"
              value={reason}
              onChange={(e) => setReason(e.target.value)}
            />
            <div style={{ display: "flex", gap: 8, justifyContent: "flex-end", marginTop: 12 }}>
              <button className="btn btn-secondary btn-sm" onClick={() => setAppealTarget(null)}>取消</button>
              <button className="btn btn-primary btn-sm" onClick={submitAppeal} disabled={submitting}>
                {submitting ? "提交中…" : "提交申诉"}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
