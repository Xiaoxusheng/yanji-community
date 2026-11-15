"use client";

// 前台公共布局：头部 + 内容区 + 页脚。
import { SiteHeader, SiteFooter } from "@/components/layout/SiteChrome";

export default function SiteLayout({ children }: { children: React.ReactNode }) {
  return (
    <div style={{ minHeight: "100vh", display: "flex", flexDirection: "column" }}>
      <SiteHeader />
      <main className="site-container" style={{ flex: 1, paddingTop: 20, paddingBottom: 40, width: "100%" }}>
        {children}
      </main>
      <SiteFooter />
    </div>
  );
}
