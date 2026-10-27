import { VIEWER_ORIGIN } from "../../utils/config"

Page({
  data: {
    viewerUrl: "",
    title: "私密方案",
  },

  onLoad(options: Record<string, string | undefined>) {
    const ticket = options.ticket ?? ""
    const title = options.title ? decodeURIComponent(options.title) : "私密方案"
    wx.setNavigationBarTitle({ title })
    if (!ticket) {
      this.setData({ title: "访问票据缺失" })
      return
    }
    this.setData({
      title,
      viewerUrl: `${VIEWER_ORIGIN}/enter?ticket=${encodeURIComponent(ticket)}`,
    })
  },
})
