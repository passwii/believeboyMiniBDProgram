export function BrandMark({ compact = false }: { compact?: boolean }) {
  return (
    <div className="flex items-center gap-3">
      <svg
        aria-hidden="true"
        viewBox="0 0 32 32"
        className="h-8 w-8 shrink-0"
        fill="none"
      >
        <path
          d="M5 5h10v10H5zM17 17h10v10H17z"
          stroke="currentColor"
          strokeWidth="1.5"
        />
        <path d="M17 5h10M5 27h10" stroke="currentColor" strokeWidth="1.5" />
        <circle cx="16" cy="16" r="2.5" fill="var(--accent)" />
      </svg>
      {!compact && (
        <div className="leading-none">
          <p className="font-display text-[1.15rem] tracking-[-0.02em]">
            BelieveBoy
          </p>
          <p className="mt-1.5 text-[0.63rem] uppercase tracking-[0.18em] text-muted">
            Proposal Desk
          </p>
        </div>
      )}
    </div>
  );
}
