"use client";

import Link from "next/link";
import { fileURL } from "@/lib/api";

const sizes: Record<number, number> = { 24: 24, 32: 32, 40: 40, 46: 46, 56: 56, 72: 72 };

// 确定性低饱和高级色盘，让无自定义头像的成员同样具备清晰辨识度
const avatarPalettes: [string, string][] = [
  ["linear-gradient(135deg, #e0e7ff 0%, #c7d2fe 100%)", "#3730a3"],
  ["linear-gradient(135deg, #dbeafe 0%, #bfdbfe 100%)", "#1d4ed8"],
  ["linear-gradient(135deg, #d1fae5 0%, #a7f3d0 100%)", "#065f46"],
  ["linear-gradient(135deg, #fef3c7 0%, #fde68a 100%)", "#92400e"],
  ["linear-gradient(135deg, #fce7f3 0%, #fbcfe8 100%)", "#9d174d"],
  ["linear-gradient(135deg, #e2e8f0 0%, #cbd5e1 100%)", "#1e293b"],
  ["linear-gradient(135deg, #ede9fe 0%, #ddd6fe 100%)", "#5b21b6"],
  ["linear-gradient(135deg, #ccfbf1 0%, #99f6e4 100%)", "#115e59"],
];

function paletteFor(name: string): [string, string] {
  let h = 0;
  for (let i = 0; i < name.length; i++) {
    h = (h * 31 + name.charCodeAt(i)) >>> 0;
  }
  return avatarPalettes[h % avatarPalettes.length];
}

export function Avatar({
  name,
  avatarKey,
  size = 32,
  link,
  ring,
}: {
  name: string;
  avatarKey?: string;
  size?: number;
  link?: string;
  ring?: boolean;
}) {
  const url = avatarKey ? fileURL(avatarKey.startsWith("/files/") || avatarKey.startsWith("http") ? avatarKey : `/files/${avatarKey}`) : null;
  const letter = (name || "?").slice(0, 1).toUpperCase();
  const px = sizes[size] ?? size;
  const [bg, fg] = paletteFor(name || "?");

  const style: React.CSSProperties = {
    width: px,
    height: px,
    borderRadius: "50%",
    background: url ? "var(--bg-inset)" : bg,
    border: ring ? "3px solid var(--bg-card)" : "1px solid var(--border)",
    boxShadow: ring ? "0 4px 14px rgba(15, 23, 42, 0.14)" : "inset 0 1px 0 rgba(255,255,255,0.45)",
    color: fg,
    display: "inline-flex",
    alignItems: "center",
    justifyContent: "center",
    fontWeight: 700,
    fontSize: Math.max(11, Math.round(px * 0.41)),
    letterSpacing: "-0.02em",
    flexShrink: 0,
    overflow: "hidden",
  };
  const inner = url ? (
    <img src={url} alt={name} style={{ width: "100%", height: "100%", objectFit: "cover" }} />
  ) : (
    letter
  );
  if (link) {
    return (
      <Link href={link} style={style} aria-label={name}>
        {inner}
      </Link>
    );
  }
  return (
    <span style={style} aria-label={name}>
      {inner}
    </span>
  );
}
