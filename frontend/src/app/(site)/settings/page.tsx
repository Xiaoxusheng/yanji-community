"use client";

// 账号设置：个人资料、隐私与通知、修改密码、会话设备管理。
import Link from "next/link";
import { useEffect, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, timeAgo } from "@/lib/api";
import type { SessionRow } from "@/lib/types";
import { Avatar } from "@/components/ui/Avatar";
import { ConfirmDialog } from "@/components/ui/Dialog";
import { ListSkeleton, ErrorState, PageHeader } from "@/components/ui/States";
import { AuthShell, Field, validatePasswordClient } from "@/components/auth/forms";
import { CaptchaField } from "@/components/auth/CaptchaField";
import { useMe } from "@/lib/auth";
import { toast } from "@/lib/toast";
import { IconLock, IconSettings, IconShield, IconUser } from "@/components/ui/Icons";

type Tab = "profile" | "privacy" | "password" | "sessions";

export default function SettingsPage() {
  const { data: me, isLoading } = useMe();
  const [tab, setTab] = useState<Tab>("profile");

  if (isLoading) return null;
  if (me && !me.authenticated) {
    return (
      <AuthShell title="请先登录">
        <p style={{ marginBottom: 16, color: "var(--text-2)" }}>登录后管理账号与个人资料。</p>
        <Link href="/login" className="btn btn-primary">去登录</Link>
      </AuthShell>
    );
  }
  const user = me?.user;
  const unverified = user && !user.email_verified;

  return (
    <div style={{ maxWidth: 740, margin: "0 auto" }}>
      <PageHeader title="账号设置" subtitle="资料展示、消息隐私、安全与登录设备管理" />
      {unverified && (
        <div className="card" style={{ padding: 14, marginBottom: 14, background: "var(--warning-soft)", borderColor: "transparent", display: "flex", alignItems: "center", gap: 10, flexWrap: "wrap" }}>
          <span style={{ color: "var(--warning)", fontWeight: 600, fontSize: 13.5 }}>邮箱尚未验证，验证后方可发帖和评论</span>
          <ResendButton />
        </div>
      )}

      <div style={{ marginBottom: 14 }}>
        <div className="seg-tabs" role="tablist" style={{ overflowX: "auto", maxWidth: "100%" }}>
          {([
            ["profile", "个人资料", IconUser],
            ["privacy", "隐私与通知", IconSettings],
            ["password", "修改密码", IconLock],
            ["sessions", "会话设备", IconShield],
          ] as const).map(([k, label, Icon]) => (
            <button
              key={k}
              role="tab"
              aria-selected={tab === k}
              className="seg-tab"
              onClick={() => setTab(k)}
            >
              <Icon size={13} /> {label}
            </button>
          ))}
        </div>
      </div>

      {tab === "profile" && <ProfileTab />}
      {tab === "privacy" && <PrivacyTab />}
      {tab === "password" && <PasswordTab />}
      {tab === "sessions" && <SessionsTab />}
    </div>
  );
}

function ResendButton() {
  const [sending, setSending] = useState(false);
  return (
    <button
      className="btn btn-secondary btn-sm"
      style={{ marginLeft: "auto" }}
      disabled={sending}
      onClick={async () => {
        setSending(true);
        try {
          await api.post("/auth/email/resend");
          toast("success", "验证邮件已发送，请查收");
        } catch (e) {
          toast("error", (e as Error).message);
        } finally {
          setSending(false);
        }
      }}
    >
      {sending ? "发送中…" : "重发验证邮件"}
    </button>
  );
}

