"use client";

import { X } from "lucide-react";
import { useEffect } from "react";

import { cn } from "@/lib/cn";

export type ToastState = {
  tone: "success" | "error";
  message: string;
};

export function Toast({
  value,
  onClose,
}: {
  value: ToastState | null;
  onClose: () => void;
}) {
  useEffect(() => {
    if (!value) return;
    const timeout = window.setTimeout(onClose, 4200);
    return () => window.clearTimeout(timeout);
  }, [onClose, value]);

  if (!value) return null;
  return (
    <div
      role="status"
      className={cn(
        "fixed bottom-5 left-1/2 z-50 flex w-[calc(100%-2rem)] max-w-md -translate-x-1/2 items-start gap-3 rounded-xl border px-4 py-3 text-[0.82rem] shadow-[0_14px_44px_rgba(42,35,25,.13)] rise-in",
        value.tone === "success"
          ? "border-success/20 bg-[oklch(0.94_0.03_145)] text-ink"
          : "border-danger/20 bg-[oklch(0.95_0.03_28)] text-ink",
      )}
    >
      <span
        className={cn(
          "mt-1 h-2 w-2 shrink-0 rounded-full",
          value.tone === "success" ? "bg-success" : "bg-danger",
        )}
      />
      <p className="flex-1 leading-5">{value.message}</p>
      <button
        type="button"
        aria-label="关闭提示"
        onClick={onClose}
        className="rounded p-1 text-muted hover:bg-ink/5 hover:text-ink"
      >
        <X size={14} />
      </button>
    </div>
  );
}
