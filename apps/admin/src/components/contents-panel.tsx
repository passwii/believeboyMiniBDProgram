"use client";

import { useMutation } from "@tanstack/react-query";
import {
  AlertTriangle,
  Check,
  FileCode2,
  FileUp,
  LoaderCircle,
} from "lucide-react";
import { useRef, useState } from "react";

import { adminApi, type ContentItem } from "@/lib/api";
import { cn } from "@/lib/cn";
import { formatBytes, formatFullDate } from "@/lib/format";
import { Button } from "@/components/ui/button";
import { StatusDot } from "@/components/ui/status";

export function ContentsPanel({
  contents,
  loading,
  onChanged,
  onError,
  onSaved,
}: {
  contents: ContentItem[];
  loading: boolean;
  onChanged: () => void;
  onError: (error: unknown) => void;
  onSaved: (message: string) => void;
}) {
  const [file, setFile] = useState<File | null>(null);
  const [dragging, setDragging] = useState(false);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const upload = useMutation({
    mutationFn: adminApi.upload,
    onSuccess: (item) => {
      setFile(null);
      setSelectedId(item.id);
      onChanged();
      onSaved(`「${item.title}」已保存为版本 ${item.version_no}`);
    },
    onError,
  });

  function acceptFile(next: File | undefined) {
    if (!next) return;
    if (!next.name.toLowerCase().endsWith(".html")) {
      onError(new Error("仅支持单个 .html 文件"));
      return;
    }
    if (next.size > 20 * 1024 * 1024) {
      onError(new Error("HTML 文件不能超过 20 MiB"));
      return;
    }
    setFile(next);
  }

  return (
    <div className="grid gap-8 xl:grid-cols-[minmax(0,1.15fr)_minmax(340px,.85fr)] rise-in">
      <section>
        <div className="mb-4 flex items-end justify-between border-b border-line pb-3">
          <div>
            <h2 className="font-display text-2xl">内容库</h2>
            <p className="mt-1 text-[0.73rem] text-muted">每次上传都会生成不可变版本</p>
          </div>
          <span className="text-[0.72rem] text-muted">{contents.length} 份</span>
        </div>
        {loading ? (
          <div className="space-y-2 pt-2">
            {[0, 1, 2].map((item) => (
              <div key={item} className="h-20 animate-pulse rounded-xl bg-ink/5" />
            ))}
          </div>
        ) : contents.length ? (
          <div className="space-y-2">
            {contents.map((item) => {
              const selected = item.id === selectedId;
              return (
                <button
                  key={item.id}
                  type="button"
                  onClick={() => setSelectedId(selected ? null : item.id)}
                  className={cn(
                    "w-full border-b border-line px-1 py-4 text-left transition-colors hover:bg-paper-deep/45",
                    selected && "bg-paper-deep/70",
                  )}
                >
                  <div className="flex items-start gap-3">
                    <span className="mt-0.5 grid h-9 w-9 shrink-0 place-items-center rounded-full bg-paper-deep text-ink-soft">
                      <FileCode2 size={16} strokeWidth={1.7} />
                    </span>
                    <span className="min-w-0 flex-1">
                      <span className="flex flex-wrap items-center gap-x-3 gap-y-1">
                        <span className="truncate text-[0.88rem] font-medium">
                          {item.title}
                        </span>
                        <StatusDot
                          tone={item.analysis.warnings.length ? "warning" : "active"}
                          label={
                            item.analysis.warnings.length
                              ? `${item.analysis.warnings.length} 条提醒`
                              : "解析通过"
                          }
                        />
                      </span>
                      <span className="mt-1.5 flex flex-wrap gap-x-4 gap-y-1 text-[0.69rem] text-muted">
                        <span>版本 {item.version_no}</span>
                        <span>{formatBytes(item.byte_size)}</span>
                        <span>{formatFullDate(item.created_at)}</span>
                      </span>
                    </span>
                  </div>
                  {selected && (
                    <div className="ml-12 mt-4 grid gap-3 text-[0.73rem] leading-5 text-ink-soft sm:grid-cols-2">
                      <InfoLine
                        label="Viewport"
                        value={item.analysis.viewport_present ? "已声明" : "缺失"}
                      />
                      <InfoLine
                        label="本地脚本"
                        value={`${item.analysis.script_count} 段`}
                      />
                      <div className="sm:col-span-2">
                        <p className="text-[0.65rem] uppercase tracking-[0.1em] text-muted">
                          SHA-256
                        </p>
                        <p className="mt-1 break-all font-mono text-[0.68rem] text-ink-soft">
                          {item.sha256}
                        </p>
                      </div>
                      {item.analysis.warnings.map((warning) => (
                        <p
                          key={warning}
                          className="flex items-start gap-2 sm:col-span-2"
                        >
                          <AlertTriangle
                            size={14}
                            className="mt-0.5 shrink-0 text-warning"
                          />
                          {warning}
                        </p>
                      ))}
                    </div>
                  )}
                </button>
              );
            })}
          </div>
        ) : (
          <div className="flex min-h-56 flex-col items-start justify-center border-b border-line">
            <FileCode2 size={27} strokeWidth={1.4} className="text-muted" />
            <p className="mt-4 font-display text-2xl">内容库还是空的</p>
            <p className="mt-2 max-w-sm text-[0.8rem] leading-6 text-muted">
              右侧上传第一份自包含 HTML。图片、样式和脚本都需要包含在同一个文件内。
            </p>
          </div>
        )}
      </section>

      <section className="xl:sticky xl:top-8 xl:self-start">
        <div className="border-t-2 border-ink pt-5">
          <h2 className="font-display text-2xl">上传新版本</h2>
          <p className="mt-2 text-[0.78rem] leading-6 text-muted">
            文件上限 20 MiB。系统会检查外部资源、标题和 viewport。
          </p>

          <button
            type="button"
            onClick={() => inputRef.current?.click()}
            onDragOver={(event) => {
              event.preventDefault();
              setDragging(true);
            }}
            onDragLeave={() => setDragging(false)}
            onDrop={(event) => {
              event.preventDefault();
              setDragging(false);
              acceptFile(event.dataTransfer.files[0]);
            }}
            className={cn(
              "mt-6 block w-full rounded-[14px] border border-dashed border-line bg-paper-deep/35 px-5 py-8 text-left transition-colors hover:border-ink/35 hover:bg-paper-deep/60",
              dragging && "border-accent bg-accent-soft/45",
            )}
          >
            <span className="flex items-center gap-3">
              <span className="grid h-10 w-10 place-items-center rounded-full border border-line bg-paper">
                <FileUp size={18} strokeWidth={1.7} />
              </span>
              <span>
                <span className="block text-[0.86rem] font-medium">
                  {file ? file.name : "选择或拖入 HTML 文件"}
                </span>
                <span className="mt-1 block text-[0.7rem] text-muted">
                  {file ? formatBytes(file.size) : "单个 .html 文件"}
                </span>
              </span>
            </span>
          </button>
          <input
            ref={inputRef}
            type="file"
            accept=".html,text/html"
            className="hidden"
            onChange={(event) => acceptFile(event.target.files?.[0])}
          />

          <Button
            className="mt-4 w-full"
            disabled={!file || upload.isPending}
            onClick={() => file && upload.mutate(file)}
          >
            {upload.isPending ? (
              <LoaderCircle size={16} className="animate-spin" />
            ) : (
              <Check size={16} />
            )}
            {upload.isPending ? "正在解析并保存…" : "上传并生成版本"}
          </Button>
        </div>
      </section>
    </div>
  );
}

function InfoLine({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <p className="text-[0.65rem] uppercase tracking-[0.1em] text-muted">{label}</p>
      <p className="mt-1">{value}</p>
    </div>
  );
}
