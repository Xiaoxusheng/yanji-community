"use client";

// 版块 / 标签 / 公告 / 敏感词管理（表格 + 弹窗 CRUD）。
import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Table, Card, Button, Space, Tag, Modal, Form, Input, InputNumber, Select, message, Popconfirm, Switch,
} from "antd";
import { api } from "@/lib/api";
import type { Category, Tag as TagType } from "@/lib/types";

const policyOptions = {
  visibility: [
    { value: "public", label: "公开" },
    { value: "private", label: "私有" },
    { value: "hidden", label: "隐藏" },
  ],
  posting: [
    { value: "all", label: "所有人可发帖" },
    { value: "verified", label: "仅已验证用户" },
    { value: "staff_only", label: "仅版主及以上" },
  ],
  review: [
    { value: "none", label: "直接发布" },
    { value: "new_users", label: "新用户需审核" },
    { value: "all", label: "全部审核" },
  ],
};

export function AdminCategoriesPage() {
  const qc = useQueryClient();
  const [editing, setEditing] = useState<Category | null>(null);
  const [creating, setCreating] = useState(false);
  const [form] = Form.useForm();

  const list = useQuery({
    queryKey: ["admin-categories"],
    queryFn: () => api.get<{ items: Category[] }>("/admin/categories"),
  });

  const openEdit = (c: Category) => {
    setEditing(c);
    form.setFieldsValue(c);
  };
  const save = async () => {
    const values = await form.validateFields();
    try {
      if (editing) {
        await api.patch(`/admin/categories/${editing.id}`, values);
        message.success("版块已更新");
      } else {
        await api.post("/admin/categories", values);
        message.success("版块已创建");
      }
      setEditing(null);
      setCreating(false);
      form.resetFields();
      qc.invalidateQueries({ queryKey: ["admin-categories"] });
    } catch (e) {
      message.error((e as Error).message);
    }
  };
  const remove = async (c: Category) => {
    try {
      await api.delete(`/admin/categories/${c.id}`);
      message.success("版块已删除");
      qc.invalidateQueries({ queryKey: ["admin-categories"] });
    } catch (e) {
      message.error((e as Error).message);
    }
  };

  const FormFields = (
    <Form form={form} layout="vertical">
      <Form.Item name="name" label="名称" rules={[{ required: true, message: "请输入名称" }]}>
        <Input maxLength={50} />
      </Form.Item>
      {!editing && (
        <Form.Item name="slug" label="Slug（URL 标识）" rules={[{ required: true, pattern: /^[a-z0-9][a-z0-9-]{1,48}[a-z0-9]$/, message: "小写字母/数字/连字符" }]}>
          <Input placeholder="如 tech" />
        </Form.Item>
      )}
      <Form.Item name="description" label="描述">
        <Input.TextArea rows={2} maxLength={1000} />
      </Form.Item>
      <Form.Item name="icon" label="图标（emoji）">
        <Input placeholder="📁" maxLength={4} />
      </Form.Item>
      <Form.Item name="sort_order" label="排序（越小越前）" initialValue={0}>
        <InputNumber style={{ width: "100%" }} />
      </Form.Item>
      <Form.Item name="visibility" label="可见性" initialValue="public">
        <Select options={policyOptions.visibility} />
      </Form.Item>
      <Form.Item name="posting_policy" label="发帖权限" initialValue="all">
        <Select options={policyOptions.posting} />
      </Form.Item>
      <Form.Item name="review_policy" label="审核策略" initialValue="none">
        <Select options={policyOptions.review} />
      </Form.Item>
      <Form.Item name="status" label="状态" initialValue="active">
        <Select options={[{ value: "active", label: "正常" }, { value: "archived", label: "归档（只读）" }]} />
      </Form.Item>
    </Form>
  );

  return (
    <Card
      title="版块管理"
      extra={
        <Button type="primary" onClick={() => { setCreating(true); form.resetFields(); }}>
          新建版块
        </Button>
      }
    >
      <Table<Category>
        rowKey="id"
        loading={list.isLoading}
        dataSource={list.data?.items ?? []}
        pagination={false}
        columns={[
          { title: "ID", dataIndex: "id", width: 60 },
          { title: "名称", dataIndex: "name", render: (_, r) => <span>{r.icon} {r.name}</span> },
          { title: "Slug", dataIndex: "slug" },
          { title: "帖子数", dataIndex: "post_count", width: 80 },
          { title: "可见性", dataIndex: "visibility", width: 80, render: (v: string) => <Tag>{v}</Tag> },
          { title: "发帖", dataIndex: "posting_policy", width: 100, render: (v: string) => <Tag color="blue">{v}</Tag> },
          { title: "审核", dataIndex: "review_policy", width: 90, render: (v: string) => <Tag color="gold">{v}</Tag> },
          { title: "状态", dataIndex: "status", width: 80, render: (v: string) => <Tag color={v === "active" ? "green" : "default"}>{v}</Tag> },
          {
            title: "操作", width: 140,
            render: (_, r) => (
              <Space size={4}>
                <Button size="small" onClick={() => openEdit(r)}>编辑</Button>
                <Popconfirm title="确定删除该版块？" description="仅可删除无帖子的版块" onConfirm={() => remove(r)}>
                  <Button size="small" danger>删除</Button>
                </Popconfirm>
              </Space>
            ),
          },
        ]}
      />
      <Modal
        title={editing ? `编辑版块：${editing.name}` : "新建版块"}
        open={!!editing || creating}
        onCancel={() => { setEditing(null); setCreating(false); }}
        onOk={save}
        okText="保存"
      >
        {FormFields}
      </Modal>
    </Card>
  );
}

