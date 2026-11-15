"use client";

// 账号设置：资料 / 隐私与通知 / 密码 / 会话设备。
import Link from "next/link";
import { useEffect, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { api, timeAgo } from "@/lib/api";
import type { SessionRow } from "@/lib/types";
import { ListSkeleton, ErrorState } from "@/components/ui/States";
import { AuthShell } from "@/components/auth/forms";
import { useMe } from "@/lib/auth";
import { toast } from "@/lib/toast";

type Tab = "profile" | "privacy" | "password" | "sessions";

export default function SettingsPage() {
  const { data: me, isLoading } = useMe();
  const [tab, setTab] = useState<Tab>("profile");

  if (isLoading) return null;
  if (me && !me.authenticated) {
    return (
      <AuthShell title="请先登录">
        <p style={{ marginBottom: 16 }}>登录后管理账号设置。</p>
        <Link href="/login" className="btn btn-primary">去登录</Link>
      </AuthShell>
    );
  }
  const user = me?.user;
  const unverified = user && !user.email_verified;

  return (
    <div style={{ maxWidth: 720, margin: "0 auto" }}>
      <h1 style={{ fontSize: 20, fontWeight: 700, marginBottom: 14 }}>账号设置</h1>
      {unverified && (
        <div className="card" style={{ padding: 14, marginBottom: 14, background: "var(--warning-soft)", border: "none", display: "flex", alignItems: "center", gap: 10 }}>
          <span style={{ color: "var(--warning)", fontWeight: 600, fontSize: 14 }}>邮箱未验证：暂时无法发帖和评论。</span>
          <ResendButton />
        </div>
      )}
      <div className="card" style={{ padding: "4px 8px", marginBottom: 14, display: "flex", gap: 2, flexWrap: "wrap" }} role="tablist">
        {([["profile", "个人资料"], ["privacy", "隐私与通知"], ["password", "修改密码"], ["sessions", "会话设备"]] as const).map(([k, label]) => (
          <button
            key={k}
            role="tab"
            aria-selected={tab === k}
            className="btn btn-text"
            onClick={() => setTab(k)}
            style={{
              fontWeight: tab === k ? 700 : 400,
              color: tab === k ? "var(--brand)" : "var(--text-2)",
              borderBottom: tab === k ? "2px solid var(--brand)" : "2px solid transparent",
              borderRadius: 0,
            }}
          >
            {label}
          </button>
        ))}
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
  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ["me-profile"],
    queryFn: () => api.get<{ profile: { display_name: string; bio: string; website: string; location: string; avatar_key: string } }>("/me/profile"),
  });
  const [displayName, setDisplayName] = useState("");
  const [bio, setBio] = useState("");
  const [website, setWebsite] = useState("");
  const [location, setLocation] = useState("");
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (data) {
      setDisplayName(data.profile.display_name);
      setBio(data.profile.bio);
      setWebsite(data.profile.website);
      setLocation(data.profile.location);
    }
  }, [data]);

  if (isLoading) return <ListSkeleton count={1} />;
  if (error) return <ErrorState message={(error as Error).message} onRetry={() => refetch()} />;

  const save = async () => {
    setSaving(true);
    try {
      await api.patch("/me/profile", { display_name: displayName, bio, website, location });
      toast("success", "资料已更新");
      qc.invalidateQueries({ queryKey: ["me-profile"] });
    } catch (e) {
      toast("error", (e as Error).message);
    } finally {
      setSaving(false);
    }
  };

  return (
    <section className="card" style={{ padding: 20 }}>
      <div style={{ marginBottom: 14 }}>
        <label style={{ display: "block", fontSize: 13.5, fontWeight: 600, marginBottom: 6 }}>昵称</label>
        <input className="input" value={displayName} maxLength={30} onChange={(e) => setDisplayName(e.target.value)} />
      </div>
      <div style={{ marginBottom: 14 }}>
        <label style={{ display: "block", fontSize: 13.5, fontWeight: 600, marginBottom: 6 }}>简介</label>
        <textarea className="textarea" value={bio} maxLength={500} rows={3} onChange={(e) => setBio(e.target.value)} />
      </div>
      <div style={{ marginBottom: 14, display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
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
      api.get<{ profile: { profile_visibility: string; show_followers: boolean; notify_email: boolean; notify_inapp: boolean } }>("/me/profile"),
  });
  const [visibility, setVisibility] = useState("public");
  const [showFollowers, setShowFollowers] = useState(true);
  const [notifyEmail, setNotifyEmail] = useState(true);
  const [notifyInapp, setNotifyInapp] = useState(true);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (data) {
      setVisibility(data.profile.profile_visibility);
      setShowFollowers(data.profile.show_followers);
      setNotifyEmail(data.profile.notify_email);
      setNotifyInapp(data.profile.notify_inapp);
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
    ["公开关注/粉丝列表", showFollowers, setShowFollowers, "关闭后其他人无法查看你的关注与粉丝"],
    ["邮件通知", notifyEmail, setNotifyEmail, "重要系统邮件（验证、安全提醒）不受此开关影响"],
    ["站内通知", notifyInapp, setNotifyInapp, "关闭后仍会在通知中心保留记录"],
  ];
  return (
    <section className="card" style={{ padding: 20 }}>
      <div style={{ marginBottom: 14 }}>
        <label style={{ display: "block", fontSize: 13.5, fontWeight: 600, marginBottom: 6 }}>主页可见性</label>
        <select className="select" style={{ maxWidth: 260 }} value={visibility} onChange={(e) => setVisibility(e.target.value)}>
          <option value="public">公开</option>
          <option value="followers">仅关注者</option>
          <option value="private">仅自己</option>
        </select>
      </div>
      {checks.map(([label, val, set, hint]) => (
        <label key={label} style={{ display: "flex", gap: 10, alignItems: "flex-start", marginBottom: 12, cursor: "pointer" }}>
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
  const [saving, setSaving] = useState(false);

  return (
    <section className="card" style={{ padding: 20 }}>
      <h3 style={{ fontWeight: 700, marginBottom: 10 }}>修改密码</h3>
      <p style={{ fontSize: 13.5, color: "var(--text-2)", marginBottom: 14 }}>
        为保障账号安全，密码修改通过邮箱验证完成：点击下方按钮，我们将向你的注册邮箱发送密码重置邮件（30 分钟内有效）。
        重置成功后所有设备将被强制退出。
      </p>
      <button
        className="btn btn-primary"
        disabled={saving}
        onClick={async () => {
          setSaving(true);
          try {
            await api.post("/auth/password/forgot", {});
            toast("success", "如邮箱已绑定，将收到重置邮件");
          } catch (e) {
            toast("error", (e as Error).message);
          } finally {
            setSaving(false);
          }
        }}
      >
        发送密码重置邮件
      </button>
      <div style={{ marginTop: 16, fontSize: 12.5, color: "var(--text-3)" }}>
        忘记邮箱或无法收信？请通过 <Link href="/help" style={{ color: "var(--brand)" }}>帮助页</Link> 联系管理员。
      </div>
    </section>
  );
}

function SessionsTab() {
  const qc = useQueryClient();
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
    if (!window.confirm("将退出全部设备（包括当前设备），确定？")) return;
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
    <section className="card" style={{ padding: 20 }}>
      <div style={{ display: "flex", alignItems: "center", marginBottom: 12 }}>
        <h3 style={{ fontWeight: 700 }}>会话设备</h3>
        <button className="btn btn-danger btn-sm" style={{ marginLeft: "auto" }} onClick={revokeAll}>
          退出全部设备
        </button>
      </div>
      <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
        {(data?.items ?? []).map((s) => (
          <div key={s.id} className="card" style={{ padding: 12, display: "flex", alignItems: "center", gap: 10, boxShadow: "none" }}>
            <span style={{ fontSize: 20 }} aria-hidden>💻</span>
            <div style={{ flex: 1 }}>
              <div style={{ fontSize: 14, fontWeight: 600 }}>
                {s.device_name} {s.current && <span className="tag tag-success">当前设备</span>}
              </div>
              <div style={{ fontSize: 12.5, color: "var(--text-3)" }}>
                {s.ip_address || "未知 IP"} · 最近活跃 {timeAgo(s.last_seen_at)} · 过期于 {new Date(s.expires_at).toLocaleDateString("zh-CN")}
              </div>
            </div>
            {!s.current && (
              <button className="btn btn-secondary btn-sm" onClick={() => revoke(s.id)}>
                撤销
              </button>
            )}
          </div>
        ))}
      </div>
    </section>
  );
}
