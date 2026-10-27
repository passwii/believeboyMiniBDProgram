export type AdminPrincipal = {
  id: string;
  email: string;
  role: "ADMIN" | "STAFF";
};

export type Analysis = {
  viewport_present: boolean;
  script_count: number;
  external_resources: string[];
  warnings: string[];
};

export type ContentItem = {
  id: string;
  title: string;
  status: string;
  version_id: string;
  version_no: number;
  sha256: string;
  byte_size: number;
  analysis: Analysis;
  created_at: string;
};

export type ShareItem = {
  id: string;
  content_id: string;
  content_version_id: string;
  content_title: string;
  type: "PUBLIC" | "PRIVATE";
  label: string;
  watermark_text: string;
  expires_at: string;
  revoked_at: string | null;
  session_ttl_seconds: number;
  created_at: string;
  h5_url: string;
  miniapp_path: string;
};

export type ApiError = {
  code: string;
  message: string;
  request_id?: string;
};

export class ApiRequestError extends Error {
  readonly status: number;
  readonly code: string;
  readonly requestId?: string;

  constructor(status: number, payload: ApiError) {
    super(payload.message);
    this.status = status;
    this.code = payload.code;
    this.requestId = payload.request_id;
  }
}

async function parseError(response: Response): Promise<ApiError> {
  try {
    const payload = (await response.json()) as Partial<ApiError>;
    return {
      code: payload.code ?? "REQUEST_FAILED",
      message: payload.message ?? "请求失败，请稍后重试",
      request_id: payload.request_id,
    };
  } catch {
    return { code: "REQUEST_FAILED", message: "请求失败，请稍后重试" };
  }
}

export async function apiFetch<T>(
  input: string,
  init: RequestInit = {},
): Promise<T> {
  const response = await fetch(input, {
    ...init,
    credentials: "include",
    headers: {
      ...(init.body instanceof FormData
        ? {}
        : { "Content-Type": "application/json" }),
      ...init.headers,
    },
  });
  if (!response.ok) {
    throw new ApiRequestError(response.status, await parseError(response));
  }
  if (response.status === 204) {
    return undefined as T;
  }
  return (await response.json()) as T;
}

export const adminApi = {
  me: () => apiFetch<AdminPrincipal>("/api/v1/admin/me"),
  login: (email: string, credential: string) =>
    apiFetch<void>("/api/v1/admin/login", {
      method: "POST",
      body: JSON.stringify({ email, credential }),
    }),
  logout: () =>
    apiFetch<void>("/api/v1/admin/logout", { method: "POST" }),
  contents: () => apiFetch<ContentItem[]>("/api/v1/admin/contents?limit=100"),
  content: (id: string) =>
    apiFetch<ContentItem>(`/api/v1/admin/contents/${id}`),
  upload: (file: File) => {
    const body = new FormData();
    body.append("file", file);
    return apiFetch<ContentItem>("/api/v1/admin/contents", {
      method: "POST",
      body,
    });
  },
  shares: () => apiFetch<ShareItem[]>("/api/v1/admin/shares?limit=100"),
  createShare: (payload: {
    content_id: string;
    type: "PUBLIC" | "PRIVATE";
    label: string;
    pin: string;
    watermark_text: string;
    expires_at: string;
    session_ttl_seconds: number;
  }) =>
    apiFetch<ShareItem>("/api/v1/admin/shares", {
      method: "POST",
      body: JSON.stringify(payload),
    }),
  revokeShare: (id: string) =>
    apiFetch<void>(`/api/v1/admin/shares/${id}/revoke`, { method: "POST" }),
  resetPin: (id: string, pin: string) =>
    apiFetch<void>(`/api/v1/admin/shares/${id}/reset-pin`, {
      method: "POST",
      body: JSON.stringify({ pin }),
    }),
};
