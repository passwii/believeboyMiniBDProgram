"use client";

import { useMutation } from "@tanstack/react-query";
import {
  CalendarClock,
  Check,
  Copy,
  ExternalLink,
  KeyRound,
  Link2,
  LoaderCircle,
  RotateCcw,
  ShieldOff,
} from "lucide-react";
import { useMemo, useState } from "react";

import { adminApi, type ContentItem, type ShareItem } from "@/lib/api";
import { cn } from "@/lib/cn";
import {
  defaultShareExpiry,
  formatFullDate,
  isShareActive,
} from "@/lib/format";
import { Button } from "@/components/ui/button";
import { Input, Label, Select } from "@/components/ui/field";
import { StatusDot } from "@/components/ui/status";

export function SharesPanel({
  contents,
  shares,
  loading,
  onChanged,
  onError,
  onSaved,
}: {
  contents: ContentItem[];
  shares: ShareItem[];
  loading: boolean;
  onChanged: () => void;
  onError: (error: unknown) => void;
  onSaved: (message: string) => void;
}) {
  const [contentId, setContentId] = useState(contents[0]?.id ?? "");
  const [type, setType] = useState<"PUBLIC" | "PRIVATE">("PRIVATE");
  const [label, setLabel] = useState("");
  const [pin, setPIN] = useState("");
  const [watermark, setWatermark] = useState("");
  const [expiresAt, setExpiresAt] = useState(defaultShareExpiry);
  const [newShare, setNewShare] = useState<ShareItem | null>(null);
  const [revokingId, setRevokingId] = useState<string | null>(null);
  const [resettingId, setResettingId] = useState<string | null>(null);
  const [resetPIN, setResetPIN] = useState("");

  const selectedContentId = contentId || contents[0]?.id || "";

  const create = useMutation({
    mutationFn: () =>
      adminApi.createShare({
        content_id: selectedContentId,
        type,
        label: label.trim(),
        pin,
        watermark_text: watermark.trim(),
        expires_at: new Date(expiresAt).toISOString(),
        session_ttl_seconds: 1800,
      }),
    onSuccess: (created) => {
      setNewShare(created);
      setLabel("");
      setPIN("");
      setWatermark("");
      setExpiresAt(defaultShareExpiry());
      onChanged();
      onSaved(`「${created.label}」分享已创建`);
    },
    onError,
  });

  const revoke = useMutation({
    mutationFn: adminApi.revokeShare,
    onSuccess: () => {
      setRevokingId(null);
      onChanged();
      onSaved("分享已撤销，已有会话同步失效");
    },
    onError,
  });

  const reset = useMutation({
    mutationFn: ({ id, value }: { id: string; value: string }) =>
      adminApi.resetPin(id, value),
    onSuccess: () => {
      setResettingId(null);
      setResetPIN("");
      onChanged();
      onSaved("密码已重置，旧会话已失效");
    },
    onError,
  });

  const sortedShares = useMemo(
    () =>
      [...shares].sort(
        (left, right) =>
          new Date(right.created_at).getTime() -
          new Date(left.created_at).getTime(),
      ),
    [shares],
  );

  async function copy(value: string, message: string) {
    try {
      await navigator.clipboard.writeText(value);
      onSaved(message);
    } catch {
      onError(new Error("浏览器未允许复制，请手动选择链接"));
    }
  }

  const canCreate =
    selectedContentId &&
    label.trim().length > 0 &&
    /^[0-9]{4}$/.test(pin) &&
    expiresAt.length > 0;

  return (
    <div className="grid gap-9 xl:grid-cols-[minmax(350px,.8fr)_minmax(0,1.2fr)] rise-in">
      <section className="xl:sticky xl:top-8 xl:self-start">
        <div className="border-t-2 border-ink pt-5">
          <div className="flex items-start justify-between gap-4">
            <div>
              <h2 className="font-display text-2xl">创建分享</h2>
              <p className="mt-2 text-[0.78rem] leading-6 text-muted">
                一个入口对应一个密码、期限和水印。
              </p>
            </div>
            <span className="text-[0.67rem] uppercase tracking-[0.12em] text-muted">
              Step 01
            </span>
          </div>

          <div className="mt-6 space-y-5">
            <div>
              <Label htmlFor="share-content">方案内容</Label>
              <Select
                id="share-content"
                value={selectedContentId}
                onChange={(event) => setContentId(event.target.value)}
                disabled={!contents.length}
              >
                {!contents.length && <option value="">请先上传 HTML</option>}
                {contents.map((item) => (
                  <option key={item.id} value={item.id}>
                    {item.title} · v{item.version_no}
                  </option>
                ))}
              </Select>
            </div>

            <div>
              <Label>分享类型</Label>
              <div className="grid grid-cols-2 gap-2">
                {(
                  [
                    {
                      value: "PRIVATE",
                      title: "专属分享",
                      note: "每个客户独立链接",
                    },
                    {
                      value: "PUBLIC",
                      title: "群分享",
                      note: "同一密码发给多人",
                    },
                  ] as const
                ).map((option) => (
                  <button
                    key={option.value}
                    type="button"
                    onClick={() => setType(option.value)}
                    className={cn(
                      "rounded-[10px] border px-3 py-3 text-left transition-colors",
                      type === option.value
                        ? "border-ink bg-ink text-paper"
                        : "border-line hover:border-ink/35",
                    )}
                  >
                    <span className="block text-[0.8rem] font-medium">
                      {option.title}
                    </span>
                    <span
                      className={cn(
                        "mt-1 block text-[0.66rem] leading-4",
                        type === option.value ? "text-paper/55" : "text-muted",
                      )}
                    >
                      {option.note}
                    </span>
                  </button>
                ))}
              </div>
            </div>

            <div>
              <Label htmlFor="share-label" hint="用于识别，不展示给客户">
                分享标签
              </Label>
              <Input
                id="share-label"
                value={label}
                onChange={(event) => setLabel(event.target.value)}
                placeholder="例如：王先生 / 11 月直播提案"
                maxLength={120}
              />
            </div>

            <div className="grid gap-4 sm:grid-cols-2">
              <div>
                <Label htmlFor="share-pin" hint="4 位数字">
                  访问密码
                </Label>
                <Input
                  id="share-pin"
                  inputMode="numeric"
                  autoComplete="off"
                  value={pin}
                  onChange={(event) =>
                    setPIN(event.target.value.replace(/\D/g, "").slice(0, 4))
                  }
                  placeholder="0000"
                  className="font-mono tracking-[0.35em]"
                />
              </div>
              <div>
                <Label htmlFor="share-expiry">有效期至</Label>
                <Input
                  id="share-expiry"
                  type="datetime-local"
                  value={expiresAt}
                  onChange={(event) => setExpiresAt(event.target.value)}
                />
              </div>
            </div>

            <div>
              <Label htmlFor="share-watermark" hint="留空则使用分享标签">
                水印文字
              </Label>
              <Input
                id="share-watermark"
                value={watermark}
                onChange={(event) => setWatermark(event.target.value)}
                placeholder="仅供项目沟通使用"
                maxLength={200}
              />
            </div>

            <Button
              className="w-full"
              disabled={!canCreate || create.isPending}
              onClick={() => create.mutate()}
            >
              {create.isPending ? (
                <LoaderCircle size={16} className="animate-spin" />
              ) : (
                <Link2 size={16} />
              )}
              {create.isPending ? "正在创建…" : "生成分享入口"}
            </Button>
          </div>
        </div>

        {newShare && (
          <div className="mt-7 border-t border-line pt-5">
            <p className="text-[0.67rem] uppercase tracking-[0.14em] text-success">
              已生成
            </p>
            <p className="mt-2 font-display text-2xl">{newShare.label}</p>
            <div className="mt-4 space-y-2">
              <CopyLine
                label="H5 链接"
                value={newShare.h5_url}
                onCopy={() => copy(newShare.h5_url, "H5 链接已复制")}
              />
              <CopyLine
                label="小程序路径"
                value={newShare.miniapp_path}
                onCopy={() => copy(newShare.miniapp_path, "小程序路径已复制")}
              />
            </div>
            <a
              href={newShare.h5_url}
              target="_blank"
              rel="noreferrer"
              className="mt-4 inline-flex items-center gap-2 text-[0.76rem] font-medium text-accent underline decoration-accent/30 underline-offset-4"
            >
              打开 H5 预览
              <ExternalLink size={14} />
            </a>
          </div>
        )}
      </section>

      <section>
        <div className="mb-4 flex items-end justify-between border-b border-line pb-3">
          <div>
            <h2 className="font-display text-2xl">分享记录</h2>
            <p className="mt-1 text-[0.73rem] text-muted">
              链接、状态与访问权限
            </p>
          </div>
          <span className="text-[0.72rem] text-muted">{shares.length} 条</span>
        </div>

        {loading ? (
          <div className="space-y-2 pt-2">
            {[0, 1, 2].map((item) => (
              <div key={item} className="h-28 animate-pulse rounded-xl bg-ink/5" />
            ))}
          </div>
        ) : sortedShares.length ? (
          <div className="divide-y divide-line">
            {sortedShares.map((share) => {
              const active = isShareActive(share);
              const resetting = resettingId === share.id;
              const confirmingRevoke = revokingId === share.id;
              return (
                <article key={share.id} className="py-5">
                  <div className="flex flex-wrap items-start justify-between gap-3">
                    <div className="min-w-0 flex-1">
                      <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
                        <h3 className="text-[0.92rem] font-medium">{share.label}</h3>
                        <span className="rounded-full border border-line px-2 py-0.5 text-[0.62rem] uppercase tracking-[0.08em] text-muted">
                          {share.type === "PUBLIC" ? "群分享" : "专属"}
                        </span>
                        <StatusDot
                          tone={
                            active ? "active" : share.revoked_at ? "danger" : "warning"
                          }
                          label={
                            active ? "有效" : share.revoked_at ? "已撤销" : "已过期"
                          }
                        />
                      </div>
                      <p className="mt-2 truncate text-[0.72rem] text-muted">
                        {share.content_title} · 有效期至 {formatFullDate(share.expires_at)}
                      </p>
                    </div>
                    {share.revoked_at ? (
                      <span className="text-[0.68rem] text-muted">
                        撤销于 {formatFullDate(share.revoked_at)}
                      </span>
                    ) : (
                      <span className="text-[0.68rem] text-muted">
                        {Math.round(share.session_ttl_seconds / 60)} 分钟会话
                      </span>
                    )}
                  </div>

                  <div className="mt-4 flex flex-wrap items-center gap-x-5 gap-y-2">
                    <button
                      type="button"
                      onClick={() => copy(share.h5_url, "H5 链接已复制")}
                      className="inline-flex items-center gap-1.5 text-[0.73rem] text-ink-soft hover:text-ink"
                    >
                      <Copy size={13} />
                      复制 H5
                    </button>
                    <button
                      type="button"
                      onClick={() => copy(share.miniapp_path, "小程序路径已复制")}
                      className="inline-flex items-center gap-1.5 text-[0.73rem] text-ink-soft hover:text-ink"
                    >
                      <Copy size={13} />
                      复制小程序路径
                    </button>
                    {!share.revoked_at && (
                      <>
                        <button
                          type="button"
                          onClick={() => {
                            setResettingId(resetting ? null : share.id);
                            setResetPIN("");
                          }}
                          className="inline-flex items-center gap-1.5 text-[0.73rem] text-ink-soft hover:text-ink"
                        >
                          <KeyRound size={13} />
                          重置密码
                        </button>
                        <button
                          type="button"
                          onClick={() =>
                            setRevokingId(confirmingRevoke ? null : share.id)
                          }
                          className="inline-flex items-center gap-1.5 text-[0.73rem] text-danger hover:text-danger/80"
                        >
                          <ShieldOff size={13} />
                          撤销
                        </button>
                      </>
                    )}
                  </div>

                  {resetting && (
                    <div className="mt-4 grid gap-3 rounded-[10px] bg-paper-deep/60 p-3 sm:grid-cols-[1fr_auto]">
                      <Input
                        aria-label="新的四位密码"
                        inputMode="numeric"
                        placeholder="输入新的 4 位密码"
                        value={resetPIN}
                        onChange={(event) =>
                          setResetPIN(
                            event.target.value.replace(/\D/g, "").slice(0, 4),
                          )
                        }
                        className="font-mono tracking-[0.3em]"
                      />
                      <Button
                        variant="secondary"
                        disabled={!/^[0-9]{4}$/.test(resetPIN) || reset.isPending}
                        onClick={() =>
                          reset.mutate({ id: share.id, value: resetPIN })
                        }
                      >
                        <RotateCcw size={14} />
                        确认重置
                      </Button>
                    </div>
                  )}

                  {confirmingRevoke && (
                    <div className="mt-4 flex flex-wrap items-center justify-between gap-3 rounded-[10px] border border-danger/20 bg-danger/[0.045] p-3">
                      <p className="text-[0.75rem] text-danger">
                        撤销后，所有已打开的页面也会在心跳检查时失效。
                      </p>
                      <div className="flex gap-2">
                        <Button
                          variant="ghost"
                          className="min-h-9 px-3 text-[0.73rem]"
                          onClick={() => setRevokingId(null)}
                        >
                          取消
                        </Button>
                        <Button
                          variant="danger"
                          className="min-h-9 px-3 text-[0.73rem]"
                          disabled={revoke.isPending}
                          onClick={() => revoke.mutate(share.id)}
                        >
                          {revoke.isPending ? "撤销中…" : "确认撤销"}
                        </Button>
                      </div>
                    </div>
                  )}
                </article>
              );
            })}
          </div>
        ) : (
          <div className="flex min-h-64 flex-col items-start justify-center border-b border-line">
            <CalendarClock size={28} strokeWidth={1.4} className="text-muted" />
            <p className="mt-4 font-display text-2xl">还没有分享入口</p>
            <p className="mt-2 max-w-sm text-[0.8rem] leading-6 text-muted">
              创建第一条分享后，这里会显示有效期、状态和可复制的访问入口。
            </p>
          </div>
        )}
      </section>
    </div>
  );
}

function CopyLine({
  label,
  value,
  onCopy,
}: {
  label: string;
  value: string;
  onCopy: () => void;
}) {
  return (
    <div className="flex items-center gap-2 border-b border-line pb-2">
      <div className="min-w-0 flex-1">
        <p className="text-[0.64rem] uppercase tracking-[0.1em] text-muted">
          {label}
        </p>
        <p className="mt-1 truncate font-mono text-[0.68rem] text-ink-soft">
          {value}
        </p>
      </div>
      <button
        type="button"
        onClick={onCopy}
        className="grid h-8 w-8 shrink-0 place-items-center rounded-full text-muted hover:bg-ink/5 hover:text-ink"
        aria-label={`复制${label}`}
      >
        <Check size={14} />
      </button>
    </div>
  );
}
