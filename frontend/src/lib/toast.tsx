"use client";

// 极简 Toast：操作成功/失败反馈（前台不引入重型组件库）。
import { create } from "zustand";
import { useEffect } from "react";

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

const colors: Record<ToastKind, string> = {
  success: "var(--success)",
  error: "var(--danger)",
  info: "var(--info)",
};

export function ToastHost() {
  const items = useToast((s) => s.items);
  const remove = useToast((s) => s.remove);
  useEffect(() => {}, [items]);
  if (items.length === 0) return null;
  return (
    <div
      style={{
        position: "fixed",
        top: 16,
        left: "50%",
        transform: "translateX(-50%)",
        zIndex: 9999,
        display: "flex",
        flexDirection: "column",
        gap: 8,
        alignItems: "center",
      }}
      role="status"
      aria-live="polite"
    >
      {items.map((t) => (
        <div
          key={t.id}
          onClick={() => remove(t.id)}
          className="card"
          style={{
            padding: "10px 18px",
            display: "flex",
            gap: 8,
            alignItems: "center",
            fontSize: 14,
            boxShadow: "var(--shadow-pop)",
            cursor: "pointer",
            borderLeft: `3px solid ${colors[t.kind]}`,
            maxWidth: 420,
          }}
        >
          <span style={{ color: colors[t.kind] }}>●</span>
          {t.text}
        </div>
      ))}
    </div>
  );
}
