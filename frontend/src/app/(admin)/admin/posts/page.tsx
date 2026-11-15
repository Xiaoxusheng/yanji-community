"use client";

// 帖子管理：全状态列表 + 审核操作（通过/驳回/隐藏/恢复/锁定/置顶/加精/移动版块）。
import { useState } from "react";
import Link from "next/link";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Table, Card, Select, Space, Tag, Button, Input, Modal, Form, message } from "antd";
import { api, timeAgo, type PageData } from "@/lib/api";
import type { PostItem } from "@/lib/types";

const statusColors: Record<string, string> = {
  draft: "default", pending_review: "gold", published: "green",
  rejected: "red", hidden: "red", locked: "blue", deleted: "default",
};

export default function AdminPostsPage() {
  const qc = useQueryClient();
  const [status, setStatus] = useState("");
  const [q, setQ] = useState("");
  const [page, setPage] = useState(1);
  const [rejectOpen, setRejectOpen] = useState<PostItem | null>(null);
  const [moveOpen, setMoveOpen] = useState<PostItem | null>(null);
  const [form] = Form.useForm();

  const list = useQuery({
    queryKey: ["admin-posts", status, q, page],
    queryFn: () => {
      let url = `/admin/posts?page=${page}&page_size=20`;
      if (status) url += `&status=${status}`;
      if (q) url += `&q=${encodeURIComponent(q)}`;
      return api.get<PageData<PostItem>>(url);
    },
  });
  const cats = useQuery({
    queryKey: ["admin-categories"],
    queryFn: () => api.get<{ items: { id: number; name: string; slug: string }[] }>("/admin/categories"),
  });

  const moderate = async (post: PostItem, action: string, reason = "", categorySlug = "") => {
    try {
      await api.post(`/admin/posts/${post.id}/moderate`, { action, reason, category_slug: categorySlug });
      message.success(`操作成功：${action}`);
      qc.invalidateQueries({ queryKey: ["admin-posts"] });
    } catch (e) {
      message.error((e as Error).message);
    }
  };

  return (
    <Card
      title="帖子管理"
      extra={
        <Space>
          <Input.Search placeholder="搜索标题" allowClear onSearch={(v) => { setQ(v); setPage(1); }} style={{ width: 200 }} />
          <Select
            placeholder="状态筛选"
            allowClear
            style={{ width: 140 }}
            value={status || undefined}
            onChange={(v) => { setStatus(v ?? ""); setPage(1); }}
            options={[
              { value: "pending_review", label: "待审核" },
              { value: "published", label: "已发布" },
              { value: "hidden", label: "已隐藏" },
              { value: "locked", label: "已锁定" },
              { value: "draft", label: "草稿" },
              { value: "rejected", label: "已驳回" },
              { value: "deleted", label: "已删除" },
            ]}
          />
        </Space>
      }
    >
      <Table<PostItem>
        rowKey="id"
        loading={list.isLoading}
        dataSource={list.data?.items ?? []}
        pagination={{ current: page, pageSize: 20, total: list.data?.total ?? 0, onChange: setPage }}
        columns={[
          {
            title: "标题", dataIndex: "title", ellipsis: true,
            render: (_, r) => (
              <Space size={4}>
                {r.is_pinned && <Tag color="purple">置顶</Tag>}
                {r.is_featured && <Tag color="gold">精华</Tag>}
                <Link href={`/p/${r.id}`} target="_blank">{r.title}</Link>
              </Space>
            ),
          },
          { title: "作者", dataIndex: ["author", "username"], width: 110 },
          { title: "版块", dataIndex: ["category", "name"], width: 100 },
          {
            title: "状态", dataIndex: "status", width: 100,
            render: (s: string) => <Tag color={statusColors[s]}>{s}</Tag>,
          },
          {
            title: "数据", width: 130,
            render: (_, r) => (
              <span style={{ fontSize: 12, color: "#999" }}>
                👁 {r.view_count} 👍 {r.like_count} 💬 {r.comment_count}
              </span>
            ),
          },
          { title: "发布时间", dataIndex: "published_at", width: 110, render: (v: string | null) => (v ? timeAgo(v) : "—") },
          {
            title: "操作", width: 230,
            render: (_, r) => (
              <Space size={4} wrap>
                {r.status === "pending_review" && (
                  <>
                    <Button size="small" type="primary" onClick={() => moderate(r, "approve")}>通过</Button>
                    <Button size="small" danger onClick={() => setRejectOpen(r)}>驳回</Button>
                  </>
                )}
                {r.status === "published" && (
                  <>
                    <Button size="small" onClick={() => moderate(r, r.is_pinned ? "unpin" : "pin")}>{r.is_pinned ? "取消置顶" : "置顶"}</Button>
                    <Button size="small" onClick={() => moderate(r, r.is_featured ? "unfeature" : "feature")}>{r.is_featured ? "取消精华" : "加精"}</Button>
                    <Button size="small" onClick={() => moderate(r, "lock")}>锁定</Button>
                    <Button size="small" danger onClick={() => {
                      const reason = window.prompt("隐藏原因（将通知作者）：");
                      if (reason !== null) moderate(r, "hide", reason);
                    }}>隐藏</Button>
                    <Button size="small" onClick={() => setMoveOpen(r)}>移动</Button>
                  </>
                )}
                {r.status === "locked" && <Button size="small" onClick={() => moderate(r, "unlock")}>解锁</Button>}
                {r.status === "hidden" && <Button size="small" onClick={() => moderate(r, "restore")}>恢复</Button>}
                {r.status === "rejected" && <Button size="small" onClick={() => moderate(r, "approve")}>改判通过</Button>}
              </Space>
            ),
          },
        ]}
      />

      <Modal
        title={`驳回：${rejectOpen?.title ?? ""}`}
        open={!!rejectOpen}
        onCancel={() => setRejectOpen(null)}
        onOk={() => form.submit()}
        okText="确认驳回"
      >
        <Form form={form} layout="vertical" onFinish={(v) => { moderate(rejectOpen!, "reject", v.reason); setRejectOpen(null); form.resetFields(); }}>
          <Form.Item name="reason" label="驳回原因（必填，将通知作者）" rules={[{ required: true, message: "请填写原因" }]}>
            <Input.TextArea rows={3} maxLength={500} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={`移动版块：${moveOpen?.title ?? ""}`}
        open={!!moveOpen}
        onCancel={() => setMoveOpen(null)}
        onOk={() => form.submit()}
        okText="移动"
      >
        <Form form={form} layout="vertical" onFinish={(v) => { moderate(moveOpen!, "move_category", "", v.category); setMoveOpen(null); form.resetFields(); }}>
          <Form.Item name="category" label="目标版块" rules={[{ required: true, message: "请选择版块" }]}>
            <Select options={(cats.data?.items ?? []).map((c) => ({ value: c.slug, label: c.name }))} />
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  );
}
