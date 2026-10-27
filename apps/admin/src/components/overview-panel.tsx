"use client";

import {
  ArrowUpRight,
  FileText,
  Link2,
  Plus,
  ShieldAlert,
} from "lucide-react";

import type { ContentItem, ShareItem } from "@/lib/api";
import { cn } from "@/lib/cn";
import { formatDate, isShareActive } from "@/lib/format";
import { Button } from "@/components/ui/button";
import { StatusDot } from "@/components/ui/status";

type View = "overview" | "contents" | "shares";

export function OverviewPanel({
  contents,
  shares,
  loading,
  onNavigate,
}: {
  contents: ContentItem[];
  shares: ShareItem[];
  loading: boolean;
  onNavigate: (view: View) => void;
}) {
  const activeShares = shares.filter(isShareActive);
  const recentContents = contents.slice(0, 4);
  const recentShares = shares.slice(0, 5);
  const warningCount = contents.reduce(
    (total, item) => total + item.analysis.warnings.length,
    0,
  );

  return (
    <div className="space-y-8 rise-in">
      <section className="grid overflow-hidden rounded-[18px] border border-line bg-paper-deep/45 lg:grid-cols-[1.2fr_.8fr]">
        <div className="p-6 sm:p-8 lg:p-10">
          <p className="max-w-xl font-display text-[clamp(2rem,4vw,3.6rem)] leading-[1.03] tracking-[-0.035em]">
            内容准备好之后，
            <br />
            给每个客户一条
            <span className="text-accent">合适的入口</span>。
          </p>
          <div className="mt-8 flex flex-wrap gap-3">
            <Button onClick={() => onNavigate("contents")}>
              <Plus size={16} />
              上传新方案
            </Button>
            <Button variant="secondary" onClick={() => onNavigate("shares")}>
              <Link2 size={16} />
              创建分享
            </Button>
          </div>
        </div>
        <div className="relative min-h-56 border-t border-line bg-ink p-7 text-paper lg:border-l lg:border-t-0 lg:p-8">
          <div className="absolute right-6 top-5 h-28 w-28 rounded-full border border-paper/10" />
          <div className="absolute right-12 top-11 h-28 w-28 rounded-full border border-paper/[0.06]" />
          <p className="text-[0.7rem] uppercase tracking-[0.17em] text-paper/50">
            Current state
          </p>
          <div className="mt-8 grid grid-cols-2 gap-6">
            <div>
              <p className="font-display text-4xl">{activeShares.length}</p>
              <p className="mt-2 text-[0.75rem] text-paper/60">个有效分享</p>
            </div>
            <div>
              <p className="font-display text-4xl">{contents.length}</p>
              <p className="mt-2 text-[0.75rem] text-paper/60">份方案内容</p>
            </div>
          </div>
          <div className="mt-6 flex items-center gap-2 text-[0.73rem] text-paper/60">
            <ShieldAlert size={15} />
            {warningCount > 0
              ? `${warningCount} 条适配或解析提醒`
              : "当前内容没有解析提醒"}
          </div>
        </div>
      </section>

      <div className="grid gap-8 xl:grid-cols-[.9fr_1.1fr]">
        <section>
          <div className="mb-3 flex items-end justify-between border-b border-line pb-3">
            <h2 className="font-display text-2xl">最近内容</h2>
            <button
              type="button"
              onClick={() => onNavigate("contents")}
              className="text-[0.76rem] text-muted hover:text-ink"
            >
              查看全部
            </button>
          </div>
          {loading ? (
            <RowsSkeleton />
          ) : recentContents.length ? (
            <div className="divide-y divide-line">
              {recentContents.map((item) => (
                <button
                  key={item.id}
                  type="button"
                  onClick={() => onNavigate("contents")}
                  className="group grid w-full grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-3 py-4 text-left"
                >
                  <span className="grid h-9 w-9 place-items-center rounded-full bg-paper-deep text-ink-soft">
                    <FileText size={16} strokeWidth={1.7} />
                  </span>
                  <span className="min-w-0">
                    <span className="block truncate text-[0.87rem] font-medium">
                      {item.title}
                    </span>
                    <span className="mt-1 block text-[0.69rem] text-muted">
                      版本 {item.version_no} · {formatDate(item.created_at)}
                    </span>
                  </span>
                  <ArrowUpRight
                    size={16}
                    className="text-muted opacity-0 transition-opacity group-hover:opacity-100"
                  />
                </button>
              ))}
            </div>
          ) : (
            <EmptyNote
              icon={FileText}
              title="还没有上传内容"
              description="先上传一份单一 HTML 文件，系统会生成独立版本。"
              action="去上传"
              onAction={() => onNavigate("contents")}
            />
          )}
        </section>

        <section>
          <div className="mb-3 flex items-end justify-between border-b border-line pb-3">
            <h2 className="font-display text-2xl">最近分享</h2>
            <button
              type="button"
              onClick={() => onNavigate("shares")}
              className="text-[0.76rem] text-muted hover:text-ink"
            >
              管理分享
            </button>
          </div>
          {loading ? (
            <RowsSkeleton />
          ) : recentShares.length ? (
            <div className="divide-y divide-line">
              {recentShares.map((share) => {
                const active = isShareActive(share);
                return (
                  <button
                    key={share.id}
                    type="button"
                    onClick={() => onNavigate("shares")}
                    className="group grid w-full grid-cols-[minmax(0,1fr)_auto] items-center gap-4 py-4 text-left"
                  >
                    <span className="min-w-0">
                      <span className="block truncate text-[0.87rem] font-medium">
                        {share.label}
                      </span>
                      <span className="mt-1 block truncate text-[0.69rem] text-muted">
                        {share.content_title} ·{" "}
                        {share.type === "PUBLIC" ? "群分享" : "专属分享"}
                      </span>
                    </span>
                    <StatusDot
                      tone={active ? "active" : share.revoked_at ? "danger" : "warning"}
                      label={active ? "有效" : share.revoked_at ? "已撤销" : "已过期"}
                    />
                  </button>
                );
              })}
            </div>
          ) : (
            <EmptyNote
              icon={Link2}
              title="还没有分享链接"
              description="选择一份方案，设置四位密码和期限后创建分享。"
              action="创建分享"
              onAction={() => onNavigate("shares")}
            />
          )}
        </section>
      </div>
    </div>
  );
}

