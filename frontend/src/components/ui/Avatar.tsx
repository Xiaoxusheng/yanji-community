"use client";

import Link from "next/link";
import { fileURL } from "@/lib/api";

const sizes: Record<number, number> = { 24: 24, 32: 32, 40: 40, 56: 56 };

export function Avatar({
  name,
  avatarKey,
  size = 32,
  link,
}: {
  name: string;
  avatarKey?: string;
  size?: number;
  link?: string;
}) {
  const url = avatarKey ? fileURL(`/files/${avatarKey}`) : null;
  const letter = (name || "?").slice(0, 1).toUpperCase();
  const style: React.CSSProperties = {
    width: sizes[size] ?? size,
    height: sizes[size] ?? size,
    borderRadius: "50%",
    background: "var(--brand-soft)",
    color: "var(--brand)",
    display: "inline-flex",
    alignItems: "center",
    justifyContent: "center",
    fontWeight: 600,
    fontSize: size * 0.42,
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