function ProfileTab() {
  const qc = useQueryClient();
  const { data: me } = useMe();
  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ["me-profile"],
    queryFn: () =>
      api.get<{ profile: { display_name: string; bio: string; website: string; location: string; avatar_key: string } }>("/me/profile"),
  });
  const [displayName, setDisplayName] = useState("");
  const [avatarKey, setAvatarKey] = useState("");
  const [bio, setBio] = useState("");
  const [website, setWebsite] = useState("");
  const [location, setLocation] = useState("");
  const [uploadingAvatar, setUploadingAvatar] = useState(false);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (data) {
      setDisplayName(data.profile.display_name);
      setAvatarKey(data.profile.avatar_key ?? "");
      setBio(data.profile.bio);
      setWebsite(data.profile.website);
      setLocation(data.profile.location);
    }
  }, [data]);

  if (isLoading) return <ListSkeleton count={1} />;
  if (error) return <ErrorState message={(error as Error).message} onRetry={() => refetch()} />;

  const uploadAvatar = async (file: File) => {
    const fd = new FormData();
    fd.append("file", file);
    setUploadingAvatar(true);
    try {
      const res = await api.upload<{ key: string; url: string }>("/uploads", fd);
      setAvatarKey(res.key);
      await api.patch("/me/profile", { avatar_key: res.key });
      toast("success", "头像已更新");
      qc.invalidateQueries({ queryKey: ["me-profile"] });
      if (me?.user?.username) {
        qc.invalidateQueries({ queryKey: ["profile", me.user.username] });
      }
    } catch (e) {
      toast("error", (e as Error).message);
    } finally {
      setUploadingAvatar(false);
    }
  };

  const save = async () => {
    setSaving(true);
    try {
      await api.patch("/me/profile", {
        display_name: displayName,
        avatar_key: avatarKey,
        bio,
        website,
        location,
      });
      toast("success", "资料已更新");
      qc.invalidateQueries({ queryKey: ["me-profile"] });
      if (me?.user?.username) {
        qc.invalidateQueries({ queryKey: ["profile", me.user.username] });
      }
    } catch (e) {
      toast("error", (e as Error).message);
    } finally {
      setSaving(false);
    }
  };

  return (
    <section className="card" style={{ padding: 22 }}>
      <div style={{ display: "flex", alignItems: "center", gap: 14, marginBottom: 18 }}>
        <Avatar name={displayName || me?.user?.username || "?"} avatarKey={avatarKey} size={56} />
        <div>
          <label className="btn btn-secondary btn-sm" style={{ cursor: uploadingAvatar ? "wait" : "pointer" }}>
            {uploadingAvatar ? "上传中…" : "更换头像"}
            <input
              type="file"
              accept="image/jpeg,image/png,image/gif,image/webp"
              hidden
              disabled={uploadingAvatar}
              onChange={(e) => {
                const f = e.target.files?.[0];
                if (f) uploadAvatar(f);
                e.target.value = "";
              }}
            />
          </label>
          <div style={{ fontSize: 12, color: "var(--text-3)", marginTop: 4 }}>
            支持 jpg / png / gif / webp 格式图片
          </div>
        </div>
      </div>

      <div style={{ marginBottom: 14 }}>
        <label style={{ display: "block", fontSize: 13.5, fontWeight: 600, marginBottom: 6 }}>昵称</label>
        <input className="input" value={displayName} maxLength={30} onChange={(e) => setDisplayName(e.target.value)} />
      </div>
      <div style={{ marginBottom: 14 }}>
        <label style={{ display: "block", fontSize: 13.5, fontWeight: 600, marginBottom: 6 }}>个人简介</label>
        <textarea className="textarea" value={bio} maxLength={500} rows={3} onChange={(e) => setBio(e.target.value)} />
      </div>
      <div style={{ marginBottom: 16, display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(200px, 1fr))", gap: 12 }}>
        <div>
          <label style={{ display: "block", fontSize: 13.5, fontWeight: 600, marginBottom: 6 }}>个人主页</label>
          <input className="input" value={website} placeholder="https://" onChange={(e) => setWebsite(e.target.value)} />
        </div>
        <div>
          <label style={{ display: "block", fontSize: 13.5, fontWeight: 600, marginBottom: 6 }}>所在地</label>
          <input className="input" value={location} maxLength={50} onChange={(e) => setLocation(e.target.value)} />
        </div>
      </div>
      <button className="btn btn-primary" onClick={save} disabled={saving}>
        {saving ? "保存中…" : "保存资料"}
      </button>
    </section>
  );
}

