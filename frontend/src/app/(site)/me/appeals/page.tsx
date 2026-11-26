"use client";

// 申诉中心：我的处罚与申诉进度。
import Link from "next/link";
import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, timeAgo } from "@/lib/api";
import type { Sanction, Appeal } from "@/lib/types";
import { ListSkeleton, EmptyState, PageHeader } from "@/components/ui/States";
import { ModalDialog } from "@/components/ui/Dialog";
import { AuthShell } from "@/components/auth/forms";
import { useMe } from "@/lib/auth";
import { toast } from "@/lib/toast";

const sanctionLabels: Record<string, string> = {
  warn: "警告",
  mute: "禁言",
  post_ban: "禁止发帖",
  temp_ban: "临时封禁",
  perma_ban: "永久封禁",
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
        <p style={{ marginBottom: 16, color: "var(--text-2)" }}>登录后查看处罚与申诉记录。</p>
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
    <div style={{ maxWidth: 780, margin: "0 auto" }}>
      <PageHeader
        title="申诉中心"
        subtitle="对处罚决定有异议时可提交申诉，管理团队将独立复核"
      />

      <h2 style={{ fontSize: 15, fontWeight: 650, marginBottom: 10 }}>处罚记录</h2>
      {sanctions.isLoading && <ListSkeleton count={1} />}
      {sanctions.data && sanctions.data.items.length === 0 && (
        <div style={{ marginBottom: 24 }}>
          <EmptyState title="暂无处罚记录" description="当前账号状态正常" />
        </div>
      )}
      <div style={{ display: "flex", flexDirection: "column", gap: 8, marginBottom: 24 }}>
        {(sanctions.data?.items ?? []).map((s) => {
          const hasPending = appealedSanctionIds.has(s.id);
          return (
            <div key={s.id} className="card" style={{ padding: 14, display: "flex", gap: 10, alignItems: "center" }}>
              <span className={`tag ${s.status === "active" ? "tag-danger" : ""}`}>{sanctionLabels[s.sanction_type] ?? s.sanction_type}</span>
              <div style={{ flex: 1, minWidth: 0 }}>
                <div style={{ fontSize: 14 }}>{s.reason}</div>
                <div className="tabular-nums" style={{ fontSize: 12, color: "var(--text-3)", marginTop: 2 }}>
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

      <h2 style={{ fontSize: 15, fontWeight: 650, marginBottom: 10 }}>申诉记录</h2>
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
              <span className="tabular-nums" style={{ marginLeft: "auto", fontSize: 12, color: "var(--text-3)" }}>{timeAgo(a.created_at)}</span>
            </div>
            <div style={{ fontSize: 13.5, marginTop: 6 }}>{a.reason}</div>
            {a.status !== "pending" && a.handle_note && (
              <div style={{ fontSize: 13, color: "var(--text-2)", marginTop: 8, background: "var(--bg-inset)", padding: "8px 12px", borderRadius: 6 }}>
                处理意见：{a.handle_note}
              </div>
            )}
          </div>
        ))}
      </div>

      <ModalDialog
        open={!!appealTarget}
        onClose={() => setAppealTarget(null)}
        title={`发起申诉：${appealTarget ? sanctionLabels[appealTarget.sanction_type] ?? appealTarget.sanction_type : ""}`}
        width={460}
      >
        <p style={{ fontSize: 13, color: "var(--text-3)", marginBottom: 10 }}>
          处罚原因：{appealTarget?.reason}
        </p>
        <textarea
          className="textarea"
          rows={5}
          maxLength={2000}
          placeholder="请详细说明申诉理由（5-2000 字）…"
          value={reason}
          onChange={(e) => setReason(e.target.value)}
        />
        <div style={{ display: "flex", gap: 8, justifyContent: "flex-end", marginTop: 14 }}>
          <button className="btn btn-secondary btn-sm" onClick={() => setAppealTarget(null)}>取消</button>
          <button className="btn btn-primary btn-sm" onClick={submitAppeal} disabled={submitting}>
            {submitting ? "提交中…" : "提交申诉"}
          </button>
        </div>
      </ModalDialog>
    </div>
  );
}
