import type {
  InputHTMLAttributes,
  SelectHTMLAttributes,
  TextareaHTMLAttributes,
} from "react";
import { cn } from "@/lib/cn";

const control =
  "w-full rounded-[9px] border border-line bg-paper px-3.5 py-2.5 text-[0.9rem] text-ink outline-none transition-[border-color,box-shadow,background-color] duration-200 placeholder:text-muted/70 hover:border-ink/25 focus:border-accent focus:ring-3 focus:ring-accent/12 disabled:cursor-not-allowed disabled:opacity-50";

export function Label({
  children,
  htmlFor,
  hint,
}: {
  children: React.ReactNode;
  htmlFor?: string;
  hint?: string;
}) {
  return (
    <div className="mb-2 flex items-baseline justify-between gap-3">
      <label
        htmlFor={htmlFor}
        className="text-[0.77rem] font-semibold uppercase tracking-[0.09em] text-ink-soft"
      >
        {children}
      </label>
      {hint && <span className="text-[0.72rem] text-muted">{hint}</span>}
    </div>
  );
}

export function Input({
  className,
  ...props
}: InputHTMLAttributes<HTMLInputElement>) {
  return <input className={cn(control, className)} {...props} />;
}

export function Select({
  className,
  ...props
}: SelectHTMLAttributes<HTMLSelectElement>) {
  return <select className={cn(control, "appearance-auto", className)} {...props} />;
}

export function Textarea({
  className,
  ...props
}: TextareaHTMLAttributes<HTMLTextAreaElement>) {
  return <textarea className={cn(control, "resize-y", className)} {...props} />;
}
