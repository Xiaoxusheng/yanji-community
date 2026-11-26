"use client";

// 举报对话框：帖子/评论/用户/私信。
import { useState } from "react";
import { api } from "@/lib/api";
import { toast } from "@/lib/toast";
import { ModalDialog } from "@/components/ui/Dialog";

const reasons = [
  { code: "spam", label: "垃圾广告 / 刷屏" },
  { code: "harassment", label: "人身攻击 / 骚扰" },
  { code: "porn_violence", label: "低俗 / 暴力内容" },
  { code: "illegal", label: "违法违规信息" },
  { code: "infringement", label: "侵权或抄袭" },
  { code: "misinformation", label: "误导 / 虚假信息" },
  { code: "flame", label: "恶意引战" },
  { code: "other", label: "其他" },
];

const targetNames: Record<string, string> = {
  post: "帖子",
  comment: "评论",
  user: "用户",
  message: "私信消息",
};

export function ReportDialog({
  open,
  onClose,
  targetType,
  targetId,
}: {
  open: boolean;
  onClose: () => void;
  targetType: "post" | "comment" | "user" | "message";
  targetId: string;
}) {
  const [reason, setReason] = useState("spam");
  const [desc, setDesc] = useState("");
  const [submitting, setSubmitting] = useState(false);

  const submit = async () => {
    setSubmitting(true);
    try {
      await api.post("/reports", { target_type: targetType, target_id: targetId, reason_code: reason, description: desc });
      toast("success", "举报已提交，管理团队将尽快处理");
      setDesc("");
      onClose();
    } catch (e) {
      toast("error", (e as Error).message);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <ModalDialog open={open} onClose={onClose} title={`举报该${targetNames[targetType] ?? "内容"}`} width={440}>
      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 8, marginBottom: 14 }}>
        {reasons.map((r) => (
          <label
            key={r.code}
            style={{
              display: "flex",
              gap: 8,
              alignItems: "center",
              fontSize: 13.5,
              padding: "8px 10px",
              borderRadius: 8,
              border: `1px solid ${reason === r.code ? "var(--brand)" : "var(--border)"}`,
              background: reason === r.code ? "var(--brand-soft)" : "transparent",
              cursor: "pointer",
            }}
          >
            <input type="radio" name="reason" checked={reason === r.code} onChange={() => setReason(r.code)} />
            <span>{r.label}</span>
          </label>
        ))}
      </div>
      <textarea
        className="textarea"
        placeholder="补充说明（可选，最多 1000 字）"
        value={desc}
        maxLength={1000}
        onChange={(e) => setDesc(e.target.value)}
        style={{ minHeight: 86, marginBottom: 14 }}
      />
      <div style={{ display: "flex", gap: 8, justifyContent: "flex-end" }}>
        <button type="button" className="btn btn-secondary btn-sm" onClick={onClose}>
          取消
        </button>
        <button type="button" className="btn btn-primary btn-sm" onClick={submit} disabled={submitting}>
          {submitting ? "提交中…" : "提交举报"}
        </button>
      </div>
    </ModalDialog>
  );
}
