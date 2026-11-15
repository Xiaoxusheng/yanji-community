"use client";

// 管理后台布局：独立侧边菜单 + 认证/权限守卫。
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { Layout, Menu, Typography, Spin, Result, Button } from "antd";
import {
  DashboardOutlined, UserOutlined, FileTextOutlined, CommentOutlined,
  FlagOutlined, AppstoreOutlined, TagsOutlined, NotificationOutlined,
  StopOutlined, SafetyCertificateOutlined, SettingOutlined, HistoryOutlined, ArrowLeftOutlined,
  TrophyOutlined, CalendarOutlined, SolutionOutlined,
} from "@ant-design/icons";
import "@ant-design/v5-patch-for-react-19";
import { useMe } from "@/lib/auth";

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

export default function AdminLayout({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const { data: me, isLoading } = useMe();
  const [allowed, setAllowed] = useState<boolean | null>(null);

  // 权限探测：尝试访问后台仪表盘（服务端 RBAC 是唯一裁决者）
  useEffect(() => {
    if (!me?.authenticated) return;
    let cancelled = false;
    fetch(`${process.env.NEXT_PUBLIC_API_BASE_URL || "http://localhost:8080/api/v1"}/admin/dashboard`, {
      credentials: "include",
    }).then((r) => {
      if (!cancelled) setAllowed(r.ok);
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
      <Result
        status="403"
        title="需要登录"
        subTitle="请先登录管理员账号"
        extra={<Button type="primary" onClick={() => router.push("/login")}>去登录</Button>}
      />
    );
  }
  if (allowed === false) {
    return (
      <Result
        status="403"
        title="没有后台访问权限"
        subTitle="你的账号未被授予管理权限。如需开通请联系超级管理员。"
        extra={<Button icon={<ArrowLeftOutlined />} onClick={() => router.push("/")}>返回社区</Button>}
      />
    );
  }

  return (
    <Layout style={{ minHeight: "100vh" }}>
      <Sider width={220} theme="light" style={{ borderRight: "1px solid #eee" }}>
        <div style={{ padding: "16px 20px", fontWeight: 800, fontSize: 17, color: "#4f46e5" }}>
          研记 · 管理后台
        </div>
        <Menu mode="inline" selectedKeys={[pathname]} items={items} style={{ borderInlineEnd: "none" }} />
        <div style={{ padding: 16 }}>
          <Link href="/" style={{ fontSize: 13, color: "#4f46e5" }}>
            ← 返回社区前台
          </Link>
        </div>
      </Sider>
      <Layout>
        <Header
          style={{
            background: "#fff", borderBottom: "1px solid #eee", display: "flex",
            alignItems: "center", padding: "0 24px", height: 52,
          }}
        >
          <Typography.Text type="secondary" style={{ fontSize: 13 }}>
            所有操作均记录审计日志；权限由服务端 RBAC 校验
          </Typography.Text>
        </Header>
        <Content style={{ padding: 20, background: "#f5f6f8" }}>{children}</Content>
      </Layout>
    </Layout>
  );
}
