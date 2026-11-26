"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/lib/api";
import { IconRefreshCw } from "@/components/ui/Icons";

export interface CaptchaData {
  captcha_id: string;
  image_data: string;
  expires_in: number;
}

/**
 * 图形验证码输入与刷新控件。
 * 每次 refreshKey 递增时自动拉取新挑战并清空旧输入，避免验证码失效后重复提交旧值。
 */
export function CaptchaField({
  captchaId,
  code,
  onCaptchaChange,
  onCodeChange,
  error,
  refreshKey = 0,
}: {
  captchaId: string;
  code: string;
  onCaptchaChange: (id: string) => void;
  onCodeChange: (v: string) => void;
  error?: string;
  refreshKey?: number;
}) {
  const [img, setImg] = useState<string>("");
  const [loading, setLoading] = useState(false);
  const [loadErr, setLoadErr] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setLoadErr(false);
    try {
      const res = await api.get<CaptchaData>("/auth/captcha");
      onCaptchaChange(res.captcha_id);
      setImg(res.image_data);
      onCodeChange("");
    } catch {
      setLoadErr(true);
    } finally {
      setLoading(false);
    }
  }, [onCaptchaChange, onCodeChange]);

  useEffect(() => {
    void load();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [refreshKey]);

  return (
    <div style={{ marginBottom: 14 }}>
      <label
        htmlFor="auth-captcha-input"
        style={{ display: "block", fontSize: 13.5, marginBottom: 6, fontWeight: 600 }}
      >
        图形验证码
      </label>
      <div style={{ display: "flex", gap: 8, alignItems: "stretch" }}>
        <input
          id="auth-captcha-input"
          className="input"
          type="text"
          value={code}
          maxLength={6}
          placeholder="输入右侧字符"
          autoComplete="off"
          spellCheck={false}
          aria-invalid={!!error}
          onChange={(e) => onCodeChange(e.target.value.toUpperCase())}
          style={{ flex: 1, letterSpacing: "0.14em", fontWeight: 600 }}
        />
        <button
          type="button"
          onClick={() => void load()}
          disabled={loading}
          title={captchaId ? "点击刷新图形验证码" : "加载验证码"}
          aria-label="刷新图形验证码"
          style={{
            width: 120,
            height: 40,
            padding: 0,
            borderRadius: "var(--radius-sm)",
            border: "1px solid var(--border)",
            background: "var(--bg-inset)",
            overflow: "hidden",
            cursor: loading ? "wait" : "pointer",
            position: "relative",
            flexShrink: 0,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            color: "var(--text-3)",
            fontSize: 12,
          }}
        >
          {img ? (
            <img
              src={img}
              alt="图形验证码"
              style={{ width: "100%", height: "100%", objectFit: "cover", display: "block", opacity: loading ? 0.45 : 1 }}
            />
          ) : (
            <span>{loadErr ? "点击重试" : "加载中…"}</span>
          )}
          <span
            aria-hidden
            style={{
              position: "absolute",
              right: 4,
              bottom: 4,
              width: 18,
              height: 18,
              borderRadius: 4,
              background: "rgba(255,255,255,0.85)",
              color: "#334155",
              display: "inline-flex",
              alignItems: "center",
              justifyContent: "center",
              boxShadow: "0 1px 2px rgba(0,0,0,0.08)",
            }}
          >
            <IconRefreshCw size={11} />
          </span>
        </button>
      </div>
      {error && <div style={{ color: "var(--danger)", fontSize: 12.5, marginTop: 4 }}>{error}</div>}
    </div>
  );
}
