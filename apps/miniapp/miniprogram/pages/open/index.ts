import { ApiError, shareApi, type ShareStatus } from "../../utils/api"

function formatDate(value: string): string {
  const date = new Date(value)
  const month = date.getMonth() + 1
  const day = date.getDate()
  const hour = `${date.getHours()}`.padStart(2, "0")
  const minute = `${date.getMinutes()}`.padStart(2, "0")
  return `${month}月${day}日 ${hour}:${minute}`
}

Page({
  data: {
    token: "",
    status: null as ShareStatus | null,
    expiresText: "",
    pin: "",
    phase: "loading" as "loading" | "locked" | "error",
    errorTitle: "",
    errorMessage: "",
    pinError: "",
    retryAfter: 0,
    submitting: false,
  },

  onLoad(options: Record<string, string | undefined>) {
    const token = options.t ?? ""
    this.setData({ token })
    if (!token) {
      this.setData({
        phase: "error",
        errorTitle: "链接不完整",
        errorMessage: "请重新打开管理员发送的小程序分享卡片。",
      })
      return
    }
    void this.loadStatus()
  },

  async loadStatus() {
    try {
      const status = await shareApi.status(this.data.token)
      if (status.state !== "ACTIVE") {
        this.setData({
          phase: "error",
          errorTitle: status.state === "REVOKED" ? "访问已收回" : "链接已过期",
          errorMessage:
            status.state === "REVOKED"
              ? "这条分享已被管理员撤销。"
              : "请联系发送者获取新的访问入口。",
        })
        return
      }
      this.setData({
        status,
        expiresText: formatDate(status.expires_at),
        phase: "locked",
      })
    } catch (cause) {
      this.setData({
        phase: "error",
        errorTitle: "暂时无法打开",
        errorMessage:
          cause instanceof Error ? cause.message : "请稍后重新打开分享。",
      })
    }
  },

  onPinInput(event: WechatMiniprogram.Input) {
    const pin = event.detail.value.replace(/\D/g, "").slice(0, 4)
    this.setData({ pin, pinError: "" })
  },

  async unlock() {
    if (this.data.pin.length !== 4 || this.data.submitting) return
    if (this.data.retryAfter > 0) return
    this.setData({ submitting: true, pinError: "" })
    try {
      const result = await shareApi.unlock(this.data.token, this.data.pin)
      wx.navigateTo({
        url: `/pages/viewer/index?ticket=${encodeURIComponent(result.ticket)}&title=${encodeURIComponent(
          this.data.status?.label ?? "私密方案",
        )}`,
      })
    } catch (cause) {
      if (cause instanceof ApiError) {
        if (cause.code === "RATE_LIMITED") {
          this.setData({
            pinError: "尝试次数过多，请稍后再试。",
            retryAfter: cause.retryAfter || 60,
          })
        } else if (cause.statusCode === 401) {
          this.setData({ pinError: "密码不正确，请检查后重新输入。" })
        } else {
          this.setData({ pinError: cause.message })
        }
      } else {
        this.setData({ pinError: "网络连接失败，请稍后重试。" })
      }
      this.setData({ pin: "" })
    } finally {
      this.setData({ submitting: false })
    }
  },

  onShareAppMessage() {
    return {
      title: "一份私密方案",
      path: `/pages/open/index?t=${encodeURIComponent(this.data.token)}`,
    }
  },
})
