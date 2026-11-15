"use client";

// 登录 / 注册 / 验证邮箱 / 找回密码 / 重置密码（共享 auth 布局）。
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, useState } from "react";
import { api } from "@/lib/api";
import { toast } from "@/lib/toast";
import { useInvalidateMe } from "@/lib/auth";

export function AuthShell({ title, subtitle, children }: { title: string; subtitle?: string; children: React.ReactNode }) {
  return (
    <div style={{ maxWidth: 420, margin: "48px auto" }}>
      <div className="card" style={{ padding: 28 }}>
        <h1 style={{ fontSize: 22, fontWeight: 700, marginBottom: 4 }}>{title}</h1>
        {subtitle && (
          <p style={{ color: "var(--text-3)", fontSize: 14, marginBottom: 20 }}>{subtitle}</p>
        )}
        {children}
      </div>
    </div>
  );
}

export function Field({
  label,
  value,
  onChange,
  type = "text",
  placeholder,
  error,
  autoComplete,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  type?: string;
  placeholder?: string;
  error?: string;
  autoComplete?: string;
}) {
  return (
    <div style={{ marginBottom: 14 }}>
      <label style={{ display: "block", fontSize: 13.5, marginBottom: 6, fontWeight: 600 }}>{label}</label>
      <input
        className="input"
        type={type}
        value={value}
        placeholder={placeholder}
        autoComplete={autoComplete}
        aria-invalid={!!error}
        onChange={(e) => onChange(e.target.value)}
      />
      {error && <div style={{ color: "var(--danger)", fontSize: 12.5, marginTop: 4 }}>{error}</div>}
    </div>
  );
}

function LoginPageInner() {
  const router = useRouter();
  const invalidate = useInvalidateMe();
  const [account, setAccount] = useState("");
  const [password, setPassword] = useState("");
  const [err, setErr] = useState<{ account?: string; password?: string }>({});
  const [submitting, setSubmitting] = useState(false);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    const next: typeof err = {};
    if (!account.trim()) next.account = "请输入用户名或邮箱";
    if (!password) next.password = "请输入密码";
    setErr(next);
    if (Object.keys(next).length) return;
    setSubmitting(true);
    try {
      await api.post("/auth/login", { account, password });
      invalidate();
      toast("success", "登录成功");
      router.push("/");
    } catch (e2) {
      toast("error", (e2 as Error).message);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <AuthShell title="登录" subtitle="欢迎回来">
      <form onSubmit={submit} noValidate>
        <Field label="用户名 / 邮箱" value={account} onChange={setAccount} error={err.account} autoComplete="username" />
        <Field label="密码" value={password} onChange={setPassword} type="password" error={err.password} autoComplete="current-password" />
        <button className="btn btn-primary" style={{ width: "100%", marginTop: 6 }} disabled={submitting}>
          {submitting ? "登录中…" : "登录"}
        </button>
      </form>
      <div style={{ display: "flex", justifyContent: "space-between", marginTop: 16, fontSize: 13.5 }}>
        <Link href="/forgot-password" style={{ color: "var(--brand)" }}>
          忘记密码？
        </Link>
        <Link href="/register" style={{ color: "var(--brand)" }}>
          注册新账号 →
        </Link>
      </div>
    </AuthShell>
  );
}

export function AuthLoginPage() {
  return (
    <Suspense>
      <LoginPageInner />
    </Suspense>
  );
}

function RegisterPageInner() {
  const router = useRouter();
  const [username, setUsername] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [err, setErr] = useState<{ username?: string; email?: string; password?: string }>({});
  const [submitting, setSubmitting] = useState(false);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    const next: typeof err = {};
    if (!/^[a-zA-Z0-9_]{3,20}$/.test(username)) next.username = "3-20 位字母、数字或下划线";
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) next.email = "邮箱格式不正确";
    if (password.length < 8) next.password = "密码至少 8 位";
    setErr(next);
    if (Object.keys(next).length) return;
    setSubmitting(true);
    try {
      await api.post("/auth/register", { username, email, password });
      toast("success", "注册成功，请查收验证邮件");
      router.push(`/verify-email?email=${encodeURIComponent(email)}`);
    } catch (e2) {
      toast("error", (e2 as Error).message);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <AuthShell title="注册" subtitle="公开注册，加入社区">
      <form onSubmit={submit} noValidate>
        <Field label="用户名" value={username} onChange={setUsername} error={err.username} placeholder="3-20 位字母、数字或下划线" autoComplete="username" />
        <Field label="邮箱" value={email} onChange={setEmail} type="email" error={err.email} autoComplete="email" />
        <Field label="密码" value={password} onChange={setPassword} type="password" error={err.password} placeholder="至少 8 位" autoComplete="new-password" />
        <button className="btn btn-primary" style={{ width: "100%", marginTop: 6 }} disabled={submitting}>
          {submitting ? "注册中…" : "注册"}
        </button>
      </form>
      <p style={{ fontSize: 12.5, color: "var(--text-3)", marginTop: 12 }}>
        注册即表示同意社区规则。注册后需完成邮箱验证才能发帖与评论。
      </p>
      <div style={{ textAlign: "center", marginTop: 14, fontSize: 13.5 }}>
        已有账号？
        <Link href="/login" style={{ color: "var(--brand)" }}>
          去登录
        </Link>
      </div>
    </AuthShell>
  );
}

