import type { ButtonHTMLAttributes } from "react";
import { cn } from "@/lib/cn";

type Variant = "primary" | "secondary" | "ghost" | "danger";

const variants: Record<Variant, string> = {
  primary:
    "bg-ink text-paper hover:bg-ink-soft disabled:bg-ink/35 disabled:text-paper/70",
  secondary:
    "border border-line bg-paper text-ink hover:border-ink/45 hover:bg-paper-deep disabled:opacity-45",
  ghost:
    "text-ink-soft hover:bg-ink/[0.055] hover:text-ink disabled:opacity-40",
  danger:
    "border border-danger/30 bg-danger/8 text-danger hover:bg-danger/14 disabled:opacity-40",
};

export function Button({
  className,
  variant = "primary",
  type = "button",
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: Variant }) {
  return (
    <button
      type={type}
      className={cn(
        "inline-flex min-h-11 items-center justify-center gap-2 rounded-[10px] px-4 text-[0.875rem] font-medium transition-[background-color,border-color,color,transform] duration-200 active:translate-y-px disabled:cursor-not-allowed",
        variants[variant],
        className,
      )}
      {...props}
    />
  );
}
