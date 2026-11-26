import type { Metadata } from "next";
import "@/styles/globals.css";
import { Providers } from "@/lib/providers";
import { ToastHost } from "@/lib/toast";
import { AntdRegistry } from "@ant-design/nextjs-registry";

export const metadata: Metadata = {
  title: {
    default: "研记社区 —— 综合讨论与内容社区",
    template: "%s · 研记社区",
  },
  description: "开放注册的综合社区论坛：技术讨论、长文章、问答投票、活动积分与即时私信。",
};

const themeInitScript = `
(function(){
  try {
    var t = localStorage.getItem('yanji-theme');
    if (t === 'dark' || t === 'light') {
      document.documentElement.setAttribute('data-theme', t);
    }
  } catch (e) {}
})();
`;

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="zh-CN" suppressHydrationWarning>
      <head>
        <script dangerouslySetInnerHTML={{ __html: themeInitScript }} />
      </head>
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
