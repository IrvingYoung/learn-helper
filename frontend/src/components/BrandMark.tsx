interface BrandMarkProps {
  size?: number
  showText?: boolean
}

export function BrandMark({ size = 28, showText = true }: BrandMarkProps) {
  return (
    <div className="flex items-center gap-2.5 select-none">
      <div
        className="relative inline-flex items-end justify-center font-display font-bold text-th-accent leading-none"
        style={{ width: size, height: size, fontSize: size * 0.78 }}
        aria-hidden="true"
      >
        <span style={{ marginBottom: -size * 0.08 }}>L</span>
        <span
          className="absolute rounded-full bg-th-accent-light"
          style={{
            width: size * 0.18,
            height: size * 0.18,
            top: size * 0.08,
            right: size * 0.12,
          }}
        />
      </div>
      {showText && (
        <div className="flex items-baseline gap-1.5">
          <span className="font-display text-[17px] font-semibold text-th-text-primary tracking-tight">
            LLM Wiki
          </span>
          <span className="text-[10px] uppercase tracking-[0.18em] text-th-text-muted font-medium hidden sm:inline">
            beta
          </span>
        </div>
      )}
    </div>
  )
}
