interface FilterChipsProps {
  options: { label: string; value: string }[];
  selected: string[];
  onToggle: (value: string) => void;
  onClear: () => void;
}

export function FilterChips({ options, selected, onToggle, onClear }: FilterChipsProps) {
  if (selected.length === 0) return null;
  return (
    <div className="flex flex-wrap items-center gap-2 mb-4">
      {options.map(opt => {
        const isActive = selected.includes(opt.value);
        return (
          <button
            key={opt.value}
            onClick={() => onToggle(opt.value)}
            className={`px-3 py-1 rounded-full text-sm border transition-colors ${
              isActive ? "bg-blue-100 text-blue-700 border-blue-300" : "bg-gray-50 text-gray-600 border-gray-200 hover:bg-gray-100"
            }`}
          >
            {opt.label}
          </button>
        );
      })}
      <button onClick={onClear} className="text-xs text-gray-400 hover:text-gray-600 ml-2">清除筛选</button>
    </div>
  );
}
