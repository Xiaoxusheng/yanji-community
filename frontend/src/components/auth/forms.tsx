"use client";

// 登录 / 注册 / 验证邮箱 / 找回密码 / 重置密码（共享 auth 布局与图形验证码安全校验）。
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, useId, useState } from "react";
import { api } from "@/lib/api";
import { toast } from "@/lib/toast";
import { useInvalidateMe } from "@/lib/auth";
import { CaptchaField } from "@/components/auth/CaptchaField";
import { IconCheckCircle, IconEye, IconEyeOff, IconShield } from "@/components/ui/Icons";

export function validatePasswordClient(pwd: string): string | undefined {
  if (pwd.length < 8 || pwd.length > 72) return "密码长度需为 8-72 位";
  if (!pwd.trim()) return "密码不能全为空格";
  let classes = 0;
  if (/[a-z]/.test(pwd)) classes++;
  if (/[A-Z]/.test(pwd)) classes++;
  if (/[0-9]/.test(pwd)) classes++;
  if (/[^a-zA-Z0-9\s]/.test(pwd)) classes++;
  if (classes < 2) return "需包含字母、数字或符号中的至少两种";
  return undefined;
}

function passwordStrength(pwd: string): { level: number; label: string; color: string } {
  if (!pwd) return { level: 0, label: "", color: "transparent" };
  let score = 0;
  if (pwd.length >= 8) score++;
  if (pwd.length >= 12) score++;
  if (/[a-z]/.test(pwd) && /[A-Z]/.test(pwd)) score++;
  if (/[0-9]/.test(pwd)) score++;
  if (/[^a-zA-Z0-9\s]/.test(pwd)) score++;
  if (score <= 2) return { level: 1, label: "弱", color: "var(--danger)" };
  if (score === 3) return { level: 2, label: "中", color: "var(--warning)" };
  return { level: 3, label: "强", color: "var(--success)" };
}

function PasswordStrengthBar({ password }: { password: string }) {
  const st = passwordStrength(password);
  if (!password) return null;
  return (
    <div style={{ marginTop: -8, marginBottom: 14 }}>
      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr 1fr", gap: 5, marginBottom: 4 }}>
        {[1, 2, 3].map((i) => (
          <div
            key={i}
            style={{
              height: 4,
              borderRadius: 999,
              background: i <= st.level ? st.color : "var(--bg-inset)",
              transition: "background-color 0.2s ease",
            }}
          />
        ))}
      </div>
      <div style={{ fontSize: 11.5, color: st.color, textAlign: "right", fontWeight: 550 }}>
        密码强度：{st.label}
      </div>
    </div>
  );
}

