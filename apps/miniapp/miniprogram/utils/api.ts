import { API_ORIGIN } from "./config"

export type ShareStatus = {
  state: "ACTIVE" | "EXPIRED" | "REVOKED"
  type: "PUBLIC" | "PRIVATE"
  label: string
  expires_at: string
}

export type MiniUnlockResult = {
  ticket: string
  expires_at: string
}

export class ApiError extends Error {
  statusCode: number
  code: string
  retryAfter: number

  constructor(statusCode: number, code: string, message: string, retryAfter = 0) {
    super(message)
    this.statusCode = statusCode
    this.code = code
    this.retryAfter = retryAfter
  }
}

function request<T>(
  path: string,
  method: "GET" | "POST",
  data?: Record<string, unknown>,
): Promise<T> {
  return new Promise((resolve, reject) => {
    wx.request({
      url: `${API_ORIGIN}${path}`,
      method,
      data,
      timeout: 15000,
      header: {
        "Content-Type": "application/json",
      },
      success(response) {
        if (response.statusCode >= 200 && response.statusCode < 300) {
          resolve(response.data as T)
          return
        }
        const payload = response.data as { code?: string; message?: string }
        const retryAfter = Number(response.header?.["Retry-After"] || 0)
        reject(
          new ApiError(
            response.statusCode,
            payload?.code ?? "REQUEST_FAILED",
            payload?.message ?? "请求失败，请稍后重试",
            Number.isFinite(retryAfter) ? retryAfter : 0,
          ),
        )
      },
      fail() {
        reject(new ApiError(0, "NETWORK_ERROR", "网络连接失败，请稍后重试"))
      },
    })
  })
}

export const shareApi = {
  status(token: string) {
    return request<ShareStatus>(
      `/api/v1/shares/${encodeURIComponent(token)}/status`,
      "GET",
    )
  },
  unlock(token: string, pin: string) {
    return request<MiniUnlockResult>(
      `/api/v1/miniapp/shares/${encodeURIComponent(token)}/unlock`,
      "POST",
      { pin },
    )
  },
}
