interface ProgressBarProps {
  completed: number;
  total: number;
  label?: string;
}

export function ProgressBar({ completed, total, label }: ProgressBarProps) {
  const pct = total > 0 ? Math.round((completed / total) * 100) : 0;
  return (
    <div className="w-full">
      {label && <div className="flex justify-between text-xs text-gray-500 mb-1"><span>{label}</span><span>{completed}/{total}</span></div>}
      <div className="w-full bg-gray-200 rounded-full h-2">
        <div className="bg-blue-500 rounded-full h-2 transition-all" style={{ width: `${pct}%` }} />
      </div>
    </div>
  );
}
