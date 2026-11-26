"use client";

// 后台：活动管理 / 申诉处理 / 勋章管理。
import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Table, Card, Button, Space, Tag, Modal, Form, Input, Select, InputNumber,
  DatePicker, message, Popconfirm, Statistic, Drawer,
} from "antd";
import { api, timeAgo } from "@/lib/api";
import type { Activity, Appeal, Badge } from "@/lib/types";

const { RangePicker } = DatePicker;

type Participant = {
  user_id: number;
  username: string;
  name: string;
  status: string;
  signed_at: string;
};

// ---------- 活动管理 ----------

export function AdminActivitiesPage() {
  const qc = useQueryClient();
  const [editing, setEditing] = useState<Activity | null>(null);
  const [creating, setCreating] = useState(false);
  const [partAct, setPartAct] = useState<Activity | null>(null);
  const [form] = Form.useForm();

  const list = useQuery({
    queryKey: ["admin-activities"],
    queryFn: () => api.get<{ items: Activity[] }>("/admin/activities"),
  });

  const participants = useQuery({
    enabled: !!partAct,
    queryKey: ["admin-activity-participants", partAct?.id],
    queryFn: () => api.get<{ items: Participant[] }>(`/admin/activities/${partAct!.id}/participants`),
  });

  const markAttended = async (userId: number) => {
    if (!partAct) return;
    try {
      const res = await api.post<{ message: string }>(`/admin/activities/${partAct.id}/attend`, { user_id: userId });
      message.success(res.message || "已标记出席");
      qc.invalidateQueries({ queryKey: ["admin-activity-participants", partAct.id] });
      qc.invalidateQueries({ queryKey: ["admin-activities"] });
    } catch (e) {
      message.error((e as Error).message);
    }
  };

  const save = async () => {
    const v = await form.validateFields();
    const range = v.range as [{ $d: Date } | null, { $d: Date } | null] | undefined;
    const payload: Record<string, unknown> = {
      title: v.title,
      description: v.description ?? "",
      location: v.location ?? "",
      capacity: v.capacity ?? 0,
      reward_points: v.reward_points ?? 0,
      status: v.status ?? "draft",
    };
    if (range?.[0]?.$d) payload.starts_at = new Date(range[0].$d).toISOString();
    else if (!editing && v.starts_at) payload.starts_at = v.starts_at;
    if (range?.[1]?.$d) payload.ends_at = new Date(range[1].$d).toISOString();
    try {
      if (editing) {
        await api.patch(`/admin/activities/${editing.id}`, payload);
        message.success("活动已更新");
      } else {
        await api.post("/admin/activities", payload);
        message.success("活动已创建");
      }
      setEditing(null);
      setCreating(false);
      form.resetFields();
      qc.invalidateQueries({ queryKey: ["admin-activities"] });
    } catch (e) {
      message.error((e as Error).message);
    }
  };

  return (
    <Card
      title="活动管理"
      extra={<Button type="primary" onClick={() => { setCreating(true); form.resetFields(); }}>新建活动</Button>}
    >
      <Table<Activity>
        rowKey="id"
        loading={list.isLoading}
        dataSource={list.data?.items ?? []}
        pagination={false}
        columns={[
          { title: "活动", dataIndex: "title" },
          { title: "时间", width: 200, render: (_, r) => new Date(r.starts_at).toLocaleString("zh-CN") },
          { title: "报名", width: 110, render: (_, r) => `${r.participant_count}/${r.capacity || "∞"}` },
          { title: "积分奖励", dataIndex: "reward_points", width: 90 },
          {
            title: "状态", dataIndex: "status", width: 90,
            render: (s: string) => (
              <Tag color={s === "published" ? "green" : s === "cancelled" ? "red" : s === "finished" ? "default" : "gold"}>{s}</Tag>
            ),
          },
          {
            title: "操作", width: 290,
            render: (_, r) => (
              <Space size={4} wrap>
                <Button size="small" onClick={() => { setEditing(r); form.setFieldsValue({ title: r.title, description: r.description, location: r.location, capacity: r.capacity, reward_points: r.reward_points, status: r.status }); }}>编辑</Button>
                <Button size="small" onClick={() => setPartAct(r)}>名单 / 签到</Button>
                {r.status === "draft" && (
                  <Button size="small" type="primary" onClick={async () => { await api.patch(`/admin/activities/${r.id}`, { status: "published" }); message.success("已发布"); qc.invalidateQueries({ queryKey: ["admin-activities"] }); }}>发布</Button>
                )}
                {r.status === "published" && (
                  <Popconfirm title="确认取消该活动？" onConfirm={async () => { await api.patch(`/admin/activities/${r.id}`, { status: "cancelled" }); message.success("已取消"); qc.invalidateQueries({ queryKey: ["admin-activities"] }); }}>
                    <Button size="small" danger>取消活动</Button>
                  </Popconfirm>
                )}
              </Space>
            ),
          },
        ]}
      />
      <Modal
        title={editing ? `编辑活动：${editing.title}` : "新建活动"}
        open={!!editing || creating}
        onCancel={() => { setEditing(null); setCreating(false); }}
        onOk={save}
        okText="保存"
        width={560}
      >
        <Form form={form} layout="vertical">
          <Form.Item name="title" label="活动标题" rules={[{ required: true, message: "请输入标题" }]}>
            <Input maxLength={120} />
          </Form.Item>
          <Form.Item name="description" label="活动说明">
            <Input.TextArea rows={4} maxLength={10000} />
          </Form.Item>
          <Form.Item name="location" label="地点">
            <Input maxLength={100} />
          </Form.Item>
          <Form.Item name="range" label="活动时间（开始 ~ 结束）" rules={[{ required: !editing, message: "请选择时间" }]}>
            <RangePicker showTime style={{ width: "100%" }} />
          </Form.Item>
          <Space size="large">
            <Form.Item name="capacity" label="名额（0 不限）" initialValue={0}>
              <InputNumber min={0} />
            </Form.Item>
            <Form.Item name="reward_points" label="出席积分奖励" initialValue={0}>
              <InputNumber min={0} />
            </Form.Item>
            <Form.Item name="status" label="状态" initialValue="draft">
              <Select style={{ width: 120 }} options={[{ value: "draft", label: "草稿" }, { value: "published", label: "发布" }]} />
            </Form.Item>
          </Space>
        </Form>
      </Modal>
      <Drawer
        title={partAct ? `报名名单与签到：${partAct.title}` : ""}
        open={!!partAct}
        onClose={() => setPartAct(null)}
        width={600}
      >
        <Table<Participant>
          rowKey="user_id"
          loading={participants.isLoading}
          dataSource={participants.data?.items ?? []}
          pagination={false}
          locale={{ emptyText: "暂无报名记录" }}
          columns={[
            {
              title: "用户",
              render: (_, p) => (
                <span>{p.name} <span style={{ color: "#999" }}>@{p.username}</span></span>
              ),
            },
            {
              title: "状态", dataIndex: "status", width: 90,
              render: (s: string) => (
                <Tag color={s === "attended" ? "green" : s === "cancelled" ? "default" : "blue"}>
                  {s === "attended" ? "已出席" : s === "cancelled" ? "已取消" : "已报名"}
                </Tag>
              ),
            },
            {
              title: "报名时间", dataIndex: "signed_at", width: 130,
              render: (v: string) => timeAgo(v),
            },
            {
              title: "操作", width: 110,
              render: (_, p) =>
                p.status === "signed_up" ? (
                  <Popconfirm title={`确认标记 @${p.username} 已出席并结算积分？`} onConfirm={() => markAttended(p.user_id)}>
                    <Button size="small" type="primary">标记出席</Button>
                  </Popconfirm>
                ) : (
                  <span style={{ color: "#999", fontSize: 12 }}>—</span>
                ),
            },
          ]}
        />
      </Drawer>
    </Card>
  );
}

