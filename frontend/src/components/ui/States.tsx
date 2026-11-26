"use client";

// 页面三态组件：加载骨架 / 空态 / 错误态 / 页头 / 分页。
import { IconAlert, IconInbox } from "@/components/ui/Icons";

export function CardSkeleton({ lines = 3 }: { lines?: number }) {
  return (
    <div className="card" style={{ padding: "18px 20px", display: "flex", flexDirection: "column", gap: 11 }}>
      <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
        <div className="skeleton" style={{ width: 24, height: 24, borderRadius: "50%" }} />
        <div className="skeleton" style={{ height: 12, width: 120 }} />
      </div>
      <div className="skeleton" style={{ height: 19, width: "68%" }} />
      {Array.from({ length: lines - 1 }).map((_, i) => (
        <div key={i} className="skeleton" style={{ height: 13, width: `${88 - i * 16}%` }} />
      ))}
    </div>
  );
}

export function ListSkeleton({ count = 5 }: { count?: number }) {
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 11 }}>
      {Array.from({ length: count }).map((_, i) => (
        <CardSkeleton key={i} />
      ))}
    </div>
  );
}

export function EmptyState({
  title,
  description,
  action,
}: {
  title: string;
  description?: string;
  action?: React.ReactNode;
}) {
  return (
    <div className="card" style={{ padding: "54px 24px", textAlign: "center" }}>
      <div
        style={{
          width: 50,
          height: 50,
          borderRadius: 14,
          background: "var(--brand-soft)",
          color: "var(--brand)",
          display: "inline-flex",
          alignItems: "center",
          justifyContent: "center",
          marginBottom: 14,
          border: "1px solid var(--border)",
        }}
      >
        <IconInbox size={22} />
      </div>
      <div style={{ fontWeight: 650, fontSize: 15.5, letterSpacing: "-0.015em", marginBottom: 5 }}>{title}</div>
      {description && (
        <div style={{ color: "var(--text-3)", fontSize: 13.5, maxWidth: 360, margin: `0 auto ${action ? 18 : 0}px`, lineHeight: 1.6 }}>
          {description}
        </div>
      )}
      {action}
    </div>
  );
}

export function ErrorState({ message, onRetry }: { message?: string; onRetry?: () => void }) {
  return (
    <div className="card" style={{ padding: "48px 24px", textAlign: "center" }}>
      <div
        style={{
          width: 50,
          height: 50,
          borderRadius: 14,
          background: "var(--danger-soft)",
          color: "var(--danger)",
          display: "inline-flex",
          alignItems: "center",
          justifyContent: "center",
          marginBottom: 14,
        }}
      >
        <IconAlert size={22} />
      </div>
      <div style={{ fontWeight: 650, fontSize: 15.5, marginBottom: 5 }}>加载失败</div>
      <div style={{ color: "var(--text-3)", fontSize: 13.5, marginBottom: onRetry ? 18 : 0 }}>
        {message || "请求未能完成，请稍后重试"}
      </div>
      {onRetry && (
        <button className="btn btn-secondary btn-sm" onClick={onRetry}>
          重新加载
        </button>
      )}
    </div>
  );
}

export function PageHeader({
  title,
  subtitle,
  eyebrow,
  action,
}: {
  title: string;
  subtitle?: string;
  eyebrow?: string;
  action?: React.ReactNode;
}) {
  return (
    <div
      style={{
        marginBottom: 20,
        paddingBottom: 16,
        borderBottom: "1px solid var(--border)",
        display: "flex",
        alignItems: "flex-end",
        gap: 14,
        flexWrap: "wrap",
      }}
    >
      <div style={{ flex: 1, minWidth: 200 }}>
        {eyebrow && (
          <div
            style={{
              fontSize: 11.5,
              fontWeight: 650,
              letterSpacing: "0.06em",
              textTransform: "uppercase",
              color: "var(--brand)",
              marginBottom: 4,
            }}
          >
            {eyebrow}
          </div>
        )}
        <h1 style={{ fontSize: 24, fontWeight: 720, letterSpacing: "-0.025em", lineHeight: 1.25 }}>{title}</h1>
        {subtitle && (
          <p style={{ color: "var(--text-3)", fontSize: 13.5, marginTop: 5, lineHeight: 1.55 }}>{subtitle}</p>
        )}
      </div>
      {action}
    </div>
  );
}

/** 分页控件 */
export function Pagination({
  page,
  pageSize,
  total,
  onChange,
}: {
  page: number;
  pageSize: number;
  total: number;
  onChange: (page: number) => void;
}) {
  const pages = Math.max(1, Math.ceil(total / pageSize));
  if (pages <= 1) return null;
  const nums: number[] = [];
  for (let i = Math.max(1, page - 2); i <= Math.min(pages, page + 2); i++) {
    nums.push(i);
  }
  return (
    <div style={{ display: "flex", gap: 6, justifyContent: "center", alignItems: "center", marginTop: 24 }}>
      <button className="btn btn-secondary btn-sm" disabled={page <= 1} onClick={() => onChange(page - 1)}>
        上一页
      </button>
      {nums[0] > 1 && <span style={{ color: "var(--text-3)", padding: "0 4px" }}>…</span>}
      {nums.map((n) => (
        <button
          key={n}
          className={`btn btn-sm tabular-nums ${n === page ? "btn-primary" : "btn-secondary"}`}
          style={{ minWidth: 34, padding: "7px 10px" }}
          onClick={() => onChange(n)}
          aria-current={n === page ? "page" : undefined}
        >
          {n}
        </button>
      ))}
      {nums[nums.length - 1] < pages && <span style={{ color: "var(--text-3)", padding: "0 4px" }}>…</span>}
      <button className="btn btn-secondary btn-sm" disabled={page >= pages} onClick={() => onChange(page + 1)}>
        下一页
      </button>
    </div>
  );
}
