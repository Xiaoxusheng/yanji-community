"use client";

// 通用对话框、确认弹窗、输入理由弹窗与图片大图灯箱（带微动效与毛玻璃层次）。
import { useEffect, useState } from "react";
import { IconX } from "@/components/ui/Icons";

export function ModalDialog({
  open,
  onClose,
  title,
  width = 460,
  children,
}: {
  open: boolean;
  onClose: () => void;
  title?: string;
  width?: number;
  children: React.ReactNode;
}) {
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, onClose]);

  if (!open) return null;
  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label={title || "对话框"}
      className="modal-backdrop"
      style={{
        position: "fixed",
        inset: 0,
        background: "rgba(9, 11, 16, 0.54)",
        backdropFilter: "blur(8px)",
        WebkitBackdropFilter: "blur(8px)",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        zIndex: 2100,
        padding: 16,
      }}
      onClick={(e) => e.target === e.currentTarget && onClose()}
    >
      <div
        className="card modal-panel"
        style={{
          width,
          maxWidth: "100%",
          maxHeight: "88vh",
          overflowY: "auto",
          padding: 22,
          borderRadius: "var(--radius-lg)",
          boxShadow: "var(--shadow-pop)",
        }}
      >
        {title && (
          <div
            style={{
              display: "flex",
              alignItems: "center",
              marginBottom: 16,
              paddingBottom: 12,
              borderBottom: "1px solid var(--border)",
            }}
          >
            <h3 style={{ fontSize: 16, fontWeight: 680, letterSpacing: "-0.015em", flex: 1 }}>{title}</h3>
            <button type="button" className="btn btn-icon" onClick={onClose} aria-label="关闭">
              <IconX size={16} />
            </button>
          </div>
        )}
        {children}
      </div>
    </div>
  );
}

export function ConfirmDialog({
  open,
  title,
  description,
  confirmText = "确认",
  danger,
  loading,
  onConfirm,
  onClose,
}: {
  open: boolean;
  title: string;
  description?: string;
  confirmText?: string;
  danger?: boolean;
  loading?: boolean;
  onConfirm: () => void;
  onClose: () => void;
}) {
  return (
    <ModalDialog open={open} onClose={onClose} title={title} width={410}>
      {description && <p style={{ fontSize: 14, color: "var(--text-2)", marginBottom: 20, lineHeight: 1.65 }}>{description}</p>}
      <div style={{ display: "flex", gap: 8, justifyContent: "flex-end" }}>
        <button type="button" className="btn btn-secondary btn-sm" onClick={onClose} disabled={loading}>
          取消
        </button>
        <button
          type="button"
          className={`btn btn-sm ${danger ? "btn-danger" : "btn-primary"}`}
          onClick={onConfirm}
          disabled={loading}
        >
          {loading ? "处理中…" : confirmText}
        </button>
      </div>
    </ModalDialog>
  );
}

export function PromptDialog({
  open,
  title,
  label,
  placeholder,
  confirmText = "确认",
  danger,
  loading,
  onConfirm,
  onClose,
}: {
  open: boolean;
  title: string;
  label?: string;
  placeholder?: string;
  confirmText?: string;
  danger?: boolean;
  loading?: boolean;
  onConfirm: (val: string) => void;
  onClose: () => void;
}) {
  const [val, setVal] = useState("");
  useEffect(() => {
    if (open) setVal("");
  }, [open]);

  return (
    <ModalDialog open={open} onClose={onClose} title={title} width={440}>
      {label && <label style={{ display: "block", fontSize: 13.5, color: "var(--text-2)", marginBottom: 8 }}>{label}</label>}
      <textarea
        className="textarea"
        rows={3}
        maxLength={500}
        placeholder={placeholder || "请填写原因…"}
        value={val}
        onChange={(e) => setVal(e.target.value)}
        autoFocus
      />
      <div style={{ display: "flex", gap: 8, justifyContent: "flex-end", marginTop: 14 }}>
        <button type="button" className="btn btn-secondary btn-sm" onClick={onClose} disabled={loading}>
          取消
        </button>
        <button
          type="button"
          className={`btn btn-sm ${danger ? "btn-danger" : "btn-primary"}`}
          disabled={loading || !val.trim()}
          onClick={() => onConfirm(val.trim())}
        >
          {loading ? "提交中…" : confirmText}
        </button>
      </div>
    </ModalDialog>
  );
}

export function ImageLightbox({ src, alt, onClose }: { src: string | null; alt?: string; onClose: () => void }) {
  useEffect(() => {
    if (!src) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [src, onClose]);

  if (!src) return null;
  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label="图片预览"
      className="modal-backdrop"
      style={{
        position: "fixed",
        inset: 0,
        background: "rgba(9, 11, 16, 0.88)",
        backdropFilter: "blur(10px)",
        WebkitBackdropFilter: "blur(10px)",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        zIndex: 2500,
        padding: 24,
        cursor: "zoom-out",
      }}
      onClick={onClose}
    >
      <button
        type="button"
        onClick={onClose}
        aria-label="关闭预览"
        style={{
          position: "fixed",
          top: 18,
          right: 18,
          width: 40,
          height: 40,
          borderRadius: 999,
          background: "rgba(255,255,255,0.14)",
          border: "1px solid rgba(255,255,255,0.18)",
          color: "#fff",
          cursor: "pointer",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
        }}
      >
        <IconX size={18} />
      </button>
      <img
        src={src}
        alt={alt || "图片大图"}
        className="modal-panel"
        style={{ maxWidth: "92vw", maxHeight: "90vh", borderRadius: 12, boxShadow: "0 20px 60px rgba(0,0,0,0.65)", cursor: "default" }}
        onClick={(e) => e.stopPropagation()}
      />
    </div>
  );
}
