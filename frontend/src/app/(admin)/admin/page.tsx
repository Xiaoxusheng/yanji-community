"use client";

// 管理后台仪表盘：高质感指标卡 + 治理待办队列 + 7 日趋势柱状图。
import Link from "next/link";
import { useQuery } from "@tanstack/react-query";
import { Card, Col, Row, Statistic, Spin, Alert, Tag } from "antd";
import { api } from "@/lib/api";
import type { DashboardData } from "@/lib/types";

export default function AdminDashboardPage() {
  const { data, isLoading, error } = useQuery({
    queryKey: ["admin-dashboard"],
    queryFn: () => api.get<DashboardData>("/admin/dashboard"),
    refetchInterval: 60_000,
  });

  if (isLoading) return <div style={{ textAlign: "center", padding: 80 }}><Spin size="large" /></div>;
  if (error || !data) return <Alert type="error" showIcon message="仪表盘加载失败" description={(error as Error)?.message} />;

  const maxTrend = Math.max(1, ...data.trends.map((t) => Math.max(t.posts, t.users, t.comments)));

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 18 }}>
      <Row gutter={[16, 16]}>
        {[
          { title: "注册用户", value: data.totals.users, extra: `今日 +${data.totals.users_today}`, accent: "#3b5bdb" },
          { title: "累计帖子", value: data.totals.posts, extra: `今日 +${data.totals.posts_today}`, accent: "#6366f1" },
          { title: "累计评论", value: data.totals.comments, extra: `今日 +${data.totals.comments_today}`, accent: "#0ea5e9" },
          { title: "24h 活跃会话", value: data.totals.online_sessions, extra: "实时在线指标", accent: "#16a34a" },
        ].map((s) => (
          <Col xs={12} md={6} key={s.title}>
            <Card bordered={false} style={{ boxShadow: "0 1px 3px rgba(15, 23, 42, 0.05)" }}>
              <Statistic title={s.title} value={s.value} valueStyle={{ fontWeight: 720, letterSpacing: "-0.02em" }} />
              <div style={{ marginTop: 8 }}>
                <Tag color="default" style={{ borderRadius: 999, border: "none", background: "rgba(15, 23, 42, 0.05)", color: "#475569" }}>
                  <span style={{ display: "inline-block", width: 6, height: 6, borderRadius: 999, background: s.accent, marginRight: 5 }} />
                  {s.extra}
                </Tag>
              </div>
            </Card>
          </Col>
        ))}
      </Row>

      <Row gutter={[16, 16]}>
        <Col xs={24} md={8}>
          <Card bordered={false} style={{ boxShadow: "0 1px 3px rgba(15, 23, 42, 0.05)" }}>
            <Statistic
              title="待审核帖子"
              value={data.queues.pending_posts}
              valueStyle={{ color: data.queues.pending_posts > 0 ? "#d97706" : "#0f172a", fontWeight: 700 }}
            />
            <Link href="/admin/posts?status=pending_review" style={{ fontSize: 13, fontWeight: 550, color: "#3b5bdb", display: "inline-block", marginTop: 6 }}>
              前往审核队列 →
            </Link>
          </Card>
        </Col>
        <Col xs={24} md={8}>
          <Card bordered={false} style={{ boxShadow: "0 1px 3px rgba(15, 23, 42, 0.05)" }}>
            <Statistic
              title="待处理举报"
              value={data.queues.pending_reports}
              valueStyle={{ color: data.queues.pending_reports > 0 ? "#dc2626" : "#0f172a", fontWeight: 700 }}
            />
            <Link href="/admin/reports" style={{ fontSize: 13, fontWeight: 550, color: "#3b5bdb", display: "inline-block", marginTop: 6 }}>
              前往处理举报 →
            </Link>
          </Card>
        </Col>
        <Col xs={24} md={8}>
          <Card bordered={false} style={{ boxShadow: "0 1px 3px rgba(15, 23, 42, 0.05)" }}>
            <Statistic title="生效中处罚" value={data.queues.active_sanctions} valueStyle={{ fontWeight: 700 }} />
            <Link href="/admin/users" style={{ fontSize: 13, fontWeight: 550, color: "#3b5bdb", display: "inline-block", marginTop: 6 }}>
              查看用户治理 →
            </Link>
          </Card>
        </Col>
      </Row>

      <Card title="近 7 日社区活跃趋势" bordered={false} style={{ boxShadow: "0 1px 3px rgba(15, 23, 42, 0.05)" }}>
        <div style={{ display: "flex", gap: 16, alignItems: "flex-end", height: 176, padding: "8px 12px 0" }}>
          {data.trends.map((t) => (
            <div key={t.day} style={{ flex: 1, textAlign: "center" }}>
              <div style={{ display: "flex", gap: 5, alignItems: "flex-end", height: 132, justifyContent: "center" }}>
                <div
                  title={`帖子 ${t.posts}`}
                  style={{
                    width: 14,
                    height: `${(t.posts / maxTrend) * 100}%`,
                    minHeight: 4,
                    background: "linear-gradient(180deg, #6366f1 0%, #3b5bdb 100%)",
                    borderRadius: "5px 5px 2px 2px",
                  }}
                />
                <div
                  title={`新用户 ${t.users}`}
                  style={{
                    width: 14,
                    height: `${(t.users / maxTrend) * 100}%`,
                    minHeight: 4,
                    background: "linear-gradient(180deg, #34d399 0%, #15803d 100%)",
                    borderRadius: "5px 5px 2px 2px",
                  }}
                />
                <div
                  title={`评论 ${t.comments}`}
                  style={{
                    width: 14,
                    height: `${(t.comments / maxTrend) * 100}%`,
                    minHeight: 4,
                    background: "linear-gradient(180deg, #38bdf8 0%, #0284c7 100%)",
                    borderRadius: "5px 5px 2px 2px",
                  }}
                />
              </div>
              <div className="tabular-nums" style={{ fontSize: 12, color: "#64748b", marginTop: 8 }}>{t.day}</div>
            </div>
          ))}
        </div>
        <div style={{ display: "flex", gap: 18, marginTop: 14, paddingTop: 12, borderTop: "1px solid rgba(15, 23, 42, 0.06)", fontSize: 13, color: "#475569" }}>
          <span><span style={{ display: "inline-block", width: 10, height: 10, background: "#3b5bdb", borderRadius: 3, marginRight: 6 }} />帖子</span>
          <span><span style={{ display: "inline-block", width: 10, height: 10, background: "#15803d", borderRadius: 3, marginRight: 6 }} />新用户</span>
          <span><span style={{ display: "inline-block", width: 10, height: 10, background: "#0284c7", borderRadius: 3, marginRight: 6 }} />评论</span>
        </div>
      </Card>
    </div>
  );
}
