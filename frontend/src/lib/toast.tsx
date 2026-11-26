"use client";

// 轻量 Toast 反馈组件（SVG 图标 + 胶囊毛玻璃浮层 + 微动效）。
import { create } from "zustand";
import { IconAlert, IconCheckCircle } from "@/components/ui/Icons";

type ToastKind = "success" | "error" | "info";
interface ToastItem {
  id: number;
  kind: ToastKind;
  text: string;
}

interface ToastState {
  items: ToastItem[];
  push: (kind: ToastKind, text: string) => void;
  remove: (id: number) => void;
}

let nextId = 1;

export const useToast = create<ToastState>((set) => ({
  items: [],
  push: (kind, text) => {
    const id = nextId++;
    set((s) => ({ items: [...s.items, { id, kind, text }] }));
    setTimeout(() => set((s) => ({ items: s.items.filter((t) => t.id !== id) })), 3200);
  },
  remove: (id) => set((s) => ({ items: s.items.filter((t) => t.id !== id) })),
}));

export function toast(kind: ToastKind, text: string) {
  useToast.getState().push(kind, text);
}

const tones: Record<ToastKind, { fg: string; bg: string }> = {
  success: { fg: "var(--success)", bg: "var(--success-soft)" },
  error: { fg: "var(--danger)", bg: "var(--danger-soft)" },
  info: { fg: "var(--brand)", bg: "var(--brand-soft)" },
};

export function ToastHost() {
  const items = useToast((s) => s.items);
  const remove = useToast((s) => s.remove);
  if (items.length === 0) return null;
  return (
    <div
      style={{
        position: "fixed",
        top: 18,
        left: "50%",
        transform: "translateX(-50%)",
        zIndex: 9999,
        display: "flex",
        flexDirection: "column",
        gap: 8,
        alignItems: "center",
        pointerEvents: "none",
      }}
      role="status"
      aria-live="polite"
    >
      {items.map((t) => (
        <div
          key={t.id}
          onClick={() => remove(t.id)}
          className="card surface-glass toast-item"
          style={{
            padding: "9px 15px 9px 11px",
            borderRadius: 999,
            display: "flex",
            gap: 9,
            alignItems: "center",
            fontSize: 13.5,
            fontWeight: 550,
            boxShadow: "var(--shadow-pop)",
            cursor: "pointer",
            maxWidth: 440,
            pointerEvents: "auto",
          }}
        >
          <span
            style={{
              color: tones[t.kind].fg,
              background: tones[t.kind].bg,
              width: 24,
              height: 24,
              borderRadius: 999,
              display: "inline-flex",
              alignItems: "center",
              justifyContent: "center",
              flexShrink: 0,
            }}
          >
            {t.kind === "success" ? <IconCheckCircle size={15} /> : <IconAlert size={15} />}
          </span>
          <span>{t.text}</span>
        </div>
      ))}
    </div>
  );
}
