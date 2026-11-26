"use client";

// 基于 Sonner 的现代堆叠式 Toast 通知框架（保持原 toast(kind, text) 接口 100% 兼容，并实时同步深浅色主题）。
import { useEffect, useState } from "react";
import { Toaster, toast as sonnerToast } from "sonner";

type ToastKind = "success" | "error" | "info";

export function toast(kind: ToastKind, text: string) {
  if (kind === "success") {
    sonnerToast.success(text);
    return;
  }
  if (kind === "error") {
    sonnerToast.error(text);
    return;
  }
  sonnerToast.info(text);
}

export function ToastHost() {
  const [theme, setTheme] = useState<"light" | "dark" | "system">("system");

  useEffect(() => {
    const sync = () => {
      const attr = document.documentElement.getAttribute("data-theme");
      if (attr === "dark" || attr === "light") setTheme(attr);
      else setTheme("system");
    };
    sync();
    const mo = new MutationObserver(sync);
    mo.observe(document.documentElement, { attributes: true, attributeFilter: ["data-theme"] });
    return () => mo.disconnect();
  }, []);

  return (
    <Toaster
      theme={theme}
      position="top-center"
      richColors
      closeButton
      duration={3200}
      toastOptions={{
        style: {
          borderRadius: "12px",
          fontSize: "13.5px",
          fontWeight: 550,
          boxShadow: "var(--shadow-pop)",
        },
      }}
    />
  );
}
