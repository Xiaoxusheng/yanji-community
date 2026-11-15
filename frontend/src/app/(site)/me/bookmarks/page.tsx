"use client";

// 我的收藏：收藏夹列表 + 收藏内容。
import Link from "next/link";
import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, timeAgo, type PageData } from "@/lib/api";
import type { BookmarkItem, FolderRow } from "@/lib/types";
import { ListSkeleton, ErrorState, EmptyState, Pagination } from "@/components/ui/States";
import { AuthShell } from "@/components/auth/forms";
import { useMe } from "@/lib/auth";
import { toast } from "@/lib/toast";

export default function BookmarksPage() {
  const { data: me, isLoading: meLoading } = useMe();
  const qc = useQueryClient();
  const [folder, setFolder] = useState(-1); // -1 全部 / 0 默认夹 / >0 自定义
  const [page, setPage] = useState(1);
  const [newName, setNewName] = useState("");

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
        <p style={{ marginBottom: 16 }}>登录后查看你的收藏。</p>
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
  const deleteFolder = async (id: number) => {
    if (!window.confirm("删除收藏夹后，其中收藏将移入默认收藏夹，确定？")) return;
    try {
      await api.delete(`/me/bookmark-folders/${id}`);
      toast("success", "收藏夹已删除");
      if (folder === id) setFolder(-1);
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
    } catch (e) {
      toast("error", (e as Error).message);
    }
  };

  return (
    <div style={{ display: "grid", gridTemplateColumns: "240px minmax(0,1fr)", gap: 20, alignItems: "start", maxWidth: 900, margin: "0 auto" }}>
      <aside style={{ display: "flex", flexDirection: "column", gap: 12 }}>
        <section className="card" style={{ padding: 14 }}>
          <h3 style={{ fontSize: 14, fontWeight: 700, marginBottom: 10 }}>收藏夹</h3>
          <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
            <button
              className={`btn btn-text ${folder === -1 ? "btn-primary" : ""}`}
              style={{ justifyContent: "space-between" }}
              onClick={() => { setFolder(-1); setPage(1); }}
            >
              全部收藏
            </button>
            {(folders.data?.items ?? []).map((f) => (
              <div key={f.id} style={{ display: "flex", alignItems: "center" }}>
                <button
                  className={`btn btn-text ${folder === f.id ? "btn-primary" : ""}`}
                  style={{ flex: 1, justifyContent: "space-between" }}
                  onClick={() => { setFolder(f.id); setPage(1); }}
                >
                  {f.name}
                  <span style={{ color: "var(--text-3)", fontSize: 12 }}>{f.count}</span>
                </button>
                {f.id > 0 && (
                  <button className="btn btn-text" style={{ color: "var(--danger)", padding: 4 }} onClick={() => deleteFolder(f.id)} aria-label={`删除收藏夹 ${f.name}`}>
                    ×
                  </button>
                )}
              </div>
            ))}
          </div>
          <div style={{ display: "flex", gap: 6, marginTop: 10 }}>
            <input
              className="input"
              style={{ padding: "6px 10px", fontSize: 13 }}
              placeholder="新收藏夹名"
              value={newName}
              maxLength={50}
              onChange={(e) => setNewName(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && createFolder()}
            />
            <button className="btn btn-secondary btn-sm" onClick={createFolder}>
              添加
            </button>
          </div>
        </section>
      </aside>

      <div>
        {items.isLoading && <ListSkeleton />}
        {items.error && <ErrorState message={(items.error as Error).message} onRetry={() => items.refetch()} />}
        {items.data && items.data.items.length === 0 && (
          <EmptyState title="还没有收藏" description="浏览帖子时点击「收藏」即可保存到这里" />
        )}
        <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
          {(items.data?.items ?? []).map((b) => (
            <div key={b.post_id} className="card" style={{ padding: 16, display: "flex", gap: 10 }}>
              <div style={{ flex: 1 }}>
                <Link href={`/p/${b.post_id}`}>
                  <h2 style={{ fontSize: 15.5, fontWeight: 600 }}>{b.title}</h2>
                </Link>
                <div style={{ fontSize: 13, color: "var(--text-3)", marginTop: 4 }}>
                  {b.author_name} · 收藏于 {timeAgo(b.created_at)} · {b.folder_name}
                </div>
              </div>
              <button className="btn btn-text btn-sm" style={{ color: "var(--danger)" }} onClick={() => removeBookmark(b.post_id)}>
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
  );
}
