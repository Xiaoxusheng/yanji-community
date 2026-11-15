"use client";

// 管理后台仪表盘：统计卡 + 7 日趋势（CSS 柱状图，无外部图表依赖）。
import { useQuery } from "@tanstack/react-query";
import { Card, Col, Row, Statistic, Spin, Alert } from "antd";
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
    <div>
      <Row gutter={[16, 16]}>
        {[
          { title: "注册用户", value: data.totals.users, extra: `今日 +${data.totals.users_today}` },
          { title: "帖子", value: data.totals.posts, extra: `今日 +${data.totals.posts_today}` },
          { title: "评论", value: data.totals.comments, extra: `今日 +${data.totals.comments_today}` },
          { title: "24h 活跃会话", value: data.totals.online_sessions, extra: "" },
        ].map((s) => (
          <Col xs={12} md={6} key={s.title}>
            <Card>
              <Statistic title={s.title} value={s.value} />
              {s.extra && <div style={{ color: "#999", fontSize: 12, marginTop: 4 }}>{s.extra}</div>}
            </Card>
          </Col>
        ))}
      </Row>

      <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
        <Col xs={24} md={8}>
          <Card>
            <Statistic title="待审核帖子" value={data.queues.pending_posts} valueStyle={{ color: data.queues.pending_posts > 0 ? "#d97706" : undefined }} />
            <a href="/admin/posts?status=pending_review" style={{ fontSize: 13 }}>前往处理 →</a>
          </Card>
        </Col>
        <Col xs={24} md={8}>
          <Card>
            <Statistic title="待处理举报" value={data.queues.pending_reports} valueStyle={{ color: data.queues.pending_reports > 0 ? "#dc2626" : undefined }} />
            <a href="/admin/reports" style={{ fontSize: 13 }}>前往处理 →</a>
          </Card>
        </Col>
        <Col xs={24} md={8}>
          <Card>
            <Statistic title="生效中处罚" value={data.queues.active_sanctions} />
            <a href="/admin/users" style={{ fontSize: 13 }}>查看用户 →</a>
          </Card>
        </Col>
      </Row>

      <Card title="近 7 日趋势" style={{ marginTop: 16 }}>
        <div style={{ display: "flex", gap: 16, alignItems: "flex-end", height: 160, padding: "0 8px" }}>
          {data.trends.map((t) => (
            <div key={t.day} style={{ flex: 1, textAlign: "center" }}>
              <div style={{ display: "flex", gap: 3, alignItems: "flex-end", height: 120, justifyContent: "center" }}>
                <div title={`帖子 ${t.posts}`} style={{ width: 12, height: `${(t.posts / maxTrend) * 100}%`, minHeight: 2, background: "#4f46e5", borderRadius: 3 }} />
                <div title={`用户 ${t.users}`} style={{ width: 12, height: `${(t.users / maxTrend) * 100}%`, minHeight: 2, background: "#16a34a", borderRadius: 3 }} />
                <div title={`评论 ${t.comments}`} style={{ width: 12, height: `${(t.comments / maxTrend) * 100}%`, minHeight: 2, background: "#2563eb", borderRadius: 3 }} />
              </div>
              <div style={{ fontSize: 12, color: "#999", marginTop: 6 }}>{t.day}</div>
            </div>
          ))}
        </div>
        <div style={{ display: "flex", gap: 16, marginTop: 10, fontSize: 13, color: "#666" }}>
          <span><span style={{ display: "inline-block", width: 10, height: 10, background: "#4f46e5", borderRadius: 2, marginRight: 4 }} />帖子</span>
          <span><span style={{ display: "inline-block", width: 10, height: 10, background: "#16a34a", borderRadius: 2, marginRight: 4 }} />新用户</span>
          <span><span style={{ display: "inline-block", width: 10, height: 10, background: "#2563eb", borderRadius: 2, marginRight: 4 }} />评论</span>
        </div>
      </Card>
    </div>
  );
}