export function AuthShell({ title, subtitle, children }: { title: string; subtitle?: string; children: React.ReactNode }) {
  return (
    <div style={{ maxWidth: 424, margin: "44px auto" }}>
      <div
        className="surface-spotlight"
        style={{
          padding: "32px 30px 28px",
          borderRadius: "var(--radius-lg)",
          boxShadow: "var(--shadow-hover)",
        }}
      >
        <div style={{ display: "flex", alignItems: "center", gap: 10, marginBottom: 18 }}>
          <span className="brand-mark" aria-hidden>
            研
          </span>
          <span style={{ fontSize: 13.5, fontWeight: 650, color: "var(--text-2)" }}>研记社区</span>
        </div>
        <h1 style={{ fontSize: 23, fontWeight: 740, letterSpacing: "-0.025em", marginBottom: 5 }}>{title}</h1>
        {subtitle && (
          <p style={{ color: "var(--text-3)", fontSize: 13.5, marginBottom: 22, lineHeight: 1.5 }}>{subtitle}</p>
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
  hint,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  type?: string;
  placeholder?: string;
  error?: string;
  autoComplete?: string;
  hint?: string;
}) {
  const inputId = useId();
  const isPassword = type === "password";
  const [visible, setVisible] = useState(false);

  return (
    <div style={{ marginBottom: 14 }}>
      <label htmlFor={inputId} style={{ display: "block", fontSize: 13.5, marginBottom: 6, fontWeight: 600 }}>
        {label}
      </label>
      <div style={{ position: "relative" }}>
        <input
          id={inputId}
          className="input"
          type={isPassword && visible ? "text" : type}
          value={value}
          placeholder={placeholder}
          autoComplete={autoComplete}
          aria-invalid={!!error}
          onChange={(e) => onChange(e.target.value)}
          style={isPassword ? { paddingRight: 38 } : undefined}
        />
        {isPassword && (
          <button
            type="button"
            tabIndex={-1}
            aria-label={visible ? "隐藏密码" : "显示密码"}
            onClick={() => setVisible((v) => !v)}
            style={{
              position: "absolute",
              right: 6,
              top: "50%",
              transform: "translateY(-50%)",
              width: 28,
              height: 28,
              border: "none",
              background: "transparent",
              color: "var(--text-3)",
              display: "inline-flex",
              alignItems: "center",
              justifyContent: "center",
              cursor: "pointer",
              borderRadius: 6,
            }}
          >
            {visible ? <IconEyeOff size={15} /> : <IconEye size={15} />}
          </button>
        )}
      </div>
      {hint && !error && <div style={{ color: "var(--text-3)", fontSize: 12, marginTop: 4 }}>{hint}</div>}
      {error && <div style={{ color: "var(--danger)", fontSize: 12.5, marginTop: 4 }}>{error}</div>}
    </div>
  );
}

function LoginPageInner() {
  const router = useRouter();
  const invalidate = useInvalidateMe();
  const [account, setAccount] = useState("");
  const [password, setPassword] = useState("");
  const [captchaId, setCaptchaId] = useState("");
  const [captchaCode, setCaptchaCode] = useState("");
  const [refreshKey, setRefreshKey] = useState(0);
  const [err, setErr] = useState<{ account?: string; password?: string; captcha?: string }>({});
  const [submitting, setSubmitting] = useState(false);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    const next: typeof err = {};
    if (!account.trim()) next.account = "请输入用户名或邮箱";
    if (!password) next.password = "请输入密码";
    if (!captchaCode.trim()) next.captcha = "请输入图形验证码";
    setErr(next);
    if (Object.keys(next).length) return;
    setSubmitting(true);
    try {
      await api.post("/auth/login", {
        account: account.trim(),
        password,
        captcha_id: captchaId,
        captcha_code: captchaCode.trim(),
      });
      invalidate();
      toast("success", "登录成功");
      router.push("/");
    } catch (e2) {
      toast("error", (e2 as Error).message);
      // 每次登录失败后后端均会消费该验证码，强制刷新新挑战防止重用报错
      setRefreshKey((k) => k + 1);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <AuthShell title="登录" subtitle="欢迎回来">
      <form onSubmit={submit} noValidate>
        <Field label="用户名 / 邮箱" value={account} onChange={setAccount} error={err.account} autoComplete="username" />
        <Field label="密码" value={password} onChange={setPassword} type="password" error={err.password} autoComplete="current-password" />
        <CaptchaField
          captchaId={captchaId}
          code={captchaCode}
          onCaptchaChange={setCaptchaId}
          onCodeChange={setCaptchaCode}
          error={err.captcha}
          refreshKey={refreshKey}
        />
        <button className="btn btn-primary" style={{ width: "100%", marginTop: 6 }} disabled={submitting}>
          {submitting ? "登录中…" : "登录"}
        </button>
      </form>
      <div
        style={{
          marginTop: 14,
          padding: "8px 10px",
          borderRadius: "var(--radius-sm)",
          background: "var(--bg-inset)",
          color: "var(--text-3)",
          fontSize: 12,
          display: "flex",
          alignItems: "center",
          gap: 6,
        }}
      >
        <IconShield size={13} />
        <span>连续输错 5 次将触发 15 分钟保护锁定</span>
      </div>
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
  const [captchaId, setCaptchaId] = useState("");
  const [captchaCode, setCaptchaCode] = useState("");
  const [refreshKey, setRefreshKey] = useState(0);
  const [err, setErr] = useState<{ username?: string; email?: string; password?: string; captcha?: string }>({});
  const [submitting, setSubmitting] = useState(false);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    const next: typeof err = {};
    if (!/^[a-zA-Z0-9_]{3,20}$/.test(username)) next.username = "3-20 位字母、数字或下划线";
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) next.email = "邮箱格式不正确";
    const pwdErr = validatePasswordClient(password);
    if (pwdErr) next.password = pwdErr;
    if (!captchaCode.trim()) next.captcha = "请输入图形验证码";
    setErr(next);
    if (Object.keys(next).length) return;
    setSubmitting(true);
    try {
      await api.post("/auth/register", {
        username,
        email,
        password,
        captcha_id: captchaId,
        captcha_code: captchaCode.trim(),
      });
      toast("success", "注册成功，请查收验证邮件");
      router.push(`/verify-email?email=${encodeURIComponent(email)}`);
    } catch (e2) {
      toast("error", (e2 as Error).message);
      setRefreshKey((k) => k + 1);
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <AuthShell title="注册" subtitle="公开注册，加入社区">
      <form onSubmit={submit} noValidate>
        <Field label="用户名" value={username} onChange={setUsername} error={err.username} placeholder="3-20 位字母、数字或下划线" autoComplete="username" />
        <Field label="邮箱" value={email} onChange={setEmail} type="email" error={err.email} autoComplete="email" />
        <Field
          label="密码"
          value={password}
          onChange={setPassword}
          type="password"
          error={err.password}
          placeholder="至少 8 位，需含字母与数字/符号"
          autoComplete="new-password"
        />
        <PasswordStrengthBar password={password} />
        <CaptchaField
          captchaId={captchaId}
          code={captchaCode}
          onCaptchaChange={setCaptchaId}
          onCodeChange={setCaptchaCode}
          error={err.captcha}
          refreshKey={refreshKey}
        />
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
      <AuthShell title="邮箱验证成功">
        <div style={{ display: "flex", alignItems: "center", gap: 8, color: "var(--success)", marginBottom: 14, fontSize: 14 }}>
          <IconCheckCircle size={18} />
          <span>你的邮箱已完成验证，现在可以发帖和评论了。</span>
        </div>
        <Link href="/" className="btn btn-primary" style={{ display: "inline-block" }}>
          去首页看看
        </Link>
      </AuthShell>
    );
  }
  return (
    <AuthShell title="验证邮箱" subtitle="输入邮件中的验证令牌完成激活">
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
  const [captchaId, setCaptchaId] = useState("");
  const [captchaCode, setCaptchaCode] = useState("");
  const [refreshKey, setRefreshKey] = useState(0);
  const [err, setErr] = useState<{ email?: string; captcha?: string }>({});
  const [sent, setSent] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    const next: typeof err = {};
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.trim())) next.email = "请输入正确的注册邮箱";
    if (!captchaCode.trim()) next.captcha = "请输入图形验证码";
    setErr(next);
    if (Object.keys(next).length) return;
    setSubmitting(true);
    try {
      await api.post("/auth/password/forgot", {
        email: email.trim(),
        captcha_id: captchaId,
        captcha_code: captchaCode.trim(),
      });
      setSent(true);
    } catch (e2) {
      toast("error", (e2 as Error).message);
      setRefreshKey((k) => k + 1);
    } finally {
      setSubmitting(false);
    }
  };

  if (sent) {
    return (
      <AuthShell title="邮件已发送">
        <p style={{ color: "var(--text-2)" }}>如该邮箱已注册，你将收到一封密码重置邮件（30 分钟内有效）。</p>
        <Link href="/login" className="btn btn-secondary" style={{ display: "inline-block", marginTop: 12 }}>
          返回登录
        </Link>
      </AuthShell>
    );
  }
  return (
    <AuthShell title="找回密码" subtitle="输入注册邮箱获取一次性重置链接">
      <form onSubmit={submit} noValidate>
        <Field label="注册邮箱" value={email} onChange={setEmail} type="email" error={err.email} autoComplete="email" />
        <CaptchaField
          captchaId={captchaId}
          code={captchaCode}
          onCaptchaChange={setCaptchaId}
          onCodeChange={setCaptchaCode}
          error={err.captcha}
          refreshKey={refreshKey}
        />
        <button className="btn btn-primary" style={{ width: "100%" }} disabled={submitting}>
          {submitting ? "发送中…" : "发送重置邮件"}
        </button>
      </form>
      <div style={{ textAlign: "center", marginTop: 14, fontSize: 13.5 }}>
        <Link href="/login" style={{ color: "var(--brand)" }}>
          返回登录
        </Link>
      </div>
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
    const pwdErr = validatePasswordClient(password);
    if (pwdErr) next.password = pwdErr;
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
    <AuthShell title="重置密码" subtitle="重置成功后将自动下线所有登录设备">
      <form onSubmit={submit}>
        <Field label="重置令牌" value={token} onChange={setToken} placeholder="邮件链接中的 token" />
        <Field
          label="新密码"
          value={password}
          onChange={setPassword}
          type="password"
          error={err.password}
          placeholder="至少 8 位，需含字母与数字/符号"
          autoComplete="new-password"
        />
        <PasswordStrengthBar password={password} />
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