export function AdminTagsPage() {
  const qc = useQueryClient();
  const [page, setPage] = useState(1);
  const [creating, setCreating] = useState(false);
  const [form] = Form.useForm();

  const list = useQuery({
    queryKey: ["admin-tags", page],
    queryFn: () => api.get<{ items: TagType[]; total: number }>("/admin/tags?page_size=20&page=" + page).catch(() => api.get<{ items: TagType[]; total: number }>("/admin/tags?page_size=20&page=" + page)),
  });

  const create = async () => {
    const values = await form.validateFields();
    try {
      await api.post("/admin/tags", values);
      message.success("标签已创建");
      setCreating(false);
      form.resetFields();
      qc.invalidateQueries({ queryKey: ["admin-tags"] });
    } catch (e) {
      message.error((e as Error).message);
    }
  };
  const setStatus = async (t: TagType, status: string) => {
    try {
      await api.patch(`/admin/tags/${t.id}`, { status });
      message.success("状态已更新");
      qc.invalidateQueries({ queryKey: ["admin-tags"] });
    } catch (e) {
      message.error((e as Error).message);
    }
  };
  const remove = async (t: TagType) => {
    try {
      await api.delete(`/admin/tags/${t.id}`);
      message.success("标签已删除");
      qc.invalidateQueries({ queryKey: ["admin-tags"] });
    } catch (e) {
      message.error((e as Error).message);
    }
  };

  return (
    <Card
      title="标签管理"
      extra={<Button type="primary" onClick={() => setCreating(true)}>新建标签</Button>}
    >
      <Table<TagType>
        rowKey="id"
        loading={list.isLoading}
        dataSource={list.data?.items ?? []}
        pagination={{ current: page, pageSize: 20, total: list.data?.total ?? 0, onChange: setPage }}
        columns={[
          { title: "ID", dataIndex: "id", width: 60 },
          { title: "名称", dataIndex: "name" },
          { title: "Slug", dataIndex: "slug" },
          { title: "使用量", dataIndex: "post_count", width: 90 },
          {
            title: "状态", dataIndex: "status", width: 90,
            render: (s: string, r) => (
              <Select size="small" value={s} style={{ width: 90 }} onChange={(v) => setStatus(r, v)}
                options={[{ value: "active", label: "正常" }, { value: "hidden", label: "隐藏" }, { value: "disabled", label: "禁用" }]} />
            ),
          },
          {
            title: "操作", width: 80,
            render: (_, r) => (
              <Popconfirm title="删除标签将移除所有关联，确定？" onConfirm={() => remove(r)}>
                <Button size="small" danger>删除</Button>
              </Popconfirm>
            ),
          },
        ]}
      />
      <Modal title="新建标签" open={creating} onCancel={() => setCreating(false)} onOk={create} okText="创建">
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="名称" rules={[{ required: true, message: "请输入名称" }]}>
            <Input maxLength={30} />
          </Form.Item>
          <Form.Item name="slug" label="Slug" rules={[{ required: true, pattern: /^[a-z0-9][a-z0-9-]{0,48}$/, message: "小写字母/数字/连字符" }]}>
            <Input />
          </Form.Item>
          <Form.Item name="description" label="描述">
            <Input.TextArea rows={2} />
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  );
}

