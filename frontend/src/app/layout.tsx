import type { Metadata } from "next";
import "@/styles/globals.css";
import { Providers } from "@/lib/providers";
import { ToastHost } from "@/lib/toast";
import { AntdRegistry } from "@ant-design/nextjs-registry";

export const metadata: Metadata = {
  title: {
    default: "研记社区 —— 综合社区论坛",
    template: "%s · 研记社区",
  },
  description: "开放注册的综合社区论坛：版块讨论、长文章、评论互动、关注与收藏。",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="zh-CN">
      <body>
        <AntdRegistry>
          <Providers>
            {children}
            <ToastHost />
          </Providers>
        </AntdRegistry>
      </body>
    </html>
  );
}
