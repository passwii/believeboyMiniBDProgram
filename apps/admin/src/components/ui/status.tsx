import { cn } from "@/lib/cn";

export function StatusDot({
  tone = "neutral",
  label,
}: {
  tone?: "active" | "warning" | "danger" | "neutral";
  label: string;
}) {
  const tones = {
    active: "bg-success",
    warning: "bg-warning",
    danger: "bg-danger",
    neutral: "bg-muted",
  };
  return (
    <span className="inline-flex items-center gap-2 text-[0.78rem] text-ink-soft">
      <span className={cn("h-1.5 w-1.5 rounded-full", tones[tone])} />
      {label}
    </span>
  );
}
