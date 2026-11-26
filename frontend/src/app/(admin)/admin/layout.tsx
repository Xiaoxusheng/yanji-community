"use client";

// 管理后台布局：与前台统一的设计令牌（Ant Design ConfigProvider）+ 侧边导航 + 服务端 RBAC 守卫。
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { ConfigProvider, Layout, Menu, Typography, Spin, Result, Button, Tag } from "antd";
import {
  DashboardOutlined, UserOutlined, FileTextOutlined, CommentOutlined,
  FlagOutlined, AppstoreOutlined, TagsOutlined, NotificationOutlined,
  StopOutlined, SafetyCertificateOutlined, SettingOutlined, HistoryOutlined, ArrowLeftOutlined,
  TrophyOutlined, CalendarOutlined, SolutionOutlined,
} from "@ant-design/icons";
import "@ant-design/v5-patch-for-react-19";
import { useMe } from "@/lib/auth";
import { api } from "@/lib/api";

const { Sider, Content, Header } = Layout;

const items = [
  { key: "/admin", icon: <DashboardOutlined />, label: <Link href="/admin">仪表盘</Link> },
  { key: "/admin/users", icon: <UserOutlined />, label: <Link href="/admin/users">用户管理</Link> },
  { key: "/admin/posts", icon: <FileTextOutlined />, label: <Link href="/admin/posts">帖子管理</Link> },
  { key: "/admin/comments", icon: <CommentOutlined />, label: <Link href="/admin/comments">评论管理</Link> },
  { key: "/admin/reports", icon: <FlagOutlined />, label: <Link href="/admin/reports">举报处理</Link> },
  { key: "/admin/categories", icon: <AppstoreOutlined />, label: <Link href="/admin/categories">版块管理</Link> },
  { key: "/admin/tags", icon: <TagsOutlined />, label: <Link href="/admin/tags">标签管理</Link> },
  { key: "/admin/announcements", icon: <NotificationOutlined />, label: <Link href="/admin/announcements">公告管理</Link> },
  { key: "/admin/sensitive-words", icon: <StopOutlined />, label: <Link href="/admin/sensitive-words">敏感词</Link> },
  { key: "/admin/appeals", icon: <SolutionOutlined />, label: <Link href="/admin/appeals">申诉处理</Link> },
  { key: "/admin/activities", icon: <CalendarOutlined />, label: <Link href="/admin/activities">活动管理</Link> },
  { key: "/admin/badges", icon: <TrophyOutlined />, label: <Link href="/admin/badges">勋章管理</Link> },
  { key: "/admin/roles", icon: <SafetyCertificateOutlined />, label: <Link href="/admin/roles">角色权限</Link> },
  { key: "/admin/settings", icon: <SettingOutlined />, label: <Link href="/admin/settings">系统设置</Link> },
  { key: "/admin/logs", icon: <HistoryOutlined />, label: <Link href="/admin/logs">审计日志</Link> },
];

const adminTheme = {
  token: {
    colorPrimary: "#3b5bdb",
    colorInfo: "#3b5bdb",
    borderRadius: 10,
    colorBgLayout: "#f5f6f9",
    colorBorderSecondary: "rgba(15, 23, 42, 0.08)",
    fontFamily:
      '"Inter", -apple-system, BlinkMacSystemFont, "SF Pro Text", "PingFang SC", "Microsoft YaHei", sans-serif',
  },
  components: {
    Card: {
      borderRadiusLG: 14,
    },
    Menu: {
      itemBorderRadius: 8,
      itemSelectedBg: "rgba(59, 91, 219, 0.09)",
      itemSelectedColor: "#3b5bdb",
    },
  },
};

export default function AdminLayout({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const { data: me, isLoading } = useMe();
  const [allowed, setAllowed] = useState<boolean | null>(null);

  // 权限探测：尝试访问后台仪表盘（服务端 RBAC 是唯一裁决者）
  useEffect(() => {
    if (!me?.authenticated) return;
    let cancelled = false;
    api.get("/admin/dashboard").then(() => {
      if (!cancelled) setAllowed(true);
    }).catch(() => {
      if (!cancelled) setAllowed(false);
    });
    return () => {
      cancelled = true;
    };
  }, [me?.authenticated]);

  if (isLoading) {
    return (
      <div style={{ minHeight: "100vh", display: "flex", alignItems: "center", justifyContent: "center" }}>
        <Spin size="large" />
      </div>
    );
  }
  if (!me?.authenticated) {
    return (
      <ConfigProvider theme={adminTheme}>
        <Result
          status="403"
          title="需要登录"
          subTitle="请先登录管理员账号"
          extra={<Button type="primary" onClick={() => router.push("/login")}>去登录</Button>}
        />
      </ConfigProvider>
    );
  }
  if (allowed === false) {
    return (
      <ConfigProvider theme={adminTheme}>
        <Result
          status="403"
          title="没有后台访问权限"
          subTitle="你的账号未被授予管理权限。如需开通请联系超级管理员。"
          extra={<Button icon={<ArrowLeftOutlined />} onClick={() => router.push("/")}>返回社区</Button>}
        />
      </ConfigProvider>
    );
  }

  return (
    <ConfigProvider theme={adminTheme}>
      <Layout style={{ minHeight: "100vh" }}>
        <Sider
          width={232}
          theme="light"
          style={{
            borderRight: "1px solid rgba(15, 23, 42, 0.08)",
            boxShadow: "1px 0 8px rgba(15, 23, 42, 0.02)",
          }}
        >
          <div
            style={{
              padding: "18px 18px 14px",
              display: "flex",
              alignItems: "center",
              gap: 10,
              borderBottom: "1px solid rgba(15, 23, 42, 0.06)",
            }}
          >
            <span className="brand-mark" aria-hidden>
              研
            </span>
            <div>
              <div style={{ fontWeight: 750, fontSize: 15.5, color: "#0f172a", letterSpacing: "-0.02em" }}>
                研记控制台
              </div>
              <div style={{ fontSize: 11.5, color: "#64748b" }}>Governance & Ops</div>
            </div>
          </div>
          <div style={{ padding: "10px 8px" }}>
            <Menu mode="inline" selectedKeys={[pathname]} items={items} style={{ borderInlineEnd: "none" }} />
          </div>
          <div style={{ padding: "12px 18px", borderTop: "1px solid rgba(15, 23, 42, 0.06)" }}>
            <Link href="/" style={{ fontSize: 13, fontWeight: 550, color: "#3b5bdb" }}>
              ← 返回社区前台
            </Link>
          </div>
        </Sider>
        <Layout>
          <Header
            style={{
              background: "rgba(255, 255, 255, 0.86)",
              backdropFilter: "blur(14px)",
              borderBottom: "1px solid rgba(15, 23, 42, 0.08)",
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
              padding: "0 24px",
              height: 56,
              position: "sticky",
              top: 0,
              zIndex: 50,
            }}
          >
            <Typography.Text type="secondary" style={{ fontSize: 13 }}>
              所有管理状态变更均写入操作审计日志，权限由服务端 RBAC 强制校验
            </Typography.Text>
            {me.user && (
              <Tag color="geekblue" style={{ margin: 0, borderRadius: 999, padding: "1px 10px" }}>
                @{me.user.username}
              </Tag>
            )}
          </Header>
          <Content style={{ padding: 24, maxWidth: 1440, width: "100%", margin: "0 auto" }}>{children}</Content>
        </Layout>
      </Layout>
    </ConfigProvider>
  );
}
