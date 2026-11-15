"use client";

// 页面三态组件：加载骨架 / 空态 / 错误态（UI_SPEC 状态矩阵强制项）。

export function CardSkeleton({ lines = 3 }: { lines?: number }) {
  return (
    <div className="card" style={{ padding: 16, display: "flex", flexDirection: "column", gap: 10 }}>
      <div className="skeleton" style={{ height: 20, width: "60%" }} />
      {Array.from({ length: lines - 1 }).map((_, i) => (
        <div key={i} className="skeleton" style={{ height: 14, width: `${90 - i * 15}%` }} />
      ))}
    </div>
  );
}

export function ListSkeleton({ count = 5 }: { count?: number }) {
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
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
    <div className="card" style={{ padding: "48px 24px", textAlign: "center" }}>
      <div style={{ fontSize: 40, marginBottom: 8, opacity: 0.5 }}>🗒️</div>
      <div style={{ fontWeight: 600, marginBottom: 4 }}>{title}</div>
      {description && (
        <div style={{ color: "var(--text-3)", fontSize: 14, marginBottom: 16 }}>{description}</div>
      )}
      {action}
    </div>
  );
}

export function ErrorState({ message, onRetry }: { message?: string; onRetry?: () => void }) {
  return (
    <div className="card" style={{ padding: "48px 24px", textAlign: "center" }}>
      <div style={{ fontSize: 40, marginBottom: 8 }}>⚠️</div>
      <div style={{ fontWeight: 600, marginBottom: 4 }}>加载失败</div>
      <div style={{ color: "var(--text-3)", fontSize: 14, marginBottom: 16 }}>
        {message || "出了点问题，请稍后重试"}
      </div>
      {onRetry && (
        <button className="btn btn-secondary btn-sm" onClick={onRetry}>
          重试
        </button>
      )}
    </div>
  );
}

export function PageHeader({ title, subtitle }: { title: string; subtitle?: string }) {
  return (
    <div style={{ marginBottom: 20 }}>
      <h1 style={{ fontSize: 24, fontWeight: 700 }}>{title}</h1>
      {subtitle && (
        <p style={{ color: "var(--text-3)", fontSize: 14, marginTop: 4 }}>{subtitle}</p>
      )}
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
  for (
    let i = Math.max(1, page - 2);
    i <= Math.min(pages, page + 2);
    i++
  ) {
    nums.push(i);
  }
  return (
    <div style={{ display: "flex", gap: 6, justifyContent: "center", marginTop: 20 }}>
      <button className="btn btn-secondary btn-sm" disabled={page <= 1} onClick={() => onChange(page - 1)}>
        上一页
      </button>
      {nums[0] > 1 && <span style={{ alignSelf: "center", color: "var(--text-3)" }}>…</span>}
      {nums.map((n) => (
        <button
          key={n}
          className={`btn btn-sm ${n === page ? "btn-primary" : "btn-secondary"}`}
          onClick={() => onChange(n)}
          aria-current={n === page ? "page" : undefined}
        >
          {n}
        </button>
      ))}
      {nums[nums.length - 1] < pages && <span style={{ alignSelf: "center", color: "var(--text-3)" }}>…</span>}
      <button className="btn btn-secondary btn-sm" disabled={page >= pages} onClick={() => onChange(page + 1)}>
        下一页
      </button>
    </div>
  );
}
