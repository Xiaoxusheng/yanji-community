"use client";

// 角色权限 / 系统设置 / 审计日志。
import { useEffect, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Table, Card, Tag, Tabs, Input, Button, message, Spin, Typography } from "antd";
import { api, timeAgo, type PageData } from "@/lib/api";
import type { RoleRow, OpLogRow } from "@/lib/types";

export function AdminRolesPage() {
  const roles = useQuery({
    queryKey: ["admin-roles"],
    queryFn: () => api.get<{ items: RoleRow[] }>("/admin/roles"),
  });
  const perms = useQuery({
    queryKey: ["admin-permissions"],
    queryFn: () => api.get<{ items: { id: number; code: string; name: string; resource: string; action: string }[] }>("/admin/permissions"),
  });

  if (roles.isLoading || perms.isLoading) return <Spin style={{ display: "block", margin: "80px auto" }} />;

  return (
    <Card title="角色与权限">
      <Tabs
        items={[
          {
            key: "roles",
            label: "角色",
            children: (
              <Table<RoleRow>
                rowKey="id"
                loading={roles.isLoading}
                dataSource={roles.data?.items ?? []}
                pagination={false}
                expandable={{
                  expandedRowRender: (r) => (
                    <div style={{ display: "flex", flexWrap: "wrap", gap: 6 }}>
                      {r.permissions.map((p) => (
                        <Tag key={p} color="blue">{p}</Tag>
                      ))}
                    </div>
                  ),
                }}
                columns={[
                  { title: "角色", dataIndex: "name", render: (_, r) => <span>{r.name} <span style={{ color: "#999", fontSize: 12 }}>({r.code})</span></span> },
                  { title: "说明", dataIndex: "description" },
                  { title: "权限数", render: (_, r) => r.permissions.length },
                  { title: "系统角色", dataIndex: "is_system", width: 90, render: (v: boolean) => (v ? <Tag>是</Tag> : <Tag color="blue">自定义</Tag>) },
                ]}
              />
            ),
          },
          {
            key: "permissions",
            label: "权限点",
            children: (
              <Table
                rowKey="id"
                dataSource={perms.data?.items ?? []}
                pagination={false}
                columns={[
                  { title: "权限点", dataIndex: "code", render: (v: string) => <code>{v}</code> },
                  { title: "名称", dataIndex: "name" },
                  { title: "资源", dataIndex: "resource", width: 110 },
                  { title: "动作", dataIndex: "action", width: 110 },
                ]}
              />
            ),
          },
        ]}
      />
      <Typography.Paragraph type="secondary" style={{ marginTop: 16, fontSize: 13 }}>
        角色为系统内置（阶段 1）；自定义角色与权限编辑将在后续阶段开放。在「用户管理」中可向用户授予角色（版主需绑定版块范围）。
      </Typography.Paragraph>
    </Card>
  );
}

interface SettingRow {
  key: string;
  value: string;
  value_type: string;
  description: string;
  is_public: boolean;
}

export function AdminSettingsPage() {
  const qc = useQueryClient();
  const { data, isLoading } = useQuery({
    queryKey: ["admin-settings"],
    queryFn: () => api.get<{ items: SettingRow[] }>("/admin/settings"),
  });
  const [values, setValues] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (data) {
      const v: Record<string, string> = {};
      for (const it of data.items) v[it.key] = it.value;
      setValues(v);
    }
  }, [data]);

  if (isLoading) return <Spin style={{ display: "block", margin: "80px auto" }} />;

  const save = async () => {
    setSaving(true);
    try {
      await api.patch("/admin/settings", {
        items: Object.entries(values).map(([key, value]) => ({ key, value })),
      });
      message.success("设置已保存");
      qc.invalidateQueries({ queryKey: ["admin-settings"] });
    } catch (e) {
      message.error((e as Error).message);
    } finally {
      setSaving(false);
    }
  };

  return (
    <Card
      title="系统设置"
      extra={<Button type="primary" loading={saving} onClick={save}>保存全部</Button>}
    >
      <Table
        rowKey="key"
        dataSource={data?.items ?? []}
        pagination={false}
        columns={[
          {
            title: "配置项", dataIndex: "key", width: 200,
            render: (v: string, r) => (
              <div>
                <code>{v}</code>
                <div style={{ fontSize: 12, color: "#999" }}>{r.description}</div>
              </div>
            ),
          },
          {
            title: "值", dataIndex: "value",
            render: (_: string, r) => (
              <Input
                value={values[r.key] ?? ""}
                onChange={(e) => setValues({ ...values, [r.key]: e.target.value })}
                style={{ maxWidth: 360 }}
              />
            ),
          },
          { title: "类型", dataIndex: "value_type", width: 80 },
          { title: "公开", dataIndex: "is_public", width: 80, render: (v: boolean) => (v ? <Tag color="green">前台可见</Tag> : <Tag>内部</Tag>) },
        ]}
      />
    </Card>
  );
}

export function AdminLogsPage() {
  const [page, setPage] = useState(1);
  const ops = useQuery({
    queryKey: ["admin-oplogs", page],
    queryFn: () => api.get<PageData<OpLogRow>>(`/admin/operation-logs?page=${page}&page_size=20`),
  });
  const logins = useQuery({
    queryKey: ["admin-loginlogs", page],
    queryFn: () => api.get<PageData<{ id: number; username: string; success: boolean; ip: string; created_at: string }>>(`/admin/login-logs?page=${page}&page_size=20`),
  });

  return (
    <Card title="审计日志">
      <Tabs
        items={[
          {
            key: "ops",
            label: "操作日志",
            children: (
              <Table<OpLogRow>
                rowKey="id"
                loading={ops.isLoading}
                dataSource={ops.data?.items ?? []}
                pagination={{ current: page, pageSize: 20, total: ops.data?.total ?? 0, onChange: setPage }}
                columns={[
                  { title: "操作者", dataIndex: "actor", width: 100, render: (v: string) => v || "系统" },
                  { title: "动作", dataIndex: "action", render: (v: string) => <code style={{ fontSize: 12 }}>{v}</code> },
                  { title: "对象", width: 160, render: (_, r) => (r.target_type ? `${r.target_type} #${r.target_id}` : "—") },
                  { title: "备注", dataIndex: "note", ellipsis: true },
                  { title: "Request ID", dataIndex: "request_id", width: 130, ellipsis: true, render: (v: string) => <span style={{ fontSize: 11, color: "#999" }}>{v}</span> },
                  { title: "IP", dataIndex: "ip", width: 110 },
                  { title: "时间", dataIndex: "created_at", width: 130, render: (v: string) => timeAgo(v) },
                ]}
              />
            ),
          },
          {
            key: "logins",
            label: "登录日志",
            children: (
              <Table
                rowKey="id"
                loading={logins.isLoading}
                dataSource={logins.data?.items ?? []}
                pagination={{ pageSize: 20 }}
                columns={[
                  { title: "用户", dataIndex: "username" },
                  { title: "结果", dataIndex: "success", width: 90, render: (v: boolean) => (v ? <Tag color="green">成功</Tag> : <Tag color="red">失败</Tag>) },
                  { title: "IP", dataIndex: "ip" },
                  { title: "时间", dataIndex: "created_at", width: 160, render: (v: string) => new Date(v).toLocaleString("zh-CN") },
                ]}
              />
            ),
          },
        ]}
      />
    </Card>
  );
}