function PrivacyTab() {
  const qc = useQueryClient();
  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ["me-profile"],
    queryFn: () =>
      api.get<{ profile: { profile_visibility: string; show_followers: boolean; notify_email: boolean; notify_inapp: boolean; allow_direct_messages?: boolean } }>("/me/profile"),
  });
  const [visibility, setVisibility] = useState("public");
  const [showFollowers, setShowFollowers] = useState(true);
  const [notifyEmail, setNotifyEmail] = useState(true);
  const [notifyInapp, setNotifyInapp] = useState(true);
  const [allowDm, setAllowDm] = useState(true);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (data) {
      setVisibility(data.profile.profile_visibility);
      setShowFollowers(data.profile.show_followers);
      setNotifyEmail(data.profile.notify_email);
      setNotifyInapp(data.profile.notify_inapp);
      setAllowDm(data.profile.allow_direct_messages !== false);
    }
  }, [data]);

  if (isLoading) return <ListSkeleton count={1} />;
  if (error) return <ErrorState message={(error as Error).message} onRetry={() => refetch()} />;

  const save = async () => {
    setSaving(true);
    try {
      await api.patch("/me/settings", {
        profile_visibility: visibility,
        show_followers: showFollowers,
        notify_email: notifyEmail,
        notify_inapp: notifyInapp,
        allow_direct_messages: allowDm,
      });
      toast("success", "设置已保存");
      qc.invalidateQueries({ queryKey: ["me-profile"] });
    } catch (e) {
      toast("error", (e as Error).message);
    } finally {
      setSaving(false);
    }
  };

  const checks: [string, boolean, (v: boolean) => void, string][] = [
    ["允许他人向我发起私信", allowDm, setAllowDm, "关闭后其他人无法新建与你的私信会话（已有会话不受影响）"],
    ["公开关注与粉丝列表", showFollowers, setShowFollowers, "关闭后他人访问你的主页时无法查看关注和粉丝列表"],
    ["邮件通知", notifyEmail, setNotifyEmail, "重要系统邮件（验证、安全提醒）不受影响"],
    ["站内通知推送", notifyInapp, setNotifyInapp, "关闭后社交类实时通知将减少打扰"],
  ];

  return (
    <section className="card" style={{ padding: 22 }}>
      <div style={{ marginBottom: 16 }}>
        <label style={{ display: "block", fontSize: 13.5, fontWeight: 600, marginBottom: 6 }}>主页可见范围</label>
        <select className="select" style={{ maxWidth: 260 }} value={visibility} onChange={(e) => setVisibility(e.target.value)}>
          <option value="public">所有人可见</option>
          <option value="followers">仅关注我的人可见</option>
          <option value="private">仅自己可见</option>
        </select>
      </div>
      {checks.map(([label, val, set, hint]) => (
        <label key={label} style={{ display: "flex", gap: 10, alignItems: "flex-start", marginBottom: 14, cursor: "pointer" }}>
          <input type="checkbox" checked={val} onChange={(e) => set(e.target.checked)} style={{ marginTop: 3 }} />
          <span>
            <span style={{ fontSize: 14, fontWeight: 600 }}>{label}</span>
            <div style={{ fontSize: 12.5, color: "var(--text-3)" }}>{hint}</div>
          </span>
        </label>
      ))}
      <button className="btn btn-primary" onClick={save} disabled={saving}>
        {saving ? "保存中…" : "保存设置"}
      </button>
    </section>
  );
}

function PasswordTab() {
  const qc = useQueryClient();
  const { data: me } = useMe();
  const email = me?.user?.email ?? "";

  const [oldPassword, setOldPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [pwdErr, setPwdErr] = useState<{ old?: string; next?: string; confirm?: string }>({});
  const [changing, setChanging] = useState(false);

  const [captchaId, setCaptchaId] = useState("");
  const [captchaCode, setCaptchaCode] = useState("");
  const [refreshKey, setRefreshKey] = useState(0);
  const [sendingReset, setSendingReset] = useState(false);

  const submitChange = async (e: React.FormEvent) => {
    e.preventDefault();
    const nextErr: typeof pwdErr = {};
    if (!oldPassword) nextErr.old = "请输入当前密码";
    const strengthErr = validatePasswordClient(newPassword);
    if (strengthErr) nextErr.next = strengthErr;
    else if (newPassword === oldPassword) nextErr.next = "新密码不能与当前密码相同";
    if (newPassword !== confirm) nextErr.confirm = "两次输入的新密码不一致";
    setPwdErr(nextErr);
    if (Object.keys(nextErr).length) return;
    setChanging(true);
    try {
      await api.post("/auth/password/change", {
        old_password: oldPassword,
        new_password: newPassword,
      });
      toast("success", "密码修改成功，其他设备已自动下线");
      setOldPassword("");
      setNewPassword("");
      setConfirm("");
      qc.invalidateQueries({ queryKey: ["sessions"] });
    } catch (e2) {
      toast("error", (e2 as Error).message);
    } finally {
      setChanging(false);
    }
  };

  const sendResetMail = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!email) {
      toast("error", "未获取到当前账号邮箱");
      return;
    }
    if (!captchaCode.trim()) {
      toast("error", "请输入图形验证码");
      return;
    }
    setSendingReset(true);
    try {
      await api.post("/auth/password/forgot", {
        email,
        captcha_id: captchaId,
        captcha_code: captchaCode.trim(),
      });
      toast("success", "如邮箱已绑定，重置邮件已发出");
      setRefreshKey((k) => k + 1);
    } catch (e2) {
      toast("error", (e2 as Error).message);
      setRefreshKey((k) => k + 1);
    } finally {
      setSendingReset(false);
    }
  };

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 14 }}>
      <section className="card" style={{ padding: 22 }}>
        <h3 style={{ fontWeight: 650, fontSize: 15.5, marginBottom: 6 }}>修改登录密码</h3>
        <p style={{ fontSize: 13.5, color: "var(--text-2)", lineHeight: 1.65, marginBottom: 16 }}>
          为保障账号安全，修改密码后将自动刷新当前会话并下线全部其他登录设备。
        </p>
        <form onSubmit={submitChange} noValidate style={{ maxWidth: 420 }}>
          <Field
            label="当前密码"
            type="password"
            value={oldPassword}
            onChange={setOldPassword}
            error={pwdErr.old}
            autoComplete="current-password"
          />
          <Field
            label="新密码"
            type="password"
            value={newPassword}
            onChange={setNewPassword}
            error={pwdErr.next}
            placeholder="8-72 位，至少包含字母、数字或符号中的两种"
            autoComplete="new-password"
          />
          <Field
            label="确认新密码"
            type="password"
            value={confirm}
            onChange={setConfirm}
            error={pwdErr.confirm}
            autoComplete="new-password"
          />
          <button className="btn btn-primary" disabled={changing}>
            {changing ? "保存中…" : "更新密码"}
          </button>
        </form>
      </section>

      <section className="card" style={{ padding: 22 }}>
        <h3 style={{ fontWeight: 650, fontSize: 15.5, marginBottom: 6 }}>通过邮箱重置密码</h3>
        <p style={{ fontSize: 13.5, color: "var(--text-2)", lineHeight: 1.65, marginBottom: 14 }}>
          若忘记当前密码，可向绑定邮箱（{email || "未读取到"}）发送一次性重置链接（30 分钟有效）。
        </p>
        <form onSubmit={sendResetMail} style={{ maxWidth: 420 }}>
          <CaptchaField
            captchaId={captchaId}
            code={captchaCode}
            onCaptchaChange={setCaptchaId}
            onCodeChange={setCaptchaCode}
            refreshKey={refreshKey}
          />
          <button className="btn btn-secondary" disabled={sendingReset || !email}>
            {sendingReset ? "发送中…" : "发送密码重置邮件"}
          </button>
        </form>
        <div style={{ marginTop: 12, fontSize: 12.5, color: "var(--text-3)" }}>
          如邮箱无法收信，请前往 <Link href="/help" style={{ color: "var(--brand)" }}>帮助页</Link> 查看说明。
        </div>
      </section>
    </div>
  );
}

