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
  const [selectedRowKeys, setSelectedRowKeys] = useState<string[]>([]);
  const [batchPending, setBatchPending] = useState(false);
  const [rejectOpen, setRejectOpen] = useState<PostItem | null>(null);
  const [moveOpen, setMoveOpen] = useState<PostItem | null>(null);
  const [batchReasonAction, setBatchReasonAction] = useState<"reject" | "hide" | null>(null);
  const [rejectForm] = Form.useForm();
  const [moveForm] = Form.useForm();
  const [batchForm] = Form.useForm();

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

  const batchModerate = async (action: string, reason = "") => {
    if (selectedRowKeys.length === 0) return;
    if (selectedRowKeys.length > 50) {
      message.warning("批量操作一次最多支持 50 篇帖子");
      return;
    }
    setBatchPending(true);
    try {
      const res = await api.post<{ results: { id: string; ok: boolean; error?: string }[] }>(
        "/admin/posts/batch-moderate",
        { ids: selectedRowKeys, action, reason },
      );
      const okCount = res.results.filter((r) => r.ok).length;
      const failCount = res.results.length - okCount;
      if (failCount === 0) {
        message.success(`批量操作完成：${okCount} 条全部成功`);
      } else {
        message.warning(`批量操作完成：${okCount} 条成功，${failCount} 条失败（可能状态不匹配或无权限）`);
      }
      setSelectedRowKeys([]);
      qc.invalidateQueries({ queryKey: ["admin-posts"] });
    } catch (e) {
      message.error((e as Error).message);
    } finally {
      setBatchPending(false);
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
      {selectedRowKeys.length > 0 && (
        <div style={{ marginBottom: 12, padding: "8px 12px", background: "#f5f7fa", borderRadius: 6, display: "flex", alignItems: "center", justifyContent: "space-between", flexWrap: "wrap", gap: 8 }}>
          <span style={{ fontSize: 13 }}>已选择 <strong>{selectedRowKeys.length}</strong> 篇帖子</span>
          <Space size={6} wrap>
            <Button size="small" type="primary" loading={batchPending} onClick={() => batchModerate("approve")}>批量通过</Button>
            <Button size="small" danger loading={batchPending} onClick={() => setBatchReasonAction("reject")}>批量驳回</Button>
            <Button size="small" danger loading={batchPending} onClick={() => setBatchReasonAction("hide")}>批量隐藏</Button>
            <Button size="small" loading={batchPending} onClick={() => batchModerate("restore")}>批量恢复</Button>
            <Button size="small" loading={batchPending} onClick={() => batchModerate("lock")}>批量锁定</Button>
            <Button size="small" loading={batchPending} onClick={() => batchModerate("unlock")}>批量解锁</Button>
            <Button size="small" type="link" onClick={() => setSelectedRowKeys([])}>清空</Button>
          </Space>
        </div>
      )}

      <Table<PostItem>
        rowKey="id"
        loading={list.isLoading}
        dataSource={list.data?.items ?? []}
        rowSelection={{
          selectedRowKeys,
          onChange: (keys) => setSelectedRowKeys(keys as string[]),
        }}
        pagination={{ current: page, pageSize: 20, total: list.data?.total ?? 0, onChange: setPage }}
        columns={[
          {
            title: "标题", dataIndex: "title", ellipsis: true,
            render: (_, r) => (
              <Space size={4}>
                {r.is_pinned && <Tag color="purple">置顶</Tag>}
                {r.is_featured && <Tag color="gold">精华</Tag>}
                <Link href={`/p/${r.id}`} target="_blank" rel="noopener noreferrer">{r.title}</Link>
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
        onCancel={() => { setRejectOpen(null); rejectForm.resetFields(); }}
        onOk={() => rejectForm.submit()}
        okText="确认驳回"
      >
        <Form form={rejectForm} layout="vertical" onFinish={(v) => { moderate(rejectOpen!, "reject", v.reason); setRejectOpen(null); rejectForm.resetFields(); }}>
          <Form.Item name="reason" label="驳回原因（必填，将通知作者）" rules={[{ required: true, message: "请填写原因" }]}>
            <Input.TextArea rows={3} maxLength={500} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={`移动版块：${moveOpen?.title ?? ""}`}
        open={!!moveOpen}
        onCancel={() => { setMoveOpen(null); moveForm.resetFields(); }}
        onOk={() => moveForm.submit()}
        okText="移动"
      >
        <Form form={moveForm} layout="vertical" onFinish={(v) => { moderate(moveOpen!, "move_category", "", v.category); setMoveOpen(null); moveForm.resetFields(); }}>
          <Form.Item name="category" label="目标版块" rules={[{ required: true, message: "请选择版块" }]}>
            <Select options={(cats.data?.items ?? []).map((c) => ({ value: c.slug, label: c.name }))} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={batchReasonAction === "reject" ? `批量驳回（${selectedRowKeys.length} 篇）` : `批量隐藏（${selectedRowKeys.length} 篇）`}
        open={!!batchReasonAction}
        onCancel={() => { setBatchReasonAction(null); batchForm.resetFields(); }}
        onOk={() => batchForm.submit()}
        okText="确认执行"
      >
        <Form
          form={batchForm}
          layout="vertical"
          onFinish={(v) => {
            const act = batchReasonAction!;
            setBatchReasonAction(null);
            batchForm.resetFields();
            void batchModerate(act, v.reason);
          }}
        >
          <Form.Item name="reason" label="处理原因（必填，将通知作者）" rules={[{ required: true, message: "请填写原因" }]}>
            <Input.TextArea rows={3} maxLength={500} />
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  );
}