export function AuthRegisterPage() {
  return (
    <Suspense>
      <RegisterPageInner />
    </Suspense>
  );
}

function VerifyEmailInner() {
  const params = useSearchParams();
  const invalidate = useInvalidateMe();
  const [token, setToken] = useState(params.get("token") ?? "");
  const [done, setDone] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSubmitting(true);
    try {
      await api.post("/auth/email/verify", { token });
      invalidate();
      setDone(true);
      toast("success", "邮箱验证成功");
    } catch (e2) {
      toast("error", (e2 as Error).message);
    } finally {
      setSubmitting(false);
    }
  };

  if (done) {
    return (
      <AuthShell title="验证成功 🎉">
        <p style={{ color: "var(--text-2)", marginBottom: 16 }}>你的邮箱已验证，现在可以发帖和评论了。</p>
        <Link href="/" className="btn btn-primary" style={{ display: "inline-block" }}>
          去首页看看
        </Link>
      </AuthShell>
    );
  }
  return (
    <AuthShell title="验证邮箱" subtitle="输入邮件中的验证码完成验证">
      <form onSubmit={submit}>
        <Field label="验证令牌" value={token} onChange={setToken} placeholder="邮件链接中的 token" />
        <button className="btn btn-primary" style={{ width: "100%" }} disabled={submitting || !token}>
          {submitting ? "验证中…" : "验证"}
        </button>
      </form>
      <p style={{ fontSize: 13, color: "var(--text-3)", marginTop: 12 }}>
        没收到邮件？登录后可在设置中重发验证邮件。
      </p>
    </AuthShell>
  );
}

export function AuthVerifyEmailPage() {
  return (
    <Suspense>
      <VerifyEmailInner />
    </Suspense>
  );
}

function ForgotPasswordInner() {
  const [email, setEmail] = useState("");
  const [sent, setSent] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSubmitting(true);
    try {
      await api.post("/auth/password/forgot", { email });
      setSent(true);
    } catch (e2) {
      toast("error", (e2 as Error).message);
    } finally {
      setSubmitting(false);
    }
  };

  if (sent) {
    return (
      <AuthShell title="邮件已发送">
        <p style={{ color: "var(--text-2)" }}>如该邮箱已注册，你将收到一封密码重置邮件（30 分钟内有效）。</p>
        <Link href="/login" className="btn btn-secondary" style={{ display: "inline-block", marginTop: 8 }}>
          返回登录
        </Link>
      </AuthShell>
    );
  }
  return (
    <AuthShell title="找回密码">
      <form onSubmit={submit}>
        <Field label="注册邮箱" value={email} onChange={setEmail} type="email" autoComplete="email" />
        <button className="btn btn-primary" style={{ width: "100%" }} disabled={submitting || !email}>
          {submitting ? "发送中…" : "发送重置邮件"}
        </button>
      </form>
    </AuthShell>
  );
}

export function AuthForgotPasswordPage() {
  return (
    <Suspense>
      <ForgotPasswordInner />
    </Suspense>
  );
}

function ResetPasswordInner() {
  const params = useSearchParams();
  const router = useRouter();
  const [token, setToken] = useState(params.get("token") ?? "");
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [err, setErr] = useState<{ password?: string; confirm?: string }>({});
  const [submitting, setSubmitting] = useState(false);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    const next: typeof err = {};
    if (password.length < 8) next.password = "密码至少 8 位";
    if (password !== confirm) next.confirm = "两次输入的密码不一致";
    setErr(next);
    if (Object.keys(next).length) return;
    setSubmitting(true);
    try {
      await api.post("/auth/password/reset", { token, new_password: password });
      toast("success", "密码已重置，请重新登录");
      router.push("/login");
    } catch (e2) {
      toast("error", (e2 as Error).message);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <AuthShell title="重置密码">
      <form onSubmit={submit}>
        <Field label="重置令牌" value={token} onChange={setToken} placeholder="邮件链接中的 token" />
        <Field label="新密码" value={password} onChange={setPassword} type="password" error={err.password} autoComplete="new-password" />
        <Field label="确认新密码" value={confirm} onChange={setConfirm} type="password" error={err.confirm} autoComplete="new-password" />
        <button className="btn btn-primary" style={{ width: "100%" }} disabled={submitting || !token}>
          {submitting ? "提交中…" : "重置密码"}
        </button>
      </form>
    </AuthShell>
  );
}

export function AuthResetPasswordPage() {
  return (
    <Suspense>
      <ResetPasswordInner />
    </Suspense>
  );
}