function SessionsTab() {
  const qc = useQueryClient();
  const [confirmLogoutAll, setConfirmLogoutAll] = useState(false);
  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ["sessions"],
    queryFn: () => api.get<{ items: SessionRow[] }>("/auth/sessions"),
  });

  const revoke = async (id: number) => {
    try {
      await api.delete(`/auth/sessions/${id}`);
      toast("success", "会话已撤销");
      qc.invalidateQueries({ queryKey: ["sessions"] });
    } catch (e) {
      toast("error", (e as Error).message);
    }
  };

  const revokeAll = async () => {
    try {
      await api.post("/auth/logout-all");
      toast("success", "已退出全部设备");
      window.location.href = "/login";
    } catch (e) {
      toast("error", (e as Error).message);
    }
  };

  if (isLoading) return <ListSkeleton count={2} />;
  if (error) return <ErrorState message={(error as Error).message} onRetry={() => refetch()} />;

  return (
    <section className="card" style={{ padding: 22 }}>
      <div style={{ display: "flex", alignItems: "center", marginBottom: 14 }}>
        <h3 style={{ fontWeight: 650, fontSize: 15.5 }}>已登录设备</h3>
        <button className="btn btn-danger btn-sm" style={{ marginLeft: "auto" }} onClick={() => setConfirmLogoutAll(true)}>
          退出全部设备
        </button>
      </div>
      <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
        {(data?.items ?? []).map((s) => (
          <div key={s.id} className="card" style={{ padding: 12, display: "flex", alignItems: "center", gap: 12, boxShadow: "none" }}>
            <div
              style={{
                width: 36,
                height: 36,
                borderRadius: 8,
                background: "var(--bg-inset)",
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                color: "var(--text-2)",
              }}
              aria-hidden
            >
              <IconShield size={16} />
            </div>
            <div style={{ flex: 1, minWidth: 0 }}>
              <div style={{ fontSize: 14, fontWeight: 600, display: "flex", alignItems: "center", gap: 8 }}>
                <span>{s.device_name || "Web 会话"}</span>
                {s.current && <span className="tag tag-success">当前设备</span>}
              </div>
              <div className="tabular-nums" style={{ fontSize: 12.5, color: "var(--text-3)", marginTop: 2 }}>
                {s.ip_address || "未知 IP"} · 最近活跃 {timeAgo(s.last_seen_at)} · 过期于 {new Date(s.expires_at).toLocaleDateString("zh-CN")}
              </div>
            </div>
            {!s.current && (
              <button className="btn btn-secondary btn-xs" onClick={() => revoke(s.id)}>
                下线
              </button>
            )}
          </div>
        ))}
      </div>

      <ConfirmDialog
        open={confirmLogoutAll}
        title="退出全部设备"
        description="确认下线所有设备（包括当前浏览器）吗？下线后需重新登录。"
        confirmText="确认退出"
        danger
        onConfirm={revokeAll}
        onClose={() => setConfirmLogoutAll(false)}
      />
    </section>
  );
}
