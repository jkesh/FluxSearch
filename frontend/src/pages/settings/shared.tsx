export function Field({
  label,
  hint,
  children,
}: {
  label: string
  hint?: string
  children: React.ReactNode
}) {
  return (
    <label className="form-control w-full">
      <div className="label py-0 pb-1">
        <span className="label-text text-xs font-medium text-base-content/70">{label}</span>
      </div>
      {children}
      {hint && (
        <div className="label py-0 pt-1">
          <span className="label-text-alt text-base-content/40">{hint}</span>
        </div>
      )}
    </label>
  )
}

export function Section({ title, desc, children }: { title: string; desc?: string; children: React.ReactNode }) {
  return (
    <section className="surface overflow-hidden">
      <div className="border-b border-base-300 bg-base-200/40 px-5 py-3.5">
        <h2 className="text-sm font-semibold">{title}</h2>
        {desc && <p className="mt-0.5 text-xs leading-relaxed text-base-content/50">{desc}</p>}
      </div>
      <div className="grid gap-4 p-5 sm:grid-cols-2">{children}</div>
    </section>
  )
}

export function Toggle({
  checked,
  onChange,
  label,
  hint,
}: {
  checked: boolean
  onChange: (v: boolean) => void
  label: string
  hint?: string
}) {
  return (
    <label className="flex cursor-pointer items-start gap-3 rounded-xl border border-base-300 px-4 py-3.5 transition-colors hover:border-primary/30 hover:bg-base-200/40 sm:col-span-2">
      <input
        type="checkbox"
        className="toggle toggle-sm toggle-primary mt-0.5"
        checked={checked}
        onChange={(e) => onChange(e.target.checked)}
      />
      <span>
        <span className="text-sm font-medium">{label}</span>
        {hint && <span className="mt-0.5 block text-xs leading-relaxed text-base-content/50">{hint}</span>}
      </span>
    </label>
  )
}
