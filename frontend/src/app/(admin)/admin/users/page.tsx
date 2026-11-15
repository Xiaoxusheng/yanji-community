"use client";

// 用户管理：列表筛选、详情抽屉、处罚/解封、角色授予。
import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Table, Card, Input, Select, Space, Tag, Button, Drawer, Descriptions,
  Modal, Form, Select as AntSelect, message, Tabs, Popconfirm,
} from "antd";
import { api, timeAgo, type PageData } from "@/lib/api";
import type { UserRow, SanctionRow, RoleRow } from "@/lib/types";

const statusColors: Record<string, string> = {
  active: "green", pending: "blue", suspended: "orange", banned: "red", deleted: "default",
};
const sanctionColors: Record<string, string> = {
  warn: "gold", mute: "orange", post_ban: "volcano", temp_ban: "red", perma_ban: "red",
};

export default function AdminUsersPage() {
  const qc = useQueryClient();
  const [q, setQ] = useState("");
  const [status, setStatus] = useState("");
  const [page, setPage] = useState(1);
  const [detail, setDetail] = useState<UserRow | null>(null);
  const [sanctionOpen, setSanctionOpen] = useState(false);
  const [roleOpen, setRoleOpen] = useState(false);
  const [form] = Form.useForm();
  const [roleForm] = Form.useForm();

  const list = useQuery({
    queryKey: ["admin-users", q, status, page],
    queryFn: () => {
      let url = `/admin/users?page=${page}&page_size=20`;
      if (q) url += `&q=${encodeURIComponent(q)}`;
      if (status) url += `&status=${status}`;
      return api.get<PageData<UserRow>>(url);
    },
  });

  const detailData = useQuery({
    enabled: !!detail,
    queryKey: ["admin-user", detail?.id],
    queryFn: () =>
      api.get<{
        user: UserRow;
        bio: string;
        website: string;
        location: string;
        roles: { id: number; role_code: string; role_name: string; scope_type: string; scope_name: string }[];
        sanctions: SanctionRow[];
      }>(`/admin/users/${detail!.id}`),
  });

  const roles = useQuery({
    enabled: roleOpen,
    queryKey: ["admin-roles"],
    queryFn: () => api.get<{ items: RoleRow[] }>("/admin/roles"),
  });

  const createSanction = async (values: { sanction_type: string; reason: string; days: number }) => {
    try {
      await api.post(`/admin/users/${detail!.id}/sanctions`, values);
      message.success("处罚已创建");
      setSanctionOpen(false);
      form.resetFields();
      qc.invalidateQueries({ queryKey: ["admin-user", detail?.id] });
      qc.invalidateQueries({ queryKey: ["admin-users"] });
    } catch (e) {
      message.error((e as Error).message);
    }
  };

  const liftSanction = async (sid: number) => {
    try {
      await api.delete(`/admin/users/${detail!.id}/sanctions/${sid}`);
      message.success("处罚已解除");
      qc.invalidateQueries({ queryKey: ["admin-user", detail?.id] });
    } catch (e) {
      message.error((e as Error).message);
    }
  };

  const unban = async () => {
    try {
      await api.post(`/admin/users/${detail!.id}/unban`);
      message.success("已解封");
      qc.invalidateQueries({ queryKey: ["admin-user", detail?.id] });
      qc.invalidateQueries({ queryKey: ["admin-users"] });
    } catch (e) {
      message.error((e as Error).message);
    }
  };

  const assignRole = async (values: { role_id: number; scope: string; category_id?: number }) => {
    try {
      await api.post(`/admin/users/${detail!.id}/roles`, values);
      message.success("角色已授予");
      setRoleOpen(false);
      roleForm.resetFields();
      qc.invalidateQueries({ queryKey: ["admin-user", detail?.id] });
    } catch (e) {
      message.error((e as Error).message);
    }
  };

  const removeRole = async (roleId: number) => {
    try {
      await api.delete(`/admin/users/${detail!.id}/roles?role_id=${roleId}`);
      message.success("角色已移除");
      qc.invalidateQueries({ queryKey: ["admin-user", detail?.id] });
    } catch (e) {
      message.error((e as Error).message);
    }
  };

  return (
    <Card
      title="用户管理"
      extra={
        <Space>
          <Input.Search placeholder="搜索用户名/邮箱" allowClear onSearch={(v) => { setQ(v); setPage(1); }} style={{ width: 220 }} />
          <Select
            placeholder="状态"
            allowClear
            style={{ width: 120 }}
            value={status || undefined}
            onChange={(v) => { setStatus(v ?? ""); setPage(1); }}
            options={[
              { value: "active", label: "正常" },
              { value: "pending", label: "待验证" },
              { value: "suspended", label: "已停用" },
              { value: "banned", label: "已封禁" },
            ]}
          />
        </Space>
      }
    >
      <Table<UserRow>
        rowKey="id"
        loading={list.isLoading}
        dataSource={list.data?.items ?? []}
        pagination={{
          current: page, pageSize: 20, total: list.data?.total ?? 0,
          onChange: setPage, showTotal: (t) => `共 ${t} 位用户`,
        }}
        onRow={(r) => ({ onClick: () => setDetail(r), style: { cursor: "pointer" } })}
        columns={[
          { title: "ID", dataIndex: "id", width: 70 },
          { title: "用户名", dataIndex: "username" },
          { title: "邮箱", dataIndex: "email", ellipsis: true },
          {
            title: "状态", dataIndex: "status", width: 100,
            render: (s: string) => <Tag color={statusColors[s]}>{s}</Tag>,
          },
          {
            title: "邮箱验证", dataIndex: "email_verified", width: 100,
            render: (v: boolean) => (v ? <Tag color="green">已验证</Tag> : <Tag>未验证</Tag>),
          },
          { title: "帖子数", dataIndex: "post_count", width: 90 },
          {
            title: "注册时间", dataIndex: "created_at", width: 120,
            render: (v: string) => timeAgo(v),
          },
        ]}
      />

      <Drawer
        open={!!detail}
        onClose={() => setDetail(null)}
        width={560}
        title={detail ? `用户：${detail.username}` : ""}
        extra={
          <Space>
            <Button size="small" onClick={() => setRoleOpen(true)}>授予角色</Button>
            <Button size="small" danger onClick={() => setSanctionOpen(true)}>创建处罚</Button>
            {detail?.status === "banned" && (
              <Popconfirm title="确认解封该用户？" onConfirm={unban}>
                <Button size="small">解封</Button>
              </Popconfirm>
            )}
          </Space>
        }
      >
        {detailData.data && (
          <Tabs
            items={[
              {
                key: "info",
                label: "资料",
                children: (
                  <Descriptions column={1} size="small" bordered>
                    <Descriptions.Item label="公开 ID">{detailData.data.user.public_id}</Descriptions.Item>
                    <Descriptions.Item label="邮箱">{detailData.data.user.email}</Descriptions.Item>
                    <Descriptions.Item label="昵称">{detailData.data.user.display_name}</Descriptions.Item>
                    <Descriptions.Item label="简介">{detailData.data.bio || "—"}</Descriptions.Item>
                    <Descriptions.Item label="所在地">{detailData.data.location || "—"}</Descriptions.Item>
                    <Descriptions.Item label="注册时间">{new Date(detailData.data.user.created_at).toLocaleString("zh-CN")}</Descriptions.Item>
                    <Descriptions.Item label="最近登录">{detailData.data.user.last_login_at ? timeAgo(detailData.data.user.last_login_at) : "—"}</Descriptions.Item>
                  </Descriptions>
                ),
              },
              {
                key: "roles",
                label: `角色 (${detailData.data.roles.length})`,
                children: (
                  <Space direction="vertical" style={{ width: "100%" }}>
                    {detailData.data.roles.map((r) => (
                      <div key={r.id} style={{ display: "flex", alignItems: "center", gap: 8 }}>
                        <Tag color="blue">{r.role_name}</Tag>
                        <span style={{ color: "#999", fontSize: 13 }}>
                          {r.scope_type === "category" ? `范围：${r.scope_name}` : "全局"}
                        </span>
                        <Popconfirm title="移除该角色？" onConfirm={() => removeRole(r.id)}>
                          <Button size="small" type="text" danger>移除</Button>
                        </Popconfirm>
                      </div>
                    ))}
                    {detailData.data.roles.length === 0 && <span style={{ color: "#999" }}>暂无角色</span>}
                  </Space>
                ),
              },
              {
                key: "sanctions",
                label: `处罚记录 (${detailData.data.sanctions.length})`,
                children: (
                  <Space direction="vertical" style={{ width: "100%" }}>
                    {detailData.data.sanctions.map((s) => (
                      <div key={s.id} style={{ display: "flex", alignItems: "center", gap: 8, borderBottom: "1px solid #f0f0f0", paddingBottom: 8 }}>
                        <Tag color={sanctionColors[s.sanction_type]}>{s.sanction_type}</Tag>
                        <span style={{ fontSize: 13, flex: 1 }}>{s.reason}</span>
                        <span style={{ fontSize: 12, color: "#999" }}>{timeAgo(s.created_at)}</span>
                        {s.status === "active" && (
                          <Popconfirm title="解除该处罚？" onConfirm={() => liftSanction(s.id)}>
                            <Button size="small">解除</Button>
                          </Popconfirm>
                        )}
                        <Tag>{s.status}</Tag>
                      </div>
                    ))}
                    {detailData.data.sanctions.length === 0 && <span style={{ color: "#999" }}>无处罚记录</span>}
                  </Space>
                ),
              },
            ]}
          />
        )}
      </Drawer>

      <Modal title="创建处罚" open={sanctionOpen} onCancel={() => setSanctionOpen(false)} onOk={() => form.submit()} okText="确认处罚">
        <Form form={form} layout="vertical" onFinish={createSanction}>
          <Form.Item name="sanction_type" label="处罚类型" rules={[{ required: true }]} initialValue="warn">
            <AntSelect
              options={[
                { value: "warn", label: "警告" },
                { value: "mute", label: "禁言" },
                { value: "post_ban", label: "禁止发帖" },
                { value: "temp_ban", label: "临时封禁" },
                { value: "perma_ban", label: "永久封禁" },
              ]}
            />
          </Form.Item>
          <Form.Item noStyle shouldUpdate={(p, c) => p.sanction_type !== c.sanction_type}>
            {({ getFieldValue }) =>
              getFieldValue("sanction_type") === "temp_ban" ? (
                <Form.Item name="days" label="封禁天数" rules={[{ required: true, message: "请输入天数" }]}>
                  <Input type="number" min={1} placeholder="如 7" />
                </Form.Item>
              ) : null
            }
          </Form.Item>
          <Form.Item name="reason" label="处罚原因（将通知用户并写入审计）" rules={[{ required: true, message: "必须填写原因" }]}>
            <Input.TextArea rows={3} maxLength={500} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal title="授予角色" open={roleOpen} onCancel={() => setRoleOpen(false)} onOk={() => roleForm.submit()} okText="授予">
        <Form form={roleForm} layout="vertical" onFinish={assignRole}>
          <Form.Item name="role_id" label="角色" rules={[{ required: true, message: "请选择角色" }]}>
            <AntSelect
              placeholder="选择角色"
              options={(roles.data?.items ?? []).map((r) => ({
                value: r.id, label: `${r.name}（${r.code}）`,
                disabled: ["admin", "superadmin"].includes(r.code) && !r.is_system === false && r.code === "superadmin",
              }))}
            />
          </Form.Item>
          <Form.Item noStyle shouldUpdate={(p, c) => p.role_id !== c.role_id}>
            {({ getFieldValue }) => {
              const role = (roles.data?.items ?? []).find((r) => r.id === getFieldValue("role_id"));
              return role?.code === "moderator" ? (
                <Form.Item name="scope" label="授权范围" rules={[{ required: true, message: "版主必须选择授权版块" }]}>
                  <CategorySelect />
                </Form.Item>
              ) : (
                <Form.Item name="scope" hidden initialValue="global">
                  <Input />
                </Form.Item>
              );
            }}
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  );
}

function CategorySelect({ value, onChange }: { value?: number; onChange?: (v: number) => void }) {
  const { data } = useQuery({
    queryKey: ["admin-categories"],
    queryFn: () => api.get<{ items: { id: number; name: string }[] }>("/admin/categories"),
  });
  return (
    <AntSelect
      placeholder="选择版块（版主范围）"
      value={value}
      onChange={onChange}
      options={(data?.items ?? []).map((c) => ({ value: c.id, label: c.name }))}
    />
  );
}