function RowsSkeleton() {
  return (
    <div className="space-y-4 pt-5">
      {[0, 1, 2].map((item) => (
        <div key={item} className="flex items-center gap-3">
          <div className="h-9 w-9 animate-pulse rounded-full bg-ink/6" />
          <div className="flex-1 space-y-2">
            <div className="h-3 w-2/3 animate-pulse rounded-full bg-ink/6" />
            <div className="h-2.5 w-1/3 animate-pulse rounded-full bg-ink/5" />
          </div>
        </div>
      ))}
    </div>
  );
}

function EmptyNote({
  icon: Icon,
  title,
  description,
  action,
  onAction,
}: {
  icon: typeof FileText;
  title: string;
  description: string;
  action: string;
  onAction: () => void;
}) {
  return (
    <div className={cn("flex items-start gap-4 py-7")}>
      <span className="grid h-11 w-11 shrink-0 place-items-center rounded-full border border-line text-ink-soft">
        <Icon size={18} strokeWidth={1.6} />
      </span>
      <div>
        <p className="text-[0.9rem] font-medium">{title}</p>
        <p className="mt-1 max-w-sm text-[0.77rem] leading-5 text-muted">
          {description}
        </p>
        <button
          type="button"
          onClick={onAction}
          className="mt-3 text-[0.77rem] font-medium text-accent underline decoration-accent/30 underline-offset-4 hover:decoration-accent"
        >
          {action}
        </button>
      </div>
    </div>
  );
}