// ---------- 申诉处理 ----------

export function AdminAppealsPage() {
  const qc = useQueryClient();
  const [status, setStatus] = useState("pending");
  const [page, setPage] = useState(1);
  const [handling, setHandling] = useState<Appeal | null>(null);
  const [form] = Form.useForm();

  const list = useQuery({
    queryKey: ["admin-appeals", status, page],
    queryFn: () => {
      let url = `/admin/appeals?page=${page}&page_size=20`;
      if (status) url += `&status=${status}`;
      return api.get<{ items: Appeal[]; total: number }>(url);
    },
  });

  const handle = async (v: { accepted: boolean; note: string }) => {
    try {
      await api.post(`/admin/appeals/${handling!.id}/handle`, v);
      message.success("申诉已处理");
      setHandling(null);
      form.resetFields();
      qc.invalidateQueries({ queryKey: ["admin-appeals"] });
    } catch (e) {
      message.error((e as Error).message);
    }
  };

  return (
    <Card
      title={
        <Space>
          <span>申诉处理</span>
          <Select
            size="small"
            style={{ width: 110 }}
            value={status}
            onChange={(v) => { setStatus(v); setPage(1); }}
            options={[
              { value: "pending", label: "待处理" },
              { value: "accepted", label: "已通过" },
              { value: "rejected", label: "未通过" },
            ]}
          />
        </Space>
      }
    >
      <Table<Appeal>
        rowKey="id"
        loading={list.isLoading}
        dataSource={list.data?.items ?? []}
        pagination={{ current: page, pageSize: 20, total: list.data?.total ?? 0, onChange: setPage }}
        columns={[
          { title: "申诉人", dataIndex: "username", width: 110 },
          { title: "申诉对象", dataIndex: "target_label", ellipsis: true },
          { title: "理由", dataIndex: "reason", ellipsis: true },
          {
            title: "状态", dataIndex: "status", width: 90,
            render: (s: string) => (
              <Tag color={s === "accepted" ? "green" : s === "rejected" ? "red" : "gold"}>
                {s === "accepted" ? "已通过" : s === "rejected" ? "未通过" : "待处理"}
              </Tag>
            ),
          },
          { title: "提交时间", dataIndex: "created_at", width: 110, render: (v: string) => timeAgo(v) },
          {
            title: "操作", width: 110,
            render: (_, r) =>
              r.status === "pending" ? (
                <Button size="small" type="primary" onClick={() => setHandling(r)}>处理</Button>
              ) : (
                <span style={{ fontSize: 12, color: "#999" }}>{r.handle_note || "—"}</span>
              ),
          },
        ]}
      />
      <Modal
        title={`处理申诉 #${handling?.id ?? ""}`}
        open={!!handling}
        onCancel={() => setHandling(null)}
        onOk={() => form.submit()}
        okText="提交"
      >
        <p style={{ color: "#666", fontSize: 13 }}>通过申诉将解除对应处罚（原始处罚记录保留，不会覆盖）。</p>
        <Form form={form} layout="vertical" onFinish={handle} initialValues={{ accepted: true }}>
          <Form.Item name="accepted" label="处理结论">
            <Select
              options={[
                { value: true, label: "通过——解除处罚并通知用户" },
                { value: false, label: "驳回——维持原处理" },
              ]}
            />
          </Form.Item>
          <Form.Item name="note" label="处理说明（必填）" rules={[{ required: true, message: "请填写说明" }]}>
            <Input.TextArea rows={3} maxLength={500} />
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  );
}

