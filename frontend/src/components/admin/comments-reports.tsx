"use client";

// 评论管理与举报处理。
import { useState } from "react";
import Link from "next/link";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Table, Card, Select, Space, Tag, Button, Modal, Form, Input, message, Tabs } from "antd";
import { api, timeAgo, type PageData } from "@/lib/api";
import type { CommentItem, ReportRow } from "@/lib/types";

const statusColors: Record<string, string> = {
  published: "green", pending_review: "gold", hidden: "red", deleted: "default",
};
const reportStatus: Record<string, { c: string; t: string }> = {
  pending: { c: "gold", t: "待处理" },
  processing: { c: "blue", t: "处理中" },
  resolved: { c: "green", t: "已处理" },
  dismissed: { c: "default", t: "已驳回" },
};
const reasonLabels: Record<string, string> = {
  spam: "垃圾广告", harassment: "人身攻击", porn_violence: "色情/暴力", illegal: "违法",
  infringement: "侵权", misinformation: "虚假信息", flame: "引战", other: "其他",
};

export function AdminCommentsPage() {
  const qc = useQueryClient();
  const [status, setStatus] = useState("");
  const [page, setPage] = useState(1);

  const list = useQuery({
    queryKey: ["admin-comments", status, page],
    queryFn: () => {
      let url = `/admin/comments?page=${page}&page_size=20`;
      if (status) url += `&status=${status}`;
      return api.get<PageData<CommentItem>>(url);
    },
  });

  const moderate = async (c: CommentItem, action: "hide" | "restore") => {
    const reason = action === "hide" ? window.prompt("隐藏原因（写入审计）：") ?? "" : "";
    if (action === "hide" && !reason) return;
    try {
      await api.post(`/admin/comments/${c.id}/moderate`, { action, reason });
      message.success("操作成功");
      qc.invalidateQueries({ queryKey: ["admin-comments"] });
    } catch (e) {
      message.error((e as Error).message);
    }
  };

  return (
    <Card
      title="评论管理"
      extra={
        <Select
          placeholder="状态"
          allowClear
          style={{ width: 140 }}
          value={status || undefined}
          onChange={(v) => { setStatus(v ?? ""); setPage(1); }}
          options={[
            { value: "published", label: "已发布" },
            { value: "hidden", label: "已隐藏" },
            { value: "pending_review", label: "待审核" },
          ]}
        />
      }
    >
      <Table<CommentItem>
        rowKey="id"
        loading={list.isLoading}
        dataSource={list.data?.items ?? []}
        pagination={{ current: page, pageSize: 20, total: list.data?.total ?? 0, onChange: setPage }}
        columns={[
          {
            title: "内容", dataIndex: "body", ellipsis: true,
            render: (v: string, r) => (
              <div>
                <div>{v.length > 80 ? v.slice(0, 80) + "…" : v}</div>
                <Link href={`/p/${r.post_id}`} target="_blank" style={{ fontSize: 12, color: "#4f46e5" }}>
                  来自帖子：{r.post_title}
                </Link>
              </div>
            ),
          },
          { title: "作者", dataIndex: "author", width: 110 },
          {
            title: "状态", dataIndex: "status", width: 90,
            render: (s: string) => <Tag color={statusColors[s]}>{s}</Tag>,
          },
          { title: "时间", dataIndex: "created_at", width: 110, render: (v: string) => timeAgo(v) },
          {
            title: "操作", width: 140,
            render: (_, r) =>
              r.status === "hidden" ? (
                <Button size="small" onClick={() => moderate(r, "restore")}>恢复</Button>
              ) : (
                <Button size="small" danger onClick={() => moderate(r, "hide")}>隐藏</Button>
              ),
          },
        ]}
      />
    </Card>
  );
}

