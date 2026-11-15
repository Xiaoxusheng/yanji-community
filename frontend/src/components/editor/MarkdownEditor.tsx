"use client";

// Markdown 编辑器：textarea + 工具栏 + 净化预览。
// 预览仅为本地方便；服务端 goldmark+bluemonday 渲染是唯一信任边界。
import { useRef, useState } from "react";
import { marked } from "marked";
import DOMPurify from "dompurify";
import { api } from "@/lib/api";
import { toast } from "@/lib/toast";

marked.setOptions({ breaks: true, gfm: true });

export function renderPreview(md: string): string {
  const html = marked.parse(md || "", { async: false }) as string;
  if (typeof window === "undefined") return html;
  return DOMPurify.sanitize(html, { USE_PROFILES: { html: true } });
}

export function MarkdownEditor({
  value,
  onChange,
  placeholder,
  minHeight = 320,
}: {
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
  minHeight?: number;
}) {
  const ref = useRef<HTMLTextAreaElement>(null);
  const [preview, setPreview] = useState(false);
  const [uploading, setUploading] = useState(false);

  const wrap = (before: string, after = before) => {
    const el = ref.current;
    if (!el) return;
    const { selectionStart: s, selectionEnd: e } = el;
    const selected = value.slice(s, e) || "文本";
    const next = value.slice(0, s) + before + selected + after + value.slice(e);
    onChange(next);
    requestAnimationFrame(() => {
      el.focus();
      el.setSelectionRange(s + before.length, s + before.length + selected.length);
    });
  };

  const uploadFile = async (file: File) => {
    const fd = new FormData();
    fd.append("file", file);
    setUploading(true);
    try {
      const res = await api.upload<{ key: string; url: string }>("/uploads", fd);
      const el = ref.current;
      const at = el ? el.selectionStart : value.length;
      onChange(value.slice(0, at) + `\n![${file.name}](${res.url})\n` + value.slice(at));
      toast("success", "图片已上传");
    } catch (err) {
      toast("error", (err as Error).message || "上传失败");
    } finally {
      setUploading(false);
    }
  };

  const tools: { label: string; title: string; act: () => void }[] = [
    { label: "B", title: "加粗", act: () => wrap("**") },
    { label: "I", title: "斜体", act: () => wrap("*") },
    { label: "H2", title: "标题", act: () => wrap("## ", "") },
    { label: "``", title: "行内代码", act: () => wrap("`") },
    { label: "```", title: "代码块", act: () => wrap("\n```\n", "\n```\n") },
    { label: ">", title: "引用", act: () => wrap("> ", "") },
    { label: "•", title: "列表", act: () => wrap("- ", "") },
    { label: "🔗", title: "链接", act: () => wrap("[", "](https://)") },
  ];

  return (
    <div className="card" style={{ overflow: "hidden" }}>
      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: 2,
          padding: "6px 10px",
          borderBottom: "1px solid var(--border)",
          flexWrap: "wrap",
        }}
        role="toolbar"
        aria-label="编辑器工具栏"
      >
        {tools.map((t) => (
          <button
            key={t.label}
            type="button"
            title={t.title}
            aria-label={t.title}
            className="btn btn-text"
            style={{ padding: "4px 8px", fontWeight: 600, minWidth: 30 }}
            onClick={t.act}
          >
            {t.label}
          </button>
        ))}
        <label
          className="btn btn-text"
          title="上传图片"
          aria-label="上传图片"
          style={{
            padding: "4px 8px",
            cursor: uploading ? "wait" : "pointer",
            opacity: uploading ? 0.5 : 1,
          }}
        >
          🖼
          <input
            type="file"
            accept="image/jpeg,image/png,image/gif,image/webp"
            hidden
            onChange={(e) => {
              const f = e.target.files?.[0];
              if (f) uploadFile(f);
              e.target.value = "";
            }}
          />
        </label>
        <div style={{ marginLeft: "auto" }}>
          <button
            type="button"
            className={`btn btn-text btn-sm ${preview ? "btn-secondary" : ""}`}
            onClick={() => setPreview((v) => !v)}
            aria-pressed={preview}
          >
            {preview ? "编辑" : "预览"}
          </button>
        </div>
      </div>
      {preview ? (
        <div
          className="prose-community"
          style={{ minHeight, padding: 16, background: "var(--bg-card)" }}
          // 后端同样会净化；此处为本地即时预览
          dangerouslySetInnerHTML={{ __html: renderPreview(value) }}
        />
      ) : (
        <textarea
          ref={ref}
          className="textarea"
          value={value}
          onChange={(e) => onChange(e.target.value)}
          placeholder={placeholder || "使用 Markdown 撰写内容…（支持代码块、引用、表格、图片）"}
          style={{ border: "none", minHeight, resize: "vertical", fontSize: 15, lineHeight: 1.7 }}
          aria-label="正文编辑区"
        />
      )}
      {uploading && <div style={{ padding: 8, color: "var(--text-3)", fontSize: 13 }}>图片上传中…</div>}
    </div>
  );
}
