"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  ChevronRight,
  FileText,
  LayoutDashboard,
  Link2,
  LogOut,
  Menu,
  X,
} from "lucide-react";
import { useState } from "react";

import { adminApi, ApiRequestError } from "@/lib/api";
import { cn } from "@/lib/cn";
import { BrandMark } from "@/components/brand-mark";
import { Button } from "@/components/ui/button";
import { LoginView } from "@/components/login-view";
import { OverviewPanel } from "@/components/overview-panel";
import { ContentsPanel } from "@/components/contents-panel";
import { SharesPanel } from "@/components/shares-panel";
import { Toast, type ToastState } from "@/components/ui/toast";

type View = "overview" | "contents" | "shares";

const navItems: Array<{
  key: View;
  label: string;
  caption: string;
  icon: typeof LayoutDashboard;
}> = [
  { key: "overview", label: "总览", caption: "桌面近况", icon: LayoutDashboard },
  { key: "contents", label: "方案内容", caption: "上传与版本", icon: FileText },
  { key: "shares", label: "分享链接", caption: "访问与权限", icon: Link2 },
];

export function AdminApp() {
  const queryClient = useQueryClient();
  const [view, setView] = useState<View>("overview");
  const [navOpen, setNavOpen] = useState(false);
  const [toast, setToast] = useState<ToastState | null>(null);

  const me = useQuery({
    queryKey: ["admin", "me"],
    queryFn: adminApi.me,
    retry: false,
  });
  const contents = useQuery({
    queryKey: ["admin", "contents"],
    queryFn: adminApi.contents,
    enabled: me.isSuccess,
  });
  const shares = useQuery({
    queryKey: ["admin", "shares"],
    queryFn: adminApi.shares,
    enabled: me.isSuccess,
  });

  const logout = useMutation({
    mutationFn: adminApi.logout,
    onSettled: () => {
      queryClient.removeQueries({ queryKey: ["admin"] });
    },
  });

  const showError = (error: unknown) => {
    if (error instanceof ApiRequestError) {
      setToast({ tone: "error", message: error.message });
      return;
    }
    setToast({
      tone: "error",
      message: error instanceof Error ? error.message : "操作失败，请稍后重试",
    });
  };

  const showSaved = (message: string) => setToast({ tone: "success", message });

  if (me.isLoading) {
    return <AppSkeleton />;
  }

  if (me.isError) {
    if (me.error instanceof ApiRequestError && me.error.status === 401) {
      return <LoginView onSuccess={() => me.refetch()} />;
    }
    return (
      <div className="grid min-h-dvh place-items-center px-5">
        <div className="max-w-md text-center">
          <BrandMark />
          <h1 className="mt-8 font-display text-4xl">暂时无法连接服务</h1>
          <p className="mt-3 text-sm leading-6 text-ink-soft">
            {me.error instanceof ApiRequestError
              ? me.error.message
              : "请确认 API 服务已经启动，然后刷新页面。"}
          </p>
          <Button className="mt-6" onClick={() => me.refetch()}>
            重新连接
          </Button>
        </div>
      </div>
    );
  }

  if (!me.data) {
    return <AppSkeleton />;
  }
  const principal = me.data;
  const current = navItems.find((item) => item.key === view) ?? navItems[0];

  return (
    <div className="min-h-dvh lg:grid lg:grid-cols-[260px_minmax(0,1fr)]">
      {navOpen && (
        <button
          type="button"
          aria-label="关闭导航"
          className="fixed inset-0 z-30 bg-ink/30 lg:hidden"
          onClick={() => setNavOpen(false)}
        />
      )}
      <aside
        className={cn(
          "fixed inset-y-0 left-0 z-40 flex w-[270px] -translate-x-full flex-col border-r border-line bg-paper px-5 py-6 transition-transform duration-300 lg:static lg:w-auto lg:translate-x-0",
          navOpen && "translate-x-0",
        )}
      >
        <div className="flex items-start justify-between">
          <BrandMark />
          <button
            type="button"
            className="grid h-10 w-10 place-items-center rounded-full text-ink-soft hover:bg-ink/5 lg:hidden"
            aria-label="关闭导航"
            onClick={() => setNavOpen(false)}
          >
            <X size={18} />
          </button>
        </div>

        <p className="mt-14 max-w-[13rem] font-display text-[1.65rem] leading-[1.1] tracking-[-0.025em] text-ink">
          私密方案，
          <br />
          只交给对的人。
        </p>
        <p className="mt-3 max-w-[14rem] text-[0.78rem] leading-5 text-muted">
          上传内容、控制入口、随时收回访问。
        </p>

        <nav className="mt-10 space-y-1.5">
          {navItems.map((item) => {
            const Icon = item.icon;
            const active = item.key === view;
            return (
              <button
                key={item.key}
                type="button"
                onClick={() => {
                  setView(item.key);
                  setNavOpen(false);
                }}
                className={cn(
                  "group flex w-full items-center gap-3 rounded-[11px] px-3 py-3 text-left transition-colors",
                  active
                    ? "bg-ink text-paper"
                    : "text-ink-soft hover:bg-ink/[0.055] hover:text-ink",
                )}
              >
                <Icon size={17} strokeWidth={1.8} />
                <span className="flex-1">
                  <span className="block text-[0.88rem] font-medium">
                    {item.label}
                  </span>
                  <span
                    className={cn(
                      "mt-0.5 block text-[0.68rem]",
                      active ? "text-paper/60" : "text-muted",
                    )}
                  >
                    {item.caption}
                  </span>
                </span>
                <ChevronRight
                  size={14}
                  className={cn("opacity-0 transition-opacity", active && "opacity-60")}
                />
              </button>
            );
          })}
        </nav>

        <div className="mt-auto border-t border-line pt-5">
          <div className="flex items-center gap-3 px-1">
            <div className="grid h-9 w-9 place-items-center rounded-full bg-accent-soft font-display text-sm text-accent">
              {principal.email.slice(0, 1).toUpperCase()}
            </div>
            <div className="min-w-0 flex-1">
              <p className="truncate text-[0.78rem] font-medium">{principal.email}</p>
              <p className="mt-0.5 text-[0.67rem] uppercase tracking-[0.13em] text-muted">
                {principal.role}
              </p>
            </div>
            <button
              type="button"
              aria-label="退出登录"
              className="grid h-9 w-9 place-items-center rounded-full text-muted hover:bg-ink/5 hover:text-ink"
              onClick={() => logout.mutate()}
            >
              <LogOut size={16} />
            </button>
          </div>
        </div>
      </aside>

      <div className="min-w-0">
        <header className="sticky top-0 z-20 flex items-center gap-3 border-b border-line/80 bg-paper/92 px-5 py-4 backdrop-blur-md lg:hidden">
          <button
            type="button"
            className="grid h-10 w-10 place-items-center rounded-full border border-line"
            aria-label="打开导航"
            onClick={() => setNavOpen(true)}
          >
            <Menu size={18} />
          </button>
          <BrandMark compact />
          <span className="ml-auto text-[0.72rem] uppercase tracking-[0.12em] text-muted">
            {current.label}
          </span>
        </header>

        <main className="mx-auto w-full max-w-[1500px] px-5 py-8 sm:px-8 lg:px-10 lg:py-10 xl:px-14">
          <div className="mb-8 flex flex-wrap items-end justify-between gap-4">
            <div>
              <p className="text-[0.69rem] uppercase tracking-[0.16em] text-muted">
                {current.caption}
              </p>
              <h1 className="mt-2 font-display text-[clamp(2rem,4vw,3.25rem)] leading-none tracking-[-0.03em]">
                {current.label}
              </h1>
            </div>
            <p className="max-w-sm text-right text-[0.75rem] leading-5 text-muted">
              每个分享都可独立设置密码、有效期与撤销状态。
            </p>
          </div>

          {view === "overview" && (
            <OverviewPanel
              contents={contents.data ?? []}
              shares={shares.data ?? []}
              loading={contents.isLoading || shares.isLoading}
              onNavigate={setView}
            />
          )}
          {view === "contents" && (
            <ContentsPanel
              contents={contents.data ?? []}
              loading={contents.isLoading}
              onChanged={() => {
                queryClient.invalidateQueries({ queryKey: ["admin", "contents"] });
              }}
              onError={showError}
              onSaved={showSaved}
            />
          )}
          {view === "shares" && (
            <SharesPanel
              contents={contents.data ?? []}
              shares={shares.data ?? []}
              loading={shares.isLoading}
              onChanged={() => {
                queryClient.invalidateQueries({ queryKey: ["admin", "shares"] });
              }}
              onError={showError}
              onSaved={showSaved}
            />
          )}
        </main>
      </div>
      <Toast value={toast} onClose={() => setToast(null)} />
    </div>
  );
}

function AppSkeleton() {
  return (
    <div className="min-h-dvh px-6 py-10 lg:grid lg:grid-cols-[260px_minmax(0,1fr)]">
      <div className="hidden border-r border-line pr-8 lg:block">
        <BrandMark />
        <div className="mt-16 h-20 w-40 animate-pulse rounded-xl bg-ink/6" />
      </div>
      <div className="mx-auto w-full max-w-5xl lg:px-12">
        <div className="h-4 w-24 animate-pulse rounded-full bg-ink/6" />
        <div className="mt-4 h-12 w-72 animate-pulse rounded-xl bg-ink/6" />
        <div className="mt-12 grid gap-5 md:grid-cols-[1.25fr_.75fr]">
          <div className="h-80 animate-pulse rounded-2xl bg-ink/5" />
          <div className="h-80 animate-pulse rounded-2xl bg-ink/5" />
        </div>
      </div>
    </div>
  );
}