export function AdminReportsPage() {
  const qc = useQueryClient();
  const [status, setStatus] = useState("pending");
  const [page, setPage] = useState(1);
  const [resolving, setResolving] = useState<ReportRow | null>(null);
  const [form] = Form.useForm();

  const list = useQuery({
    queryKey: ["admin-reports", status, page],
    queryFn: () => {
      let url = `/admin/reports?page=${page}&page_size=20`;
      if (status) url += `&status=${status}`;
      return api.get<PageData<ReportRow>>(url);
    },
  });

  const assign = async (r: ReportRow) => {
    try {
      await api.post(`/admin/reports/${r.id}/assign`);
      message.success("已受理");
      qc.invalidateQueries({ queryKey: ["admin-reports"] });
    } catch (e) {
      message.error((e as Error).message);
    }
  };

  const resolve = async (values: { resolution: string; note: string }) => {
    try {
      await api.post(`/admin/reports/${resolving!.id}/resolve`, values);
      message.success("举报已处理");
      setResolving(null);
      form.resetFields();
      qc.invalidateQueries({ queryKey: ["admin-reports"] });
    } catch (e) {
      message.error((e as Error).message);
    }
  };

  return (
    <Card
      title="举报处理"
      extra={
        <Select
          style={{ width: 130 }}
          value={status || undefined}
          onChange={(v) => { setStatus(v ?? ""); setPage(1); }}
          options={[
            { value: "pending", label: "待处理" },
            { value: "processing", label: "处理中" },
            { value: "resolved", label: "已处理" },
            { value: "dismissed", label: "已驳回" },
          ]}
        />
      }
    >
      <Table<ReportRow>
        rowKey="id"
        loading={list.isLoading}
        dataSource={list.data?.items ?? []}
        pagination={{ current: page, pageSize: 20, total: list.data?.total ?? 0, onChange: setPage }}
        columns={[
          {
            title: "举报对象", dataIndex: "target_label", ellipsis: true,
            render: (v: string, r) => (
              <div>
                <Tag>{r.target_type}</Tag>
                {v}
                <div style={{ fontSize: 12, color: "#999" }}>{reasonLabels[r.reason_code] ?? r.reason_code} · {r.description}</div>
              </div>
            ),
          },
          { title: "举报人", dataIndex: "reporter", width: 100 },
          {
            title: "状态", dataIndex: "status", width: 90,
            render: (s: string) => <Tag color={reportStatus[s]?.c}>{reportStatus[s]?.t ?? s}</Tag>,
          },
          { title: "处理人", dataIndex: "assignee", width: 90, render: (v: string) => v || "—" },
          { title: "时间", dataIndex: "created_at", width: 110, render: (v: string) => timeAgo(v) },
          {
            title: "操作", width: 150,
            render: (_, r) =>
              r.status === "pending" || r.status === "processing" ? (
                <Space size={4}>
                  {r.status === "pending" && <Button size="small" onClick={() => assign(r)}>受理</Button>}
                  <Button size="small" type="primary" onClick={() => setResolving(r)}>处理</Button>
                </Space>
              ) : (
                <span style={{ fontSize: 12, color: "#999" }}>{r.resolution_note || "—"}</span>
              ),
          },
        ]}
      />

      <Modal
        title={`处理举报 #${resolving?.id ?? ""}`}
        open={!!resolving}
        onCancel={() => setResolving(null)}
        onOk={() => form.submit()}
        okText="提交处理结果"
      >
        <Form form={form} layout="vertical" onFinish={resolve} initialValues={{ resolution: "action_taken" }}>
          <Form.Item name="resolution" label="处理结论" rules={[{ required: true }]}>
            <Select
              options={[
                { value: "action_taken", label: "核实属实，已处置内容" },
                { value: "dismissed", label: "举报不成立，驳回" },
              ]}
            />
          </Form.Item>
          <Form.Item noStyle shouldUpdate={(p, c) => p.resolution !== c.resolution}>
            {({ getFieldValue }) =>
              getFieldValue("resolution") === "action_taken" ? (
                <Form.Item name="note" label="处理说明（必填，通知目标作者）" rules={[{ required: true, message: "请填写处理说明" }]}>
                  <Input.TextArea rows={3} maxLength={500} />
                </Form.Item>
              ) : (
                <Form.Item name="note" label="驳回备注（可选）">
                  <Input.TextArea rows={2} maxLength={500} />
                </Form.Item>
              )
            }
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  );
}

export function AdminModerationPage() {
  return (
    <Tabs
      defaultActiveKey="reports"
      items={[
        { key: "reports", label: "举报处理", children: <AdminReportsPage /> },
        { key: "comments", label: "评论管理", children: <AdminCommentsPage /> },
      ]}
    />
  );
}
