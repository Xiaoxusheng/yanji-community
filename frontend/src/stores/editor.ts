"use client";

// 编辑器草稿本地暂存（Zustand persist，仅存 UI 草稿文本，不含认证凭证）。
import { create } from "zustand";
import { persist } from "zustand/middleware";

export interface DraftContent {
  title: string;
  body: string;
  tags: string[];
  category: string;
  postType?: string;
  savedAt: number;
}

interface DraftState {
  drafts: Record<string, DraftContent>;
  save: (key: string, d: { title: string; body: string; tags: string[]; category: string; postType?: string }) => void;
  clear: (key: string) => void;
}

export const useDraftStore = create<DraftState>()(
  persist(
    (set) => ({
      drafts: {},
      save: (key, d) =>
        set((s) => ({ drafts: { ...s.drafts, [key]: { ...d, savedAt: Date.now() } } })),
      clear: (key) =>
        set((s) => {
          const next = { ...s.drafts };
          delete next[key];
          return { drafts: next };
        }),
    }),
    { name: "yanji-editor-drafts" },
  ),
);
