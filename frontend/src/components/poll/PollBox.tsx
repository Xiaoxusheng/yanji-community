"use client";

// 投票组件：单选/多选视觉指示器、Framer Motion 弹簧百分比进度条、领先选项徽章与改票支持。
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { motion } from "framer-motion";
import { api } from "@/lib/api";
import type { Poll } from "@/lib/types";
import { toast } from "@/lib/toast";
import { useMe } from "@/lib/auth";
import { IconBarChart, IconCheck } from "@/components/ui/Icons";

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
    return (
      <div className="card" style={{ padding: 18, marginTop: 18 }}>
        <div className="skeleton" style={{ height: 96 }} />
      </div>
    );
  }
  if (poll.error || !poll.data) return null;
  const p = poll.data.poll;
  const voted = p.my_votes.length > 0;
  const canVote = !!me?.authenticated && !p.expired && (!voted || p.allow_revote);
  const showResults = voted || p.expired;
  const maxVotes = Math.max(0, ...p.options.map((o) => o.votes));

  const toggle = (id: number) => {
    if (!me?.authenticated || !canVote) return;
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
      toast("success", voted ? "改票成功" : "投票成功");
      setSelected([]);
      qc.invalidateQueries({ queryKey: ["poll", postId] });
    } catch (e) {
      toast("error", (e as Error).message);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <section
      className="surface-spotlight"
      style={{ padding: 20, marginTop: 18 }}
      aria-label="社区投票"
    >
      <div style={{ display: "flex", gap: 8, alignItems: "center", marginBottom: 14, flexWrap: "wrap" }}>
        <h3 style={{ fontSize: 15, fontWeight: 680, display: "inline-flex", alignItems: "center", gap: 6, margin: 0 }}>
          <IconBarChart size={16} style={{ color: "var(--brand)" }} /> 社区投票
        </h3>
        <span className="tag tag-brand">{p.multiple_choice ? `多选 · 最多 ${p.max_choices} 项` : "单选"}</span>
        <span className="tag tag-outline tabular-nums">{p.total_voters} 人参与</span>
        {p.allow_revote && !p.expired && <span className="tag tag-outline">允许改票</span>}
        {p.expired && <span className="tag tag-warning">已截止</span>}
        {p.ends_at && !p.expired && (
          <span className="tabular-nums" style={{ fontSize: 12, color: "var(--text-3)", marginLeft: "auto" }}>
            截止 {new Date(p.ends_at).toLocaleString("zh-CN")}
          </span>
        )}
      </div>

      <div style={{ display: "flex", flexDirection: "column", gap: 9 }}>
        {p.options.map((o) => {
          const isMine = p.my_votes.includes(o.id);
          const isSelected = selected.includes(o.id);
          const pct = Math.min(100, o.percent);
          const active = isSelected || isMine;
          const isLeader = showResults && maxVotes > 0 && o.votes === maxVotes;
          return (
            <button
              key={o.id}
              type="button"
              onClick={() => toggle(o.id)}
              disabled={!canVote}
              aria-pressed={active}
              style={{
                position: "relative",
                textAlign: "left",
                border: `1px solid ${active ? "var(--brand)" : "var(--border-strong)"}`,
                background: "var(--bg-card)",
                borderRadius: 10,
                padding: "11px 14px",
                cursor: canVote ? "pointer" : "default",
                overflow: "hidden",
                transition: "border-color 0.16s ease, box-shadow 0.16s ease",
                boxShadow: active ? "0 0 0 2px var(--brand-soft)" : undefined,
              }}
            >
              {showResults && (
                <motion.span
                  aria-hidden
                  initial={{ width: 0 }}
                  animate={{ width: `${pct}%` }}
                  transition={{ type: "spring", stiffness: 220, damping: 28 }}
                  style={{
                    position: "absolute",
                    inset: 0,
                    background: isMine ? "var(--brand-soft)" : "var(--bg-hover)",
                  }}
                />
              )}
              <span
                style={{
                  position: "relative",
                  display: "flex",
                  justifyContent: "space-between",
                  alignItems: "center",
                  gap: 10,
                  fontSize: 13.75,
                }}
              >
                <span style={{ display: "inline-flex", alignItems: "center", gap: 9, fontWeight: active ? 620 : 480, minWidth: 0 }}>
                  <span
                    aria-hidden
                    style={{
                      width: 17,
                      height: 17,
                      borderRadius: p.multiple_choice ? 5 : 999,
                      border: `1.5px solid ${active ? "var(--brand)" : "var(--border-strong)"}`,
                      background: active ? "var(--brand)" : "var(--bg-card)",
                      color: "#fff",
                      display: "inline-flex",
                      alignItems: "center",
                      justifyContent: "center",
                      flexShrink: 0,
                      transition: "all 0.15s ease",
                    }}
                  >
                    {active && <IconCheck size={11} strokeWidth={2.5} />}
                  </span>
                  <span style={{ overflowWrap: "anywhere" }}>{o.content}</span>
                  {isLeader && (
                    <span className="tag tag-brand" style={{ fontSize: 10.5, padding: "0 6px", lineHeight: "17px" }}>
                      领先
                    </span>
                  )}
                </span>
                {showResults && (
                  <span className="tabular-nums" style={{ color: isMine ? "var(--brand)" : "var(--text-2)", fontSize: 12.5, fontWeight: isMine ? 640 : 500, flexShrink: 0 }}>
                    {o.percent}% · {o.votes} 票
                  </span>
                )}
              </span>
            </button>
          );
        })}
      </div>

      {canVote && (
        <div style={{ display: "flex", alignItems: "center", gap: 10, marginTop: 14 }}>
          <button
            className="btn btn-brand btn-sm"
            onClick={submit}
            disabled={submitting || selected.length === 0}
          >
            {submitting ? "提交中…" : voted ? "确认改票" : "提交投票"}
          </button>
          {selected.length > 0 && (
            <span style={{ fontSize: 12.5, color: "var(--text-3)" }}>已选 {selected.length} 项</span>
          )}
        </div>
      )}
      {!me?.authenticated && (
        <p style={{ fontSize: 13, color: "var(--text-3)", marginTop: 12, marginBottom: 0 }}>
          <a href="/login" style={{ color: "var(--brand)", fontWeight: 560 }}>登录</a> 后即可参与投票
        </p>
      )}
    </section>
  );
}