export function AdminAnnouncementsPage() {
  const qc = useQueryClient();
  const [editing, setEditing] = useState<{ id?: number } | null>(null);
  const [form] = Form.useForm();

  const list = useQuery({
    queryKey: ["admin-announcements"],
    queryFn: () => api.get<{ items: { id: number; title: string; body: string; status: string; author: string; published_at: string | null; created_at: string }[] }>("/admin/announcements"),
  });

  const save = async () => {
    const values = await form.validateFields();
    try {
      if (editing?.id) {
        await api.patch(`/admin/announcements/${editing.id}`, values);
        message.success("公告已更新");
      } else {
        await api.post("/admin/announcements", values);
        message.success("公告已创建");
      }
      setEditing(null);
      form.resetFields();
      qc.invalidateQueries({ queryKey: ["admin-announcements"] });
    } catch (e) {
      message.error((e as Error).message);
    }
  };
  const remove = async (id: number) => {
    try {
      await api.delete(`/admin/announcements/${id}`);
      message.success("公告已删除");
      qc.invalidateQueries({ queryKey: ["admin-announcements"] });
    } catch (e) {
      message.error((e as Error).message);
    }
  };

  return (
    <Card title="公告管理" extra={<Button type="primary" onClick={() => { setEditing({}); form.resetFields(); }}>新建公告</Button>}>
      <Table
        rowKey="id"
        loading={list.isLoading}
        dataSource={list.data?.items ?? []}
        pagination={false}
        columns={[
          { title: "标题", dataIndex: "title" },
          { title: "状态", dataIndex: "status", width: 90, render: (s: string) => <Tag color={s === "published" ? "green" : s === "archived" ? "default" : "gold"}>{s}</Tag> },
          { title: "作者", dataIndex: "author", width: 100 },
          { title: "创建时间", dataIndex: "created_at", width: 120, render: (v: string) => new Date(v).toLocaleDateString("zh-CN") },
          {
            title: "操作", width: 200,
            render: (_, r) => (
              <Space size={4}>
                <Button size="small" onClick={() => { setEditing(r); form.setFieldsValue(r); }}>编辑</Button>
                {r.status !== "published" && (
                  <Button size="small" type="primary" onClick={async () => { await api.patch(`/admin/announcements/${r.id}`, { status: "published" }); message.success("已发布"); qc.invalidateQueries({ queryKey: ["admin-announcements"] }); }}>发布</Button>
                )}
                {r.status === "published" && (
                  <Button size="small" onClick={async () => { await api.patch(`/admin/announcements/${r.id}`, { status: "archived" }); message.success("已归档"); qc.invalidateQueries({ queryKey: ["admin-announcements"] }); }}>归档</Button>
                )}
                <Popconfirm title="确定删除？" onConfirm={() => remove(r.id)}>
                  <Button size="small" danger>删除</Button>
                </Popconfirm>
              </Space>
            ),
          },
        ]}
      />
      <Modal title={editing?.id ? "编辑公告" : "新建公告"} open={!!editing} onCancel={() => setEditing(null)} onOk={save} okText="保存">
        <Form form={form} layout="vertical">
          <Form.Item name="title" label="标题" rules={[{ required: true, message: "请输入标题" }]}>
            <Input maxLength={200} />
          </Form.Item>
          <Form.Item name="body" label="内容">
            <Input.TextArea rows={5} maxLength={10000} />
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  );
}

