"use client";

// 基于 Radix UI Dialog + Framer Motion 的无障碍对话框、确认弹窗、理由输入弹窗与图片大图灯箱。
// 具备自动 Portal 挂载、焦点锁定、滚动锁定与丝滑弹簧入退场动效。
import { useEffect, useState } from "react";
import * as RadixDialog from "@radix-ui/react-dialog";
import { AnimatePresence, motion } from "framer-motion";
import { IconAlert, IconX } from "@/components/ui/Icons";

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
  return (
    <RadixDialog.Root open={open} onOpenChange={(next) => !next && onClose()}>
      <AnimatePresence>
        {open && (
          <RadixDialog.Portal forceMount>
            <RadixDialog.Overlay asChild>
              <motion.div
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                exit={{ opacity: 0 }}
                transition={{ duration: 0.16, ease: "easeOut" }}
                style={{
                  position: "fixed",
                  inset: 0,
                  background: "rgba(9, 11, 16, 0.56)",
                  backdropFilter: "blur(8px)",
                  WebkitBackdropFilter: "blur(8px)",
                  zIndex: 2100,
                }}
              />
            </RadixDialog.Overlay>
            <RadixDialog.Content asChild aria-describedby={undefined}>
              <div
                style={{
                  position: "fixed",
                  inset: 0,
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  zIndex: 2101,
                  padding: 16,
                  pointerEvents: "none",
                }}
              >
                <motion.div
                  initial={{ opacity: 0, y: 10, scale: 0.96 }}
                  animate={{ opacity: 1, y: 0, scale: 1 }}
                  exit={{ opacity: 0, y: 6, scale: 0.97 }}
                  transition={{ type: "spring", stiffness: 420, damping: 32, mass: 0.8 }}
                  className="card"
                  style={{
                    width,
                    maxWidth: "100%",
                    maxHeight: "88vh",
                    overflowY: "auto",
                    padding: 22,
                    borderRadius: "var(--radius-lg)",
                    boxShadow: "var(--shadow-pop)",
                    pointerEvents: "auto",
                  }}
                >
                  {title ? (
                    <div
                      style={{
                        display: "flex",
                        alignItems: "center",
                        gap: 10,
                        marginBottom: 16,
                        paddingBottom: 12,
                        borderBottom: "1px solid var(--border)",
                      }}
                    >
                      <RadixDialog.Title
                        style={{
                          fontSize: 16,
                          fontWeight: 680,
                          letterSpacing: "-0.015em",
                          flex: 1,
                          margin: 0,
                        }}
                      >
                        {title}
                      </RadixDialog.Title>
                      <RadixDialog.Close asChild>
                        <button type="button" className="btn btn-icon" aria-label="关闭对话框" style={{ width: 30, height: 30 }}>
                          <IconX size={16} />
                        </button>
                      </RadixDialog.Close>
                    </div>
                  ) : (
                    <RadixDialog.Title className="sr-only">对话框</RadixDialog.Title>
                  )}
                  {children}
                </motion.div>
              </div>
            </RadixDialog.Content>
          </RadixDialog.Portal>
        )}
      </AnimatePresence>
    </RadixDialog.Root>
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
    <ModalDialog open={open} onClose={onClose} title={title} width={420}>
      <div style={{ display: "flex", gap: 12, alignItems: "flex-start", marginBottom: 20 }}>
        {danger && (
          <span
            style={{
              width: 36,
              height: 36,
              borderRadius: 10,
              background: "var(--danger-soft)",
              color: "var(--danger)",
              display: "inline-flex",
              alignItems: "center",
              justifyContent: "center",
              flexShrink: 0,
            }}
            aria-hidden
          >
            <IconAlert size={18} />
          </span>
        )}
        {description && (
          <p style={{ fontSize: 14, color: "var(--text-2)", margin: 0, lineHeight: 1.65 }}>{description}</p>
        )}
      </div>
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
      {label && (
        <label style={{ display: "block", fontSize: 13.5, color: "var(--text-2)", marginBottom: 8 }}>{label}</label>
      )}
      <textarea
        className="textarea"
        rows={3}
        maxLength={500}
        placeholder={placeholder || "请填写原因…"}
        value={val}
        onChange={(e) => setVal(e.target.value)}
        autoFocus
      />
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginTop: 14 }}>
        <span className="tabular-nums" style={{ fontSize: 12, color: "var(--text-3)" }}>
          {val.length} / 500
        </span>
        <div style={{ display: "flex", gap: 8 }}>
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
      </div>
    </ModalDialog>
  );
}

export function ImageLightbox({ src, alt, onClose }: { src: string | null; alt?: string; onClose: () => void }) {
  return (
    <RadixDialog.Root open={!!src} onOpenChange={(next) => !next && onClose()}>
      <AnimatePresence>
        {src && (
          <RadixDialog.Portal forceMount>
            <RadixDialog.Overlay asChild>
              <motion.div
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                exit={{ opacity: 0 }}
                transition={{ duration: 0.18 }}
                onClick={onClose}
                style={{
                  position: "fixed",
                  inset: 0,
                  background: "rgba(9, 11, 16, 0.88)",
                  backdropFilter: "blur(12px)",
                  WebkitBackdropFilter: "blur(12px)",
                  zIndex: 2500,
                }}
              />
            </RadixDialog.Overlay>
            <RadixDialog.Content asChild aria-describedby={undefined}>
              <div
                onClick={onClose}
                style={{
                  position: "fixed",
                  inset: 0,
                  display: "flex",
                  flexDirection: "column",
                  alignItems: "center",
                  justifyContent: "center",
                  zIndex: 2501,
                  padding: 24,
                  cursor: "zoom-out",
                }}
              >
                <RadixDialog.Title className="sr-only">{alt || "图片预览"}</RadixDialog.Title>
                <RadixDialog.Close asChild>
                  <button
                    type="button"
                    aria-label="关闭预览"
                    style={{
                      position: "fixed",
                      top: 18,
                      right: 18,
                      width: 40,
                      height: 40,
                      borderRadius: 999,
                      background: "rgba(255,255,255,0.14)",
                      border: "1px solid rgba(255,255,255,0.2)",
                      color: "#fff",
                      cursor: "pointer",
                      display: "flex",
                      alignItems: "center",
                      justifyContent: "center",
                    }}
                  >
                    <IconX size={18} />
                  </button>
                </RadixDialog.Close>
                <motion.img
                  src={src}
                  alt={alt || "图片大图"}
                  initial={{ opacity: 0, scale: 0.94 }}
                  animate={{ opacity: 1, scale: 1 }}
                  exit={{ opacity: 0, scale: 0.96 }}
                  transition={{ type: "spring", stiffness: 360, damping: 28 }}
                  style={{
                    maxWidth: "92vw",
                    maxHeight: "86vh",
                    borderRadius: 12,
                    boxShadow: "0 24px 64px rgba(0,0,0,0.68)",
                    cursor: "default",
                  }}
                  onClick={(e) => e.stopPropagation()}
                />
                <div
                  style={{
                    marginTop: 12,
                    fontSize: 12,
                    color: "rgba(255,255,255,0.62)",
                    userSelect: "none",
                  }}
                >
                  按 Esc 或点击空白区域关闭
                </div>
              </div>
            </RadixDialog.Content>
          </RadixDialog.Portal>
        )}
      </AnimatePresence>
    </RadixDialog.Root>
  );
}
