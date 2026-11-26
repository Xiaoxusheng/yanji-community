"use client";

// Markdown 编辑器：快捷键工具栏 + 图片粘贴/拖拽上传 + 编辑/分栏/预览模式。
import { useRef, useState } from "react";
import { marked } from "marked";
import DOMPurify from "dompurify";
import { api } from "@/lib/api";
import { toast } from "@/lib/toast";
import { IconColumns, IconEdit, IconEye, IconImage, IconLink } from "@/components/ui/Icons";

marked.setOptions({ breaks: true, gfm: true });

export function renderPreview(md: string): string {
  // SSR 阶段 DOMPurify 不可用，禁止返回未经净化的原始 HTML
  if (typeof window === "undefined") return "";
  const html = marked.parse(md || "", { async: false }) as string;
  return DOMPurify.sanitize(html, { USE_PROFILES: { html: true } });
}

type ViewMode = "edit" | "split" | "preview";

export function MarkdownEditor({
  value,
  onChange,
  placeholder,
  minHeight = 320,
  onSubmitShortcut,
}: {
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
  minHeight?: number;
  onSubmitShortcut?: () => void;
}) {
  const ref = useRef<HTMLTextAreaElement>(null);
  const [mode, setMode] = useState<ViewMode>("edit");
  const [uploading, setUploading] = useState(false);
  const [dragging, setDragging] = useState(false);

  const wrap = (before: string, after = before, fallback = "文本") => {
    const el = ref.current;
    if (!el) return;
    const { selectionStart: s, selectionEnd: e } = el;
    const selected = value.slice(s, e) || fallback;
    const next = value.slice(0, s) + before + selected + after + value.slice(e);
    onChange(next);
    requestAnimationFrame(() => {
      el.focus();
      el.setSelectionRange(s + before.length, s + before.length + selected.length);
    });
  };

  const uploadFile = async (file: File) => {
    if (!file.type.startsWith("image/")) {
      toast("error", "仅支持上传图片文件");
      return;
    }
    const fd = new FormData();
    fd.append("file", file);
    setUploading(true);
    try {
      const res = await api.upload<{ key: string; url: string }>("/uploads", fd);
      const el = ref.current;
      const at = el ? el.selectionStart : value.length;
      const snippet = `\n![${file.name || "image"}](${res.url})\n`;
      onChange(value.slice(0, at) + snippet + value.slice(at));
      toast("success", "图片已插入");
    } catch (err) {
      toast("error", (err as Error).message || "上传失败");
    } finally {
      setUploading(false);
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "b") {
      e.preventDefault();
      wrap("**");
      return;
    }
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "i") {
      e.preventDefault();
      wrap("*");
      return;
    }
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
      e.preventDefault();
      wrap("[", "](https://)", "链接文字");
      return;
    }
    if ((e.metaKey || e.ctrlKey) && e.key === "Enter" && onSubmitShortcut) {
      e.preventDefault();
      onSubmitShortcut();
    }
  };

  const tools: { label: React.ReactNode; title: string; act: () => void }[] = [
    { label: "B", title: "加粗 (Ctrl+B)", act: () => wrap("**") },
    { label: <i style={{ fontFamily: "Georgia, serif" }}>I</i>, title: "斜体 (Ctrl+I)", act: () => wrap("*") },
    { label: "H2", title: "二级标题", act: () => wrap("## ", "", "标题") },
    { label: "</>", title: "行内代码", act: () => wrap("`", "`", "code") },
    { label: "代码块", title: "插入代码块", act: () => wrap("\n```\n", "\n```\n", "代码") },
    { label: "引用", title: "插入引用", act: () => wrap("> ", "", "引用内容") },
    { label: "列表", title: "无序列表", act: () => wrap("- ", "", "列表项") },
    { label: <IconLink size={14} />, title: "插入链接 (Ctrl+K)", act: () => wrap("[", "](https://)", "链接文字") },
  ];

  return (
    <div
      className="card"
      style={{
        overflow: "hidden",
        borderColor: dragging ? "var(--brand)" : undefined,
      }}
    >
      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: 2,
          padding: "7px 10px",
          borderBottom: "1px solid var(--border)",
          background: "var(--bg-inset)",
          flexWrap: "wrap",
        }}
        role="toolbar"
        aria-label="编辑器工具栏"
      >
        {tools.map((t, idx) => (
          <button
            key={idx}
            type="button"
            title={t.title}
            aria-label={t.title}
            className="btn btn-text btn-xs"
            style={{ fontWeight: 600, minWidth: 28, height: 28 }}
            onClick={t.act}
          >
            {t.label}
          </button>
        ))}

        <label
          className="btn btn-text btn-xs"
          title="上传或粘贴/拖拽图片"
          aria-label="上传图片"
          style={{
            height: 28,
            cursor: uploading ? "wait" : "pointer",
            opacity: uploading ? 0.5 : 1,
          }}
        >
          <IconImage size={14} />
          <span>图片</span>
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

        <div className="seg-tabs" style={{ marginLeft: "auto", background: "var(--bg-card)" }}>
          <button
            type="button"
            className={`seg-tab ${mode === "edit" ? "is-active" : ""}`}
            style={{ padding: "4px 9px", fontSize: 12 }}
            onClick={() => setMode("edit")}
          >
            <IconEdit size={12} /> 编辑
          </button>
          <button
            type="button"
            className={`seg-tab hide-mobile ${mode === "split" ? "is-active" : ""}`}
            style={{ padding: "4px 9px", fontSize: 12 }}
            onClick={() => setMode("split")}
          >
            <IconColumns size={12} /> 分栏
          </button>
          <button
            type="button"
            className={`seg-tab ${mode === "preview" ? "is-active" : ""}`}
            style={{ padding: "4px 9px", fontSize: 12 }}
            onClick={() => setMode("preview")}
          >
            <IconEye size={12} /> 预览
          </button>
        </div>
      </div>

      <div
        style={{
          display: "grid",
          gridTemplateColumns: mode === "split" ? "1fr 1fr" : "1fr",
        }}
      >
        {mode !== "preview" && (
          <textarea
            ref={ref}
            className="textarea"
            value={value}
            onChange={(e) => onChange(e.target.value)}
            onKeyDown={handleKeyDown}
            onPaste={(e) => {
              const files = Array.from(e.clipboardData?.files ?? []);
              const img = files.find((f) => f.type.startsWith("image/"));
              if (img) {
                e.preventDefault();
                uploadFile(img);
              }
            }}
            onDragOver={(e) => {
              e.preventDefault();
              setDragging(true);
            }}
            onDragLeave={() => setDragging(false)}
            onDrop={(e) => {
              e.preventDefault();
              setDragging(false);
              const img = Array.from(e.dataTransfer?.files ?? []).find((f) => f.type.startsWith("image/"));
              if (img) uploadFile(img);
            }}
            placeholder={placeholder || "支持 Markdown 语法、直接粘贴或拖拽图片插入…"}
            style={{
              border: "none",
              borderRadius: 0,
              boxShadow: "none",
              minHeight,
              resize: "vertical",
              fontSize: 14.5,
              lineHeight: 1.72,
              padding: 16,
              borderRight: mode === "split" ? "1px solid var(--border)" : "none",
            }}
            aria-label="正文编辑区"
          />
        )}
        {mode !== "edit" && (
          <div
            className="prose-community"
            style={{ minHeight, padding: 16, background: "var(--bg-card)", overflowY: "auto" }}
            dangerouslySetInnerHTML={{
              __html: value.trim() ? renderPreview(value) : '<p style="color:var(--text-3)">暂无内容可预览</p>',
            }}
          />
        )}
      </div>

      <div
        style={{
          display: "flex",
          justifyContent: "space-between",
          padding: "6px 14px",
          borderTop: "1px solid var(--border)",
          fontSize: 12,
          color: "var(--text-3)",
          background: "var(--bg-inset)",
        }}
      >
        <span>{uploading ? "正在上传图片…" : "支持粘贴/拖拽图片"}</span>
        <span className="tabular-nums">{value.length} 字</span>
      </div>
    </div>
  );
}
