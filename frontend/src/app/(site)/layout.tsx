"use client";

// 前台公共布局：头部 + 内容区 + 页脚 + 移动端底栏 + 返回顶部。
import { SiteHeader, SiteFooter, MobileBottomNav, BackToTop } from "@/components/layout/SiteChrome";

export default function SiteLayout({ children }: { children: React.ReactNode }) {
  return (
    <div style={{ minHeight: "100vh", display: "flex", flexDirection: "column" }}>
      <SiteHeader />
      <main className="site-container" style={{ flex: 1, paddingTop: 22, paddingBottom: 40, width: "100%" }}>
        {children}
      </main>
      <SiteFooter />
      <MobileBottomNav />
      <BackToTop />
    </div>
  );
}
