"use client";

// 发帖 / 编辑帖子：类型选择卡、版块、标签联想胶囊、高级投票配置与 Markdown 分栏编辑器。
import { Suspense, useEffect, useMemo, useRef, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { Category, Tag, PostDetail } from "@/lib/types";
import { MarkdownEditor } from "@/components/editor/MarkdownEditor";
import { AuthShell } from "@/components/auth/forms";
import { PageHeader } from "@/components/ui/States";
import { useMe } from "@/lib/auth";
import { toast } from "@/lib/toast";
import { useDraftStore } from "@/stores/editor";
import { IconBarChart, IconFileText, IconHelpCircle, IconMessage, IconPlus, IconX } from "@/components/ui/Icons";

const postTypes = [
  { key: "discussion", label: "讨论帖", icon: IconMessage },
  { key: "article", label: "长文章", icon: IconFileText },
  { key: "question", label: "问答帖", icon: IconHelpCircle },
  { key: "poll", label: "投票帖", icon: IconBarChart },
] as const;

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
  const [pollMaxChoices, setPollMaxChoices] = useState(2);
  const [pollAllowRevote, setPollAllowRevote] = useState(false);
  const [pollEndsAt, setPollEndsAt] = useState("");
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
      setBody("");
      setCategory(p.category.slug);
      setPostType(p.post_type || "discussion");
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
      if (localDraft.postType) setPostType(localDraft.postType);
    } else if (presetCategory) {
      setCategory(presetCategory);
    }
    if (!editId) {
      initialized.current = true;
      setEditLoading(false);
    }
  }, [editId, post.data, draftKey, presetCategory, router]);

  // 编辑态拉取原始 Markdown
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
      .slice(0, 8);
  }, [tagInput, tags, allTags.data]);

  const addTag = (slug: string) => {
    if (!slug) return;
    if (tags.includes(slug)) {
      setTagInput("");
      return;
    }
    if (tags.length >= 5) {
      toast("error", "最多添加 5 个标签");
      return;
    }
    setTags([...tags, slug]);
    setTagInput("");
  };

  const buildPollPayload = () => {
    const cleanOpts = pollOptions.map((o) => o.trim()).filter(Boolean);
    return {
      options: cleanOpts,
      multiple_choice: pollMultiple,
      ...(pollMultiple ? { max_choices: Math.min(cleanOpts.length, Math.max(2, pollMaxChoices)) } : {}),
      allow_revote: pollAllowRevote,
      ...(pollEndsAt ? { ends_at: new Date(pollEndsAt).toISOString() } : {}),
    };
  };

  const saveLocalDraft = () => {
    useDraftStore.getState().save(draftKey, { title, body, tags, category, postType });
  };

  const saveDraftToServer = async () => {
    if (!title.trim() || !category) {
      toast("error", "保存草稿需要标题和版块");
      return;
    }
    const cleanOpts = pollOptions.map((o) => o.trim()).filter(Boolean);
    if (!editId && postType === "poll" && cleanOpts.length < 2) {
      toast("error", "投票草稿至少需要 2 个选项");
      return;
    }
    setSavingDraft(true);
    try {
      if (editId) {
        await api.patch(`/posts/${editId}`, { title, body_markdown: body, tag_slugs: tags });
        toast("success", "修改已保存");
      } else {
        await api.post("/posts", {
          title,
          body_markdown: body || "（草稿）",
          category_slug: category,
          post_type: postType,
          tag_slugs: tags,
          is_draft: true,
          ...(postType === "poll" ? { poll: buildPollPayload() } : {}),
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
    if (!editId && postType === "poll") {
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
        if (p && p.status === "draft") {
          await api.post(`/posts/${editId}/publish`);
        }
      } else {
        const res = await api.post<{ id: string; status: string }>("/posts", {
          title,
          body_markdown: body,
          category_slug: category,
          post_type: postType,
          tag_slugs: tags,
          ...(postType === "poll" ? { poll: buildPollPayload() } : {}),
        });
        id = res.id;
        if (res.status === "pending_review") {
          useDraftStore.getState().clear(draftKey);
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

  if (!me?.authenticated && !me) {
    return <div style={{ padding: 48, textAlign: "center", color: "var(--text-3)" }}>加载中…</div>;
  }
  if (me && !user) {
    return (
      <AuthShell title="请先登录">
        <p style={{ marginBottom: 16, color: "var(--text-2)" }}>登录后即可发表讨论、长文章、问答或投票。</p>
        <a href="/login" className="btn btn-primary">去登录</a>
      </AuthShell>
    );
  }
  if (user && !user.email_verified) {
    return (
      <AuthShell title="需要邮箱验证">
        <p style={{ marginBottom: 16, color: "var(--text-2)" }}>
          完成邮箱验证后才能发布帖子，请前往设置重发或查收验证邮件。
        </p>
        <a href="/settings" className="btn btn-primary">前往账号设置</a>
      </AuthShell>
    );
  }
  if (editLoading) {
    return <div style={{ padding: 48, textAlign: "center", color: "var(--text-3)" }}>正在加载帖子内容…</div>;
  }

  return (
    <div style={{ maxWidth: 920, margin: "0 auto" }}>
      <PageHeader
        title={editId ? "编辑内容" : "发布新内容"}
        subtitle={editId ? "修改内容后将保留版本修订记录" : "支持分栏实时预览、粘贴上传截图与 Ctrl+Enter 快捷发布"}
      />

      <div style={{ display: "flex", flexDirection: "column", gap: 14 }}>
        {/* 类型与版块栏 */}
        <div style={{ display: "flex", gap: 12, flexWrap: "wrap", alignItems: "center" }}>
          <div className="seg-tabs" role="radiogroup" aria-label="内容类型">
            {postTypes.map((pt) => {
              const Icon = pt.icon;
              const selected = postType === pt.key;
              return (
                <button
                  key={pt.key}
                  type="button"
                  disabled={!!editId}
                  aria-checked={selected}
                  role="radio"
                  className={`seg-tab ${selected ? "is-active" : ""}`}
                  onClick={() => setPostType(pt.key)}
                  style={{ opacity: editId && !selected ? 0.45 : 1 }}
                >
                  <Icon size={13} /> {pt.label}
                </button>
              );
            })}
          </div>

          <select
            className="select"
            value={category}
            onChange={(e) => setCategory(e.target.value)}
            aria-label="选择版块"
            style={{ maxWidth: 230, marginLeft: "auto" }}
          >
            <option value="">选择发布版块…</option>
            {(categories.data?.items ?? []).map((c) => (
              <option key={c.id} value={c.slug}>
                {c.name}
              </option>
            ))}
          </select>
        </div>

        {/* 标题 */}
        <input
          className="input"
          placeholder="输入清晰具体的标题（5-200 字）…"
          value={title}
          maxLength={200}
          onChange={(e) => setTitle(e.target.value)}
          onBlur={saveLocalDraft}
          aria-label="帖子标题"
          style={{ fontSize: 17, fontWeight: 650, padding: "12px 14px" }}
        />

        {/* 投票配置 */}
        {postType === "poll" && !editId && (
          <div className="card" style={{ padding: 16 }}>
            <div style={{ display: "flex", alignItems: "center", gap: 14, marginBottom: 12, flexWrap: "wrap" }}>
              <span style={{ fontSize: 14, fontWeight: 650, display: "inline-flex", alignItems: "center", gap: 6 }}>
                <IconBarChart size={15} /> 投票选项
              </span>
              <label style={{ fontSize: 13, display: "flex", gap: 6, alignItems: "center", cursor: "pointer" }}>
                <input type="checkbox" checked={pollMultiple} onChange={(e) => setPollMultiple(e.target.checked)} />
                允许多选
              </label>
              {pollMultiple && (
                <label style={{ fontSize: 13, display: "flex", gap: 6, alignItems: "center" }}>
                  最多选
                  <input
                    type="number"
                    className="input"
                    min={2}
                    max={10}
                    value={pollMaxChoices}
                    onChange={(e) => setPollMaxChoices(Math.max(2, Number(e.target.value) || 2))}
                    style={{ width: 64, padding: "4px 8px", fontSize: 13 }}
                  />
                  项
                </label>
              )}
              <label style={{ fontSize: 13, display: "flex", gap: 6, alignItems: "center", cursor: "pointer" }}>
                <input type="checkbox" checked={pollAllowRevote} onChange={(e) => setPollAllowRevote(e.target.checked)} />
                允许改票
              </label>
              <label style={{ fontSize: 13, display: "flex", gap: 6, alignItems: "center", marginLeft: "auto" }}>
                截止时间
                <input
                  type="datetime-local"
                  className="input"
                  value={pollEndsAt}
                  onChange={(e) => setPollEndsAt(e.target.value)}
                  style={{ width: 196, padding: "4px 8px", fontSize: 12.5 }}
                />
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
                      className="btn btn-icon"
                      aria-label={`删除选项 ${i + 1}`}
                      onClick={() => setPollOptions(pollOptions.filter((_, j) => j !== i))}
                    >
                      <IconX size={15} />
                    </button>
                  )}
                </div>
              ))}
            </div>
            {pollOptions.length < 10 && (
              <button
                type="button"
                className="btn btn-secondary btn-sm"
                style={{ marginTop: 10 }}
                onClick={() => setPollOptions([...pollOptions, ""])}
              >
                <IconPlus size={14} /> 添加选项
              </button>
            )}
          </div>
        )}

        {/* 标签选择与联想 */}
        <div className="card" style={{ padding: "10px 14px" }}>
          <div style={{ display: "flex", gap: 6, alignItems: "center", flexWrap: "wrap" }}>
            {tags.map((t) => (
              <span key={t} className="tag tag-brand">
                # {t}
                <button
                  type="button"
                  onClick={() => setTags(tags.filter((x) => x !== t))}
                  aria-label={`移除标签 ${t}`}
                  style={{ marginLeft: 3, background: "none", border: "none", cursor: "pointer", color: "inherit", display: "inline-flex" }}
                >
                  <IconX size={11} />
                </button>
              </span>
            ))}
            <input
              className="input"
              style={{ width: 170, padding: "4px 10px", fontSize: 13, border: "none", boxShadow: "none" }}
              placeholder={tags.length >= 5 ? "已达上限（5个）" : "输入标签按回车添加…"}
              disabled={tags.length >= 5}
              value={tagInput}
              onChange={(e) => setTagInput(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter" && tagInput.trim()) {
                  e.preventDefault();
                  addTag(tagInput.trim().toLowerCase().replace(/[^a-z0-9\u4e00-\u9fa5-]/g, "-"));
                }
              }}
              aria-label="添加标签"
            />
          </div>
          {tagSuggestions.length > 0 && tags.length < 5 && (
            <div style={{ display: "flex", gap: 6, flexWrap: "wrap", marginTop: 8, paddingTop: 8, borderTop: "1px solid var(--border)" }}>
              <span style={{ fontSize: 12, color: "var(--text-3)", alignSelf: "center" }}>推荐标签：</span>
              {tagSuggestions.map((t) => (
                <button key={t.id} type="button" className="tag" style={{ cursor: "pointer" }} onClick={() => addTag(t.slug)}>
                  + {t.name}
                </button>
              ))}
            </div>
          )}
        </div>

        {/* 正文编辑器 */}
        <MarkdownEditor value={body} onChange={setBody} minHeight={420} onSubmitShortcut={publish} />

        <div style={{ display: "flex", gap: 10, alignItems: "center", flexWrap: "wrap" }}>
          <button className="btn btn-primary" onClick={publish} disabled={submitting}>
            {submitting ? "发布中…" : editId ? "保存并更新" : "发布内容"}
          </button>
          {!editId && (
            <button className="btn btn-secondary" onClick={saveDraftToServer} disabled={savingDraft}>
              {savingDraft ? "保存中…" : "保存草稿"}
            </button>
          )}
          <span style={{ fontSize: 12.5, color: "var(--text-3)", marginLeft: "auto" }}>
            离开标题时自动本地暂存草稿
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
