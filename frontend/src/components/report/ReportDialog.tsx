"use client";

// 举报对话框：帖子/评论/用户。
import { useState } from "react";
import { api } from "@/lib/api";
import { toast } from "@/lib/toast";

const reasons = [
  { code: "spam", label: "垃圾广告" },
  { code: "harassment", label: "人身攻击 / 骚扰" },
  { code: "porn_violence", label: "色情 / 暴力" },
  { code: "illegal", label: "违法违规" },
  { code: "infringement", label: "侵权" },
  { code: "misinformation", label: "虚假信息" },
  { code: "flame", label: "恶意引战" },
  { code: "other", label: "其他" },
];

export function ReportDialog({
  open,
  onClose,
  targetType,
  targetId,
}: {
  open: boolean;
  onClose: () => void;
  targetType: "post" | "comment" | "user";
  targetId: string;
}) {
  const [reason, setReason] = useState("spam");
  const [desc, setDesc] = useState("");
  const [submitting, setSubmitting] = useState(false);

  if (!open) return null;
  const submit = async () => {
    setSubmitting(true);
    try {
      await api.post("/reports", { target_type: targetType, target_id: targetId, reason_code: reason, description: desc });
      toast("success", "举报已提交，我们会尽快处理");
      setDesc("");
      onClose();
    } catch (e) {
      toast("error", (e as Error).message);
    } finally {
      setSubmitting(false);
    }
  };
  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label="举报"
      style={{
        position: "fixed",
        inset: 0,
        background: "rgba(0,0,0,0.45)",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        zIndex: 2000,
        padding: 16,
      }}
      onClick={(e) => e.target === e.currentTarget && onClose()}
    >
      <div className="card" style={{ width: 420, maxWidth: "100%", padding: 20 }}>
        <h3 style={{ fontWeight: 700, marginBottom: 12 }}>举报该{targetType === "post" ? "帖子" : targetType === "comment" ? "评论" : "用户"}</h3>
        <div style={{ display: "flex", flexDirection: "column", gap: 8, marginBottom: 12 }}>
          {reasons.map((r) => (
            <label key={r.code} style={{ display: "flex", gap: 8, alignItems: "center", fontSize: 14, cursor: "pointer" }}>
              <input type="radio" name="reason" checked={reason === r.code} onChange={() => setReason(r.code)} />
              {r.label}
            </label>
          ))}
        </div>
        <textarea
          className="textarea"
          placeholder="补充说明（可选，最多 1000 字）"
          value={desc}
          maxLength={1000}
          onChange={(e) => setDesc(e.target.value)}
          style={{ minHeight: 80, marginBottom: 12 }}
        />
        <div style={{ display: "flex", gap: 8, justifyContent: "flex-end" }}>
          <button className="btn btn-secondary btn-sm" onClick={onClose}>
            取消
          </button>
          <button className="btn btn-primary btn-sm" onClick={submit} disabled={submitting}>
            {submitting ? "提交中…" : "提交举报"}
          </button>
        </div>
      </div>
    </div>
  );
}