export function AdminSensitiveWordsPage() {
  const qc = useQueryClient();
  const [creating, setCreating] = useState(false);
  const [form] = Form.useForm();

  const list = useQuery({
    queryKey: ["admin-words"],
    queryFn: () => api.get<{ items: { id: number; word: string; match_type: string; action: string; is_enabled: boolean; created_at: string }[] }>("/admin/sensitive-words"),
  });

  const create = async () => {
    const values = await form.validateFields();
    try {
      await api.post("/admin/sensitive-words", values);
      message.success("敏感词已添加");
      setCreating(false);
      form.resetFields();
      qc.invalidateQueries({ queryKey: ["admin-words"] });
    } catch (e) {
      message.error((e as Error).message);
    }
  };
  const toggle = async (id: number, enabled: boolean) => {
    try {
      await api.patch(`/admin/sensitive-words/${id}`, { is_enabled: enabled });
      qc.invalidateQueries({ queryKey: ["admin-words"] });
    } catch (e) {
      message.error((e as Error).message);
    }
  };
  const remove = async (id: number) => {
    try {
      await api.delete(`/admin/sensitive-words/${id}`);
      message.success("已删除");
      qc.invalidateQueries({ queryKey: ["admin-words"] });
    } catch (e) {
      message.error((e as Error).message);
    }
  };

  return (
    <Card title="敏感词管理" extra={<Button type="primary" onClick={() => setCreating(true)}>添加敏感词</Button>}>
      <Table
        rowKey="id"
        loading={list.isLoading}
        dataSource={list.data?.items ?? []}
        pagination={false}
        columns={[
          { title: "词语", dataIndex: "word" },
          { title: "匹配", dataIndex: "match_type", width: 90, render: (v: string) => <Tag>{v === "exact" ? "精确" : "包含"}</Tag> },
          {
            title: "动作", dataIndex: "action", width: 90,
            render: (v: string) => <Tag color={v === "block" ? "red" : "gold"}>{v === "block" ? "拒绝发布" : "转审核"}</Tag>,
          },
          {
            title: "启用", dataIndex: "is_enabled", width: 80,
            render: (v: boolean, r) => <Switch checked={v} onChange={(nv) => toggle(r.id, nv)} />,
          },
          {
            title: "操作", width: 80,
            render: (_, r) => <Popconfirm title="确定删除？" onConfirm={() => remove(r.id)}><Button size="small" danger>删除</Button></Popconfirm>,
          },
        ]}
      />
      <Modal title="添加敏感词" open={creating} onCancel={() => setCreating(false)} onOk={create} okText="添加">
        <Form form={form} layout="vertical" initialValues={{ match_type: "contains", action: "block" }}>
          <Form.Item name="word" label="词语" rules={[{ required: true, message: "请输入词语" }]}>
            <Input maxLength={100} />
          </Form.Item>
          <Form.Item name="match_type" label="匹配方式">
            <Select options={[{ value: "contains", label: "包含匹配" }, { value: "exact", label: "精确匹配" }]} />
          </Form.Item>
          <Form.Item name="action" label="命中动作">
            <Select options={[{ value: "block", label: "拒绝发布" }, { value: "review", label: "转入人工审核" }]} />
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  );
}

