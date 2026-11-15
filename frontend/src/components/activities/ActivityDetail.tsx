"use client";

// 活动详情页组件。
import Link from "next/link";
import { useParams } from "next/navigation";
import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, timeAgo } from "@/lib/api";
import type { Activity } from "@/lib/types";
import { ListSkeleton, ErrorState } from "@/components/ui/States";
import { toast } from "@/lib/toast";

export function fmtRange(a: Activity): string {
  const s = new Date(a.starts_at).toLocaleString("zh-CN", { dateStyle: "medium", timeStyle: "short" });
  const e = a.ends_at ? new Date(a.ends_at).toLocaleString("zh-CN", { dateStyle: "medium", timeStyle: "short" }) : "";
  return e ? `${s} ~ ${e}` : s;
}

export function ActivityDetailPage() {
  const params = useParams<{ id: string }>();
  const qc = useQueryClient();
  const [busy, setBusy] = useState(false);

  const detail = useQuery({
    queryKey: ["activity", params.id],
    queryFn: () => api.get<{ activity: Activity }>(`/activities/${params.id}`),
  });

  if (detail.isLoading) return <ListSkeleton count={1} />;
  if (detail.error || !detail.data)
    return <ErrorState message={(detail.error as Error)?.message} onRetry={() => detail.refetch()} />;
  const a = detail.data.activity;
  const full = a.capacity > 0 && a.participant_count >= a.capacity;

  const join = async () => {
    setBusy(true);
    try {
      const res = await api.post<{ message: string }>(`/activities/${a.id}/join`);
      toast("success", res.message);
      qc.invalidateQueries({ queryKey: ["activity", params.id] });
      qc.invalidateQueries({ queryKey: ["activities"] });
    } catch (e) {
      toast("error", (e as Error).message);
    } finally {
      setBusy(false);
    }
  };
  const cancel = async () => {
    if (!window.confirm("确定取消报名吗？")) return;
    setBusy(true);
    try {
      await api.delete(`/activities/${a.id}/join`);
      toast("success", "已取消报名");
      qc.invalidateQueries({ queryKey: ["activity", params.id] });
    } catch (e) {
      toast("error", (e as Error).message);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div style={{ maxWidth: 720, margin: "0 auto" }}>
      <Link href="/activities" style={{ fontSize: 13, color: "var(--brand)" }}>← 返回活动列表</Link>
      <article className="card" style={{ padding: 24, marginTop: 10 }}>
        <h1 style={{ fontSize: 22, fontWeight: 700, marginBottom: 8 }}>{a.title}</h1>
        <div style={{ display: "flex", gap: 8, flexWrap: "wrap", marginBottom: 16 }}>
          <span className="tag tag-info">{fmtRange(a)}</span>
          {a.location && <span className="tag">📍 {a.location}</span>}
          <span className="tag">{a.participant_count}/{a.capacity || "∞"} 人已报名</span>
          {a.reward_points > 0 && <span className="tag tag-success">出席奖励 +{a.reward_points} 积分</span>}
          <span style={{ fontSize: 12.5, color: "var(--text-3)", alignSelf: "center" }}>
            发起人：{a.created_by_name} · {timeAgo(a.created_at)}
          </span>
        </div>
        <div className="prose-community" style={{ whiteSpace: "pre-wrap" }}>{a.description || "暂无详细介绍"}</div>
        <div style={{ display: "flex", gap: 10, marginTop: 20, paddingTop: 16, borderTop: "1px solid var(--border)", flexWrap: "wrap" }}>
          {a.is_joined ? (
            <button className="btn btn-secondary" onClick={cancel} disabled={busy}>
              取消报名
            </button>
          ) : full ? (
            <button className="btn btn-primary" disabled>
              名额已满
            </button>
          ) : a.status !== "published" ? (
            <span className="tag tag-warning">该活动当前不可报名</span>
          ) : (
            <button className="btn btn-primary" onClick={join} disabled={busy}>
              {busy ? "提交中…" : "立即报名"}
            </button>
          )}
          <span style={{ fontSize: 12.5, color: "var(--text-3)", alignSelf: "center" }}>
            {a.signup_ends_at && `报名截止：${new Date(a.signup_ends_at).toLocaleString("zh-CN")}`}
          </span>
        </div>
      </article>
    </div>
  );
}
