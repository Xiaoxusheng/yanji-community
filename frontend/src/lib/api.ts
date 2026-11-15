// 统一 API Client：携带会话 Cookie，按稳定错误码处理。
export const API_BASE =
  process.env.NEXT_PUBLIC_API_BASE_URL || "http://localhost:8080/api/v1";

export class ApiError extends Error {
  code: string;
  status: number;
  requestId?: string;

  constructor(status: number, code: string, message: string, requestId?: string) {
    super(message);
    this.status = status;
    this.code = code;
    this.requestId = requestId;
  }
}

export interface Envelope<T = unknown> {
  code: string;
  message: string;
  data: T;
  request_id?: string;
}

export interface PageData<T> {
  items: T[];
  page: number;
  page_size: number;
  total: number;
  has_more: boolean;
}

interface RequestOptions {
  method?: string;
  body?: unknown;
  formData?: FormData;
  idempotencyKey?: string;
}

async function request<T>(path: string, opts: RequestOptions = {}): Promise<T> {
  const headers: Record<string, string> = {};
  if (opts.body !== undefined) headers["Content-Type"] = "application/json";
  if (opts.idempotencyKey) headers["Idempotency-Key"] = opts.idempotencyKey;

  const resp = await fetch(`${API_BASE}${path}`, {
    method: opts.method ?? "GET",
    headers,
    body: opts.formData ?? (opts.body !== undefined ? JSON.stringify(opts.body) : undefined),
    credentials: "include",
    cache: "no-store",
  });

  let env: Envelope<T> | null = null;
  try {
    env = (await resp.json()) as Envelope<T>;
  } catch {
    // 非 JSON 响应（网关错误等）
  }
  if (!resp.ok || !env || (env.code && env.code !== "OK")) {
    const code = env?.code ?? `HTTP_${resp.status}`;
    const message = env?.message ?? "网络错误，请稍后重试";
    throw new ApiError(resp.status, code, message, env?.request_id);
  }
  return env.data;
}

export const api = {
  get: <T>(path: string) => request<T>(path),
  post: <T>(path: string, body?: unknown, idempotencyKey?: string) =>
    request<T>(path, { method: "POST", body, idempotencyKey }),
  patch: <T>(path: string, body?: unknown) => request<T>(path, { method: "PATCH", body }),
  delete: <T>(path: string) => request<T>(path, { method: "DELETE" }),
  upload: <T>(path: string, formData: FormData) =>
    request<T>(path, { method: "POST", formData }),
};

/** 拼接文件访问地址（本地存储静态服务） */
export function fileURL(key: string): string | null {
  if (!key) return null;
  if (key.startsWith("http://") || key.startsWith("https://")) return key;
  const origin = API_BASE.replace(/\/api\/v1$/, "");
  return `${origin}${key.startsWith("/") ? "" : "/files/"}${key.startsWith("/") ? "" : "files/"}${key}`;
}

/** 相对时间格式化 */
export function timeAgo(iso: string | null | undefined): string {
  if (!iso) return "";
  const t = new Date(iso).getTime();
  if (Number.isNaN(t)) return "";
  const diff = Date.now() - t;
  const minute = 60_000, hour = 3_600_000, day = 86_400_000;
  if (diff < minute) return "刚刚";
  if (diff < hour) return `${Math.floor(diff / minute)} 分钟前`;
  if (diff < day) return `${Math.floor(diff / hour)} 小时前`;
  if (diff < day * 30) return `${Math.floor(diff / day)} 天前`;
  return new Date(iso).toLocaleDateString("zh-CN");
}

/** 格式化数字（1.2k） */
export function fmtCount(n: number): string {
  if (n < 1000) return String(n);
  if (n < 1_000_000) return `${(n / 1000).toFixed(1)}k`;
  return `${(n / 1_000_000).toFixed(1)}M`;
}
