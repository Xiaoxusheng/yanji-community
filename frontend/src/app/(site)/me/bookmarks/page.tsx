"use client";

// 我的收藏：收藏夹管理（新建/重命名/删除）+ 收藏内容列表。
import Link from "next/link";
import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, timeAgo, type PageData } from "@/lib/api";
import type { BookmarkItem, FolderRow } from "@/lib/types";
import { ListSkeleton, ErrorState, EmptyState, PageHeader, Pagination } from "@/components/ui/States";
import { ConfirmDialog, PromptDialog } from "@/components/ui/Dialog";
import { AuthShell } from "@/components/auth/forms";
import { useMe } from "@/lib/auth";
import { toast } from "@/lib/toast";
import { IconEdit, IconPlus, IconTrash } from "@/components/ui/Icons";

export default function BookmarksPage() {
  const { data: me, isLoading: meLoading } = useMe();
  const qc = useQueryClient();
  const [folder, setFolder] = useState(-1);
  const [page, setPage] = useState(1);
  const [newName, setNewName] = useState("");
  const [confirmDeleteFolder, setConfirmDeleteFolder] = useState<FolderRow | null>(null);
  const [renameTarget, setRenameTarget] = useState<FolderRow | null>(null);

  const folders = useQuery({
    enabled: !!me?.authenticated,
    queryKey: ["bookmark-folders"],
    queryFn: () => api.get<{ items: FolderRow[] }>("/me/bookmark-folders"),
  });
  const items = useQuery({
    enabled: !!me?.authenticated,
    queryKey: ["bookmarks", folder, page],
    queryFn: () => api.get<PageData<BookmarkItem>>(`/me/bookmarks?folder_id=${folder}&page=${page}&page_size=20`),
  });

  if (meLoading) return null;
  if (me && !me.authenticated) {
    return (
      <AuthShell title="请先登录">
        <p style={{ marginBottom: 16, color: "var(--text-2)" }}>登录后查看和管理你的收藏。</p>
        <Link href="/login" className="btn btn-primary">去登录</Link>
      </AuthShell>
    );
  }

  const createFolder = async () => {
    if (!newName.trim()) return;
    try {
      await api.post("/me/bookmark-folders", { name: newName.trim() });
      setNewName("");
      toast("success", "收藏夹已创建");
      qc.invalidateQueries({ queryKey: ["bookmark-folders"] });
    } catch (e) {
      toast("error", (e as Error).message);
    }
  };

  const renameFolder = async (name: string) => {
    if (!renameTarget) return;
    try {
      await api.patch(`/me/bookmark-folders/${renameTarget.id}`, { name });
      toast("success", "已重命名");
      setRenameTarget(null);
      qc.invalidateQueries({ queryKey: ["bookmark-folders"] });
      qc.invalidateQueries({ queryKey: ["bookmarks"] });
    } catch (e) {
      toast("error", (e as Error).message);
    }
  };

  const deleteFolder = async () => {
    if (!confirmDeleteFolder) return;
    try {
      await api.delete(`/me/bookmark-folders/${confirmDeleteFolder.id}`);
      toast("success", "收藏夹已删除");
      if (folder === confirmDeleteFolder.id) setFolder(-1);
      setConfirmDeleteFolder(null);
      qc.invalidateQueries({ queryKey: ["bookmark-folders"] });
      qc.invalidateQueries({ queryKey: ["bookmarks"] });
    } catch (e) {
      toast("error", (e as Error).message);
    }
  };

  const removeBookmark = async (postId: string) => {
    try {
      await api.delete(`/posts/${postId}/bookmark`);
      toast("success", "已取消收藏");
      qc.invalidateQueries({ queryKey: ["bookmarks"] });
      qc.invalidateQueries({ queryKey: ["bookmark-folders"] });
    } catch (e) {
      toast("error", (e as Error).message);
    }
  };

  return (
    <div style={{ maxWidth: 940, margin: "0 auto" }}>
      <PageHeader title="我的收藏" subtitle="按收藏夹整理你保存的好内容" />
      <div className="layout-left-rail">
        <aside>
          <section className="card" style={{ padding: 14 }}>
            <h3 style={{ fontSize: 13.5, fontWeight: 650, marginBottom: 10 }}>收藏夹</h3>
            <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
              <button
                className={`btn btn-sm ${folder === -1 ? "btn-primary" : "btn-text"}`}
                style={{ justifyContent: "space-between" }}
                onClick={() => {
                  setFolder(-1);
                  setPage(1);
                }}
              >
                <span>全部收藏</span>
              </button>
              {(folders.data?.items ?? []).map((f) => (
                <div key={f.id} style={{ display: "flex", alignItems: "center", gap: 2 }}>
                  <button
                    className={`btn btn-sm ${folder === f.id ? "btn-primary" : "btn-text"}`}
                    style={{ flex: 1, justifyContent: "space-between", minWidth: 0 }}
                    onClick={() => {
                      setFolder(f.id);
                      setPage(1);
                    }}
                  >
                    <span style={{ overflow: "hidden", textOverflow: "ellipsis" }}>{f.name}</span>
                    <span className="tabular-nums" style={{ fontSize: 11.5, opacity: 0.7 }}>{f.count}</span>
                  </button>
                  {f.id > 0 && (
                    <>
                      <button
                        className="btn btn-icon"
                        style={{ width: 26, height: 26 }}
                        onClick={() => setRenameTarget(f)}
                        aria-label={`重命名 ${f.name}`}
                      >
                        <IconEdit size={12} />
                      </button>
                      <button
                        className="btn btn-icon"
                        style={{ width: 26, height: 26, color: "var(--danger)" }}
                        onClick={() => setConfirmDeleteFolder(f)}
                        aria-label={`删除 ${f.name}`}
                      >
                        <IconTrash size={12} />
                      </button>
                    </>
                  )}
                </div>
              ))}
            </div>

            <div style={{ display: "flex", gap: 6, marginTop: 12, paddingTop: 12, borderTop: "1px solid var(--border)" }}>
              <input
                className="input"
                style={{ padding: "6px 10px", fontSize: 13 }}
                placeholder="新建收藏夹…"
                value={newName}
                maxLength={50}
                onChange={(e) => setNewName(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && createFolder()}
              />
              <button className="btn btn-secondary btn-sm" onClick={createFolder} aria-label="添加收藏夹">
                <IconPlus size={14} />
              </button>
            </div>
          </section>
        </aside>

        <div>
          {items.isLoading && <ListSkeleton />}
          {items.error && <ErrorState message={(items.error as Error).message} onRetry={() => items.refetch()} />}
          {items.data && items.data.items.length === 0 && (
            <EmptyState title="暂无收藏内容" description="在阅读帖子时点击「收藏」即可归档到这里" />
          )}
          <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
            {(items.data?.items ?? []).map((b) => (
              <div key={b.post_id} className="card card-interactive" style={{ padding: 16, display: "flex", gap: 12, alignItems: "center" }}>
                <div style={{ flex: 1, minWidth: 0 }}>
                  <Link href={`/p/${b.post_id}`}>
                    <h2 style={{ fontSize: 15.5, fontWeight: 650 }}>{b.title}</h2>
                  </Link>
                  {b.excerpt && (
                    <p style={{ fontSize: 13, color: "var(--text-2)", margin: "4px 0 6px", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                      {b.excerpt}
                    </p>
                  )}
                  <div className="tabular-nums" style={{ fontSize: 12.5, color: "var(--text-3)", display: "flex", gap: 8, alignItems: "center", flexWrap: "wrap" }}>
                    <span>{b.author_name}</span>
                    <span>·</span>
                    <span>收藏于 {timeAgo(b.created_at)}</span>
                    <span className="tag tag-outline">{b.folder_name}</span>
                  </div>
                </div>
                <button className="btn btn-text btn-xs" style={{ color: "var(--danger)" }} onClick={() => removeBookmark(b.post_id)}>
                  取消收藏
                </button>
              </div>
            ))}
          </div>
          {items.data && items.data.total > 0 && (
            <Pagination page={page} pageSize={20} total={items.data.total} onChange={setPage} />
          )}
        </div>
      </div>

      <ConfirmDialog
        open={!!confirmDeleteFolder}
        title="删除收藏夹"
        description={`删除「${confirmDeleteFolder?.name ?? ""}」后，其中内容将保留在默认收藏夹中。`}
        confirmText="确认删除"
        danger
        onConfirm={deleteFolder}
        onClose={() => setConfirmDeleteFolder(null)}
      />
      <PromptDialog
        open={!!renameTarget}
        title={`重命名：${renameTarget?.name ?? ""}`}
        placeholder="输入新的收藏夹名称"
        confirmText="保存名称"
        onConfirm={renameFolder}
        onClose={() => setRenameTarget(null)}
      />
    </div>
  );
}