// ---------- 勋章管理 ----------

export function AdminBadgesPage() {
  const qc = useQueryClient();
  const [creating, setCreating] = useState(false);
  const [form] = Form.useForm();

  const list = useQuery({
    queryKey: ["admin-badges"],
    queryFn: () => api.get<{ items: Badge[] }>("/admin/badges"),
  });

  const create = async () => {
    const v = await form.validateFields();
    try {
      await api.post("/admin/badges", v);
      message.success("勋章已创建");
      setCreating(false);
      form.resetFields();
      qc.invalidateQueries({ queryKey: ["admin-badges"] });
    } catch (e) {
      message.error((e as Error).message);
    }
  };

  return (
    <Card title="勋章管理" extra={<Button type="primary" onClick={() => setCreating(true)}>新建勋章</Button>}>
      <Statistic title="勋章总数" value={list.data?.items.length ?? 0} style={{ marginBottom: 16 }} />
      <Table<Badge>
        rowKey="id"
        loading={list.isLoading}
        dataSource={list.data?.items ?? []}
        pagination={false}
        columns={[
          { title: "勋章", render: (_, r) => <span>{r.icon} {r.name}</span> },
          { title: "Code", dataIndex: "code", render: (v: string) => <code>{v}</code> },
          { title: "说明", dataIndex: "description", ellipsis: true },
          { title: "持有者", dataIndex: "holder_count", width: 90 },
          {
            title: "授予方式", dataIndex: "condition_type", width: 100,
            render: (v: string) => <Tag color={v === "auto" ? "blue" : "default"}>{v === "auto" ? "规则自动" : "手动授予"}</Tag>,
          },
          {
            title: "启用", dataIndex: "is_enabled", width: 80,
            render: (v: boolean) => (v ? <Tag color="green">启用</Tag> : <Tag>停用</Tag>),
          },
        ]}
      />
      <p style={{ color: "#999", fontSize: 13, marginTop: 12 }}>
        手动授予：在「用户管理」详情抽屉中为用户授予/收回勋章。
      </p>
      <Modal title="新建勋章" open={creating} onCancel={() => setCreating(false)} onOk={create} okText="创建">
        <Form form={form} layout="vertical">
          <Form.Item name="code" label="Code（唯一）" rules={[{ required: true, pattern: /^[a-z0-9_]{2,30}$/, message: "小写字母/数字/下划线" }]}>
            <Input />
          </Form.Item>
          <Form.Item name="name" label="名称" rules={[{ required: true, message: "请输入名称" }]}>
            <Input maxLength={30} />
          </Form.Item>
          <Form.Item name="description" label="说明">
            <Input.TextArea rows={2} />
          </Form.Item>
          <Form.Item name="icon" label="图标（emoji）">
            <Input placeholder="🎖" maxLength={4} />
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  );
}
