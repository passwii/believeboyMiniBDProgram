export type ShareState = "ACTIVE" | "EXPIRED" | "REVOKED";

export type ShareStatus = {
  share_id: string;
  state: ShareState;
  type: "PUBLIC" | "PRIVATE";
  label: string;
  expires_at: string;
};

export type ViewerSession = {
  share_id: string;
  content_title: string;
  label: string;
  watermark_text: string;
  expires_at: string;
};

export type ContentURL = {
  url: string;
  watermark_text: string;
  expires_at: string;
};

export class ViewerRequestError extends Error {
  readonly status: number;
  readonly code: string;
  readonly retryAfter: number | null;

  constructor(
    status: number,
    payload: { code?: string; message?: string },
    retryAfter: number | null = null,
  ) {
    super(payload.message ?? "请求失败，请稍后重试");
    this.status = status;
    this.code = payload.code ?? "REQUEST_FAILED";
    this.retryAfter = retryAfter;
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, {
    ...init,
    credentials: "include",
    headers: {
      ...(init?.body ? { "Content-Type": "application/json" } : {}),
      ...init?.headers,
    },
  });
  if (!response.ok) {
    let payload: { code?: string; message?: string } = {};
    try {
      payload = await response.json();
    } catch {
      // Keep the default message for non-JSON failures.
    }
    const retryAfter = Number(response.headers.get("Retry-After") || 0) || null;
    throw new ViewerRequestError(response.status, payload, retryAfter);
  }
  if (response.status === 204) return undefined as T;
  return (await response.json()) as T;
}

export const viewerApi = {
  status: (token: string) =>
    request<ShareStatus>(`/api/v1/shares/${encodeURIComponent(token)}/status`),
  session: () => request<ViewerSession>("/api/v1/viewer/session"),
  unlock: (token: string, pin: string) =>
    request<void>(`/api/v1/shares/${encodeURIComponent(token)}/unlock`, {
      method: "POST",
      body: JSON.stringify({ pin }),
    }),
  contentURL: () =>
    request<ContentURL>("/api/v1/viewer/content-url", { method: "POST" }),
  heartbeat: () =>
    request<void>("/api/v1/viewer/heartbeat", { method: "POST" }),
};
