interface DifficultyBadgeProps {
  difficulty: "beginner" | "intermediate" | "advanced" | "easy" | "medium" | "hard";
}

const STYLES: Record<string, string> = {
  beginner: "bg-green-100 text-green-700 border-green-300",
  easy: "bg-green-100 text-green-700 border-green-300",
  intermediate: "bg-yellow-100 text-yellow-700 border-yellow-300",
  medium: "bg-yellow-100 text-yellow-700 border-yellow-300",
  advanced: "bg-red-100 text-red-700 border-red-300",
  hard: "bg-red-100 text-red-700 border-red-300",
};

const LABELS: Record<string, string> = {
  beginner: "入门",
  easy: "简单",
  intermediate: "进阶",
  medium: "中等",
  advanced: "高级",
  hard: "困难",
};

export function DifficultyBadge({ difficulty }: DifficultyBadgeProps) {
  return (
    <span className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium border ${STYLES[difficulty] ?? ""}`}>
      {LABELS[difficulty] ?? difficulty}
    </span>
  );
}
