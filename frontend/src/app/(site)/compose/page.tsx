"use client";

// 发帖 / 编辑帖子：标题、版块、标签、Markdown 编辑器、草稿保存、发布。
import { Suspense, useEffect, useMemo, useRef, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { Category, Tag, PostDetail } from "@/lib/types";
import { MarkdownEditor } from "@/components/editor/MarkdownEditor";
import { AuthShell } from "@/components/auth/forms";
import { useMe } from "@/lib/auth";
import { toast } from "@/lib/toast";
import { useDraftStore } from "@/stores/editor";

function ComposeInner() {
  const router = useRouter();
  const params = useSearchParams();
  const qc = useQueryClient();
  const { data: me } = useMe();
  const user = me?.authenticated ? me.user : null;

  const editId = params.get("post");
  const presetCategory = params.get("category") ?? "";
  const draftKey = editId ? `edit:${editId}` : "new";

  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  const [category, setCategory] = useState(presetCategory);
  const [tagInput, setTagInput] = useState("");
  const [tags, setTags] = useState<string[]>([]);
  const [postType, setPostType] = useState("discussion");
  const [pollOptions, setPollOptions] = useState<string[]>(["", ""]);
  const [pollMultiple, setPollMultiple] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [savingDraft, setSavingDraft] = useState(false);
  const [editLoading, setEditLoading] = useState(!!editId);
  const initialized = useRef(false);

  const categories = useQuery({
    queryKey: ["categories"],
    queryFn: () => api.get<{ items: Category[] }>("/categories"),
  });
  const allTags = useQuery({
    queryKey: ["tags", "all"],
    queryFn: () => api.get<{ items: Tag[] }>("/tags?page_size=100"),
  });
  const post = useQuery({
    queryKey: ["post", editId],
    enabled: !!editId,
    queryFn: () => api.get<{ post: PostDetail }>(`/posts/${editId}`),
  });

  // 载入编辑内容或恢复本地草稿
  useEffect(() => {
    if (initialized.current) return;
    const localDraft = useDraftStore.getState().drafts[draftKey];
    if (editId && post.data) {
      const p = post.data.post;
      if (!p.is_author) {
        toast("error", "只能编辑自己的帖子");
        router.replace(`/p/${editId}`);
        return;
      }
      setTitle(p.title);
      setBody(""); // 正文 Markdown 需要单独获取：详情回传的是净化 HTML
      setCategory(p.category.slug);
      setTags(p.tags.map((t) => t.slug));
      initialized.current = true;
      setEditLoading(false);
      return;
    }
    if (!editId && localDraft) {
      setTitle(localDraft.title);
      setBody(localDraft.body);
      setTags(localDraft.tags);
      setCategory(localDraft.category || presetCategory);
    } else if (presetCategory) {
      setCategory(presetCategory);
    }
    if (!editId) {
      initialized.current = true;
      setEditLoading(false);
    }
  }, [editId, post.data, draftKey, presetCategory, router]);

  // 编辑态：拉取原始 Markdown
  useEffect(() => {
    if (!editId || !post.data) return;
    api
      .get<{ items: { body_markdown: string }[] }>(`/posts/${editId}/revisions`)
      .then((revs) => {
        if (revs.items.length > 0) setBody(revs.items[0].body_markdown);
      })
      .catch(() => {});
  }, [editId, post.data]);

  const tagSuggestions = useMemo(() => {
    const q = tagInput.trim().toLowerCase();
    return (allTags.data?.items ?? [])
      .filter((t) => !tags.includes(t.slug) && (!q || t.slug.includes(q) || t.name.includes(q)))
      .slice(0, 6);
  }, [tagInput, tags, allTags.data]);

  const addTag = (slug: string) => {
    if (tags.length >= 5) {
      toast("error", "最多 5 个标签");
      return;
    }
    setTags([...tags, slug]);
    setTagInput("");
  };

  const saveLocalDraft = () => {
    useDraftStore.getState().save(draftKey, { title, body, tags, category });
  };

  const saveDraftToServer = async () => {
    if (!title.trim() || !category) {
      toast("error", "保存草稿需要标题和版块");
      return;
    }
    setSavingDraft(true);
    try {
      if (editId) {
        await api.patch(`/posts/${editId}`, { title, body_markdown: body, tag_slugs: tags });
        toast("success", "修改已保存");
      } else {
        await api.post("/posts", {
          title, body_markdown: body || "（草稿）", category_slug: category,
          post_type: "discussion", tag_slugs: tags, is_draft: true,
        });
        toast("success", "草稿已保存");
      }
      useDraftStore.getState().clear(draftKey);
      qc.invalidateQueries({ queryKey: ["me-posts"] });
      router.push("/me/posts");
    } catch (e) {
      toast("error", (e as Error).message);
    } finally {
      setSavingDraft(false);
    }
  };

  const publish = async () => {
    const next: string[] = [];
    if (title.trim().length < 5) next.push("标题至少 5 个字");
    if (!category) next.push("请选择版块");
    if (!body.trim()) next.push("正文不能为空");
    if (postType === "poll") {
        const opts = pollOptions.map((o) => o.trim()).filter(Boolean);
        if (opts.length < 2) next.push("投票至少需要 2 个选项");
      }
      if (next.length) {
        toast("error", next.join("；"));
        return;
      }
    setSubmitting(true);
    try {
      let id = editId ?? "";
      if (editId) {
        await api.patch(`/posts/${editId}`, { title, body_markdown: body, tag_slugs: tags, edit_reason: "编辑更新" });
        const p = post.data?.post;
        if (p && (p.status === "draft")) {
          await api.post(`/posts/${editId}/publish`);
        }
      } else {
        const res = await api.post<{ id: string; status: string }>("/posts", {
          title, body_markdown: body, category_slug: category,
          post_type: postType, tag_slugs: tags,
          ...(postType === "poll"
            ? {
                poll: {
                  options: pollOptions.map((o) => o.trim()).filter(Boolean),
                  multiple_choice: pollMultiple,
                },
              }
            : {}),
        });
        id = res.id;
        if (res.status === "pending_review") {
          toast("info", "内容已提交，等待审核通过后公开显示");
          router.push("/me/posts");
          return;
        }
      }
      useDraftStore.getState().clear(draftKey);
      toast("success", "发布成功");
      router.push(`/p/${id}`);
    } catch (e) {
      toast("error", (e as Error).message);
    } finally {
      setSubmitting(false);
    }
  };

  // 登录/验证守卫
  if (!me?.authenticated && !me) {
    return <div style={{ padding: 40, textAlign: "center", color: "var(--text-3)" }}>加载中…</div>;
  }
  if (me && !user) {
    return (
      <AuthShell title="请先登录">
        <p style={{ marginBottom: 16, color: "var(--text-2)" }}>登录后即可发布内容。</p>
        <a href="/login" className="btn btn-primary">去登录</a>
      </AuthShell>
    );
  }
  if (user && !user.email_verified) {
    return (
      <AuthShell title="需要邮箱验证">
        <p style={{ marginBottom: 16, color: "var(--text-2)" }}>
          为维护社区质量，完成邮箱验证后才能发帖。请查收验证邮件。
        </p>
        <a href="/settings" className="btn btn-primary">去设置重发验证邮件</a>
      </AuthShell>
    );
  }
  if (editLoading) {
    return <div style={{ padding: 40, textAlign: "center", color: "var(--text-3)" }}>加载帖子中…</div>;
  }

  return (
    <div style={{ maxWidth: 860, margin: "0 auto" }}>
      <h1 style={{ fontSize: 22, fontWeight: 700, marginBottom: 16 }}>
        {editId ? "编辑帖子" : "发布新帖"}
      </h1>
      <div style={{ display: "flex", flexDirection: "column", gap: 14 }}>
        <input
          className="input"
          placeholder="标题（5-200 字，清晰具体更容易获得回复）"
          value={title}
          maxLength={200}
          onChange={(e) => setTitle(e.target.value)}
          onBlur={saveLocalDraft}
          aria-label="帖子标题"
          style={{ fontSize: 16, fontWeight: 600, padding: "12px 14px" }}
        />
        <div style={{ display: "flex", gap: 12, flexWrap: "wrap" }}>
          <select
            className="select"
            value={category}
            onChange={(e) => setCategory(e.target.value)}
            aria-label="选择版块"
            style={{ maxWidth: 240 }}
          >
            <option value="">选择版块…</option>
            {(categories.data?.items ?? []).map((c) => (
              <option key={c.id} value={c.slug}>
                {c.name}
              </option>
            ))}
          </select>
          <select
            className="select"
            value={postType}
            onChange={(e) => setPostType(e.target.value)}
            aria-label="帖子类型"
            style={{ maxWidth: 180 }}
          >
            <option value="discussion">💬 讨论帖</option>
            <option value="article">📄 长文章</option>
            <option value="question">❓ 问答帖</option>
            <option value="poll">📊 投票帖</option>
          </select>
        </div>
        {postType === "poll" && (
          <div className="card" style={{ padding: 16 }}>
            <div style={{ display: "flex", alignItems: "center", gap: 10, marginBottom: 10 }}>
              <b style={{ fontSize: 14 }}>📊 投票选项</b>
              <label style={{ fontSize: 13, display: "flex", gap: 6, alignItems: "center", marginLeft: "auto" }}>
                <input type="checkbox" checked={pollMultiple} onChange={(e) => setPollMultiple(e.target.checked)} />
                允许多选
              </label>
            </div>
            <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
              {pollOptions.map((opt, i) => (
                <div key={i} style={{ display: "flex", gap: 8 }}>
                  <input
                    className="input"
                    placeholder={`选项 ${i + 1}`}
                    value={opt}
                    maxLength={200}
                    onChange={(e) => setPollOptions(pollOptions.map((x, j) => (j === i ? e.target.value : x)))}
                    aria-label={`投票选项 ${i + 1}`}
                  />
                  {pollOptions.length > 2 && (
                    <button
                      type="button"
                      className="btn btn-text"
                      aria-label={`删除选项 ${i + 1}`}
                      onClick={() => setPollOptions(pollOptions.filter((_, j) => j !== i))}
                    >
                      ×
                    </button>
                  )}
                </div>
              ))}
            </div>
            {pollOptions.length < 10 && (
              <button type="button" className="btn btn-secondary btn-sm" style={{ marginTop: 10 }} onClick={() => setPollOptions([...pollOptions, ""])}>
                + 添加选项
              </button>
            )}
          </div>
        )}
        <div>
          <div style={{ display: "flex", gap: 8, alignItems: "center", flexWrap: "wrap", marginBottom: 8 }}>
            {tags.map((t) => (
              <span key={t} className="tag tag-brand">
                # {t}
                <button
                  onClick={() => setTags(tags.filter((x) => x !== t))}
                  aria-label={`移除标签 ${t}`}
                  style={{ marginLeft: 4, background: "none", border: "none", cursor: "pointer", color: "inherit", fontWeight: 700 }}
                >
                  ×
                </button>
              </span>
            ))}
            <input
              className="input"
              style={{ maxWidth: 180, padding: "5px 10px" }}
              placeholder="添加标签（回车）"
              value={tagInput}
              list="tag-suggestions"
              onChange={(e) => setTagInput(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter" && tagInput.trim()) {
                  e.preventDefault();
                  addTag(tagInput.trim().toLowerCase().replace(/[^a-z0-9\u4e00-\u9fa5-]/g, "-"));
                }
              }}
              aria-label="添加标签"
            />
            <datalist id="tag-suggestions">
              {tagSuggestions.map((t) => (
                <option key={t.id} value={t.slug}>{t.name}</option>
              ))}
            </datalist>
          </div>
        </div>
        <MarkdownEditor value={body} onChange={setBody} minHeight={420} />
        <div style={{ display: "flex", gap: 10, alignItems: "center" }}>
          <button className="btn btn-primary" onClick={publish} disabled={submitting}>
            {submitting ? "发布中…" : editId ? "保存并发布" : "发布"}
          </button>
          {!editId && (
            <button className="btn btn-secondary" onClick={saveDraftToServer} disabled={savingDraft}>
              {savingDraft ? "保存中…" : "存为草稿"}
            </button>
          )}
          <span style={{ fontSize: 12.5, color: "var(--text-3)" }}>
            草稿会自动暂存在本地浏览器
          </span>
          <span style={{ marginLeft: "auto", fontSize: 12.5, color: "var(--text-3)" }}>
            {body.length} 字符
          </span>
        </div>
      </div>
    </div>
  );
}

export default function ComposePage() {
  return (
    <Suspense>
      <ComposeInner />
    </Suspense>
  );
}
