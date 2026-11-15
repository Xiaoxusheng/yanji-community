"use client";

// 投票组件：单选/多选/改票/结果百分比条。
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { api } from "@/lib/api";
import type { Poll } from "@/lib/types";
import { toast } from "@/lib/toast";
import { useMe } from "@/lib/auth";

export function PollBox({ postId }: { postId: string }) {
  const qc = useQueryClient();
  const { data: me } = useMe();
  const [selected, setSelected] = useState<number[]>([]);
  const [submitting, setSubmitting] = useState(false);

  const poll = useQuery({
    queryKey: ["poll", postId],
    queryFn: () => api.get<{ poll: Poll }>(`/posts/${postId}/poll`),
  });

  if (poll.isLoading) {
    return <div className="card" style={{ padding: 16, marginTop: 16 }}><div className="skeleton" style={{ height: 80 }} /></div>;
  }
  if (poll.error || !poll.data) return null; // 非投票帖静默
  const p = poll.data.poll;
  const voted = p.my_votes.length > 0;
  const canVote = !!me?.authenticated && !p.expired && (!voted || p.allow_revote);

  const toggle = (id: number) => {
    if (!me?.authenticated) return;
    if (!p.multiple_choice) {
      setSelected([id]);
      return;
    }
    setSelected((s) =>
      s.includes(id) ? s.filter((x) => x !== id) : s.length >= p.max_choices ? s : [...s, id],
    );
  };

  const submit = async () => {
    if (selected.length === 0) {
      toast("error", "请选择至少一个选项");
      return;
    }
    setSubmitting(true);
    try {
      await api.post(`/posts/${postId}/poll/vote`, { option_ids: selected });
      toast("success", "投票成功");
      setSelected([]);
      qc.invalidateQueries({ queryKey: ["poll", postId] });
    } catch (e) {
      toast("error", (e as Error).message);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <section className="card" style={{ padding: 18, marginTop: 18 }} aria-label="投票">
      <div style={{ display: "flex", gap: 8, alignItems: "center", marginBottom: 12, flexWrap: "wrap" }}>
        <h3 style={{ fontSize: 15, fontWeight: 700 }}>📊 投票</h3>
        <span className="tag">{p.multiple_choice ? `多选（最多 ${p.max_choices} 项）` : "单选"}</span>
        <span className="tag">{p.total_voters} 人参与</span>
        {p.expired && <span className="tag tag-warning">已截止</span>}
        {p.ends_at && !p.expired && (
          <span style={{ fontSize: 12.5, color: "var(--text-3)" }}>
            截止：{new Date(p.ends_at).toLocaleString("zh-CN")}
          </span>
        )}
      </div>
      <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
        {p.options.map((o) => {
          const isMine = p.my_votes.includes(o.id);
          const pct = Math.min(100, o.percent);
          return (
            <button
              key={o.id}
              onClick={() => toggle(o.id)}
              disabled={!canVote}
              aria-pressed={isMine}
              style={{
                position: "relative", textAlign: "left", border: isMine ? "1.5px solid var(--brand)" : "1px solid var(--border)",
                background: "var(--bg-card)", borderRadius: 8, padding: "10px 14px", cursor: canVote ? "pointer" : "default",
                overflow: "hidden",
              }}
            >
              {voted && (
                <span
                  aria-hidden
                  style={{
                    position: "absolute", inset: 0, width: `${pct}%`,
                    background: isMine ? "var(--brand-soft)" : "var(--bg-inset)",
                    transition: "width .3s ease",
                  }}
                />
              )}
              <span style={{ position: "relative", display: "flex", justifyContent: "space-between", fontSize: 14 }}>
                <span>{isMine && "✓ "}{o.content}</span>
                {voted && <span style={{ color: "var(--text-2)" }}>{o.percent}% · {o.votes} 票</span>}
              </span>
            </button>
          );
        })}
      </div>
      {canVote && (
        <button className="btn btn-primary btn-sm" style={{ marginTop: 12 }} onClick={submit} disabled={submitting || selected.length === 0}>
          {submitting ? "投票中…" : voted ? "修改投票" : "投票"}
        </button>
      )}
      {!me?.authenticated && (
        <p style={{ fontSize: 13, color: "var(--text-3)", marginTop: 10 }}>
          <a href="/login" style={{ color: "var(--brand)" }}>登录</a> 后参与投票
        </p>
      )}
    </section>
  );
}
