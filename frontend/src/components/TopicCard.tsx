import { ReactNode } from "react";

interface TopicCardProps {
  icon: string;
  title: string;
  children: ReactNode;
}

export function TopicCard({ icon, title, children }: TopicCardProps) {
  return (
    <div className="bg-white border border-gray-200 rounded-xl p-5 mb-4 shadow-sm">
      <h3 className="flex items-center gap-2 text-base font-semibold text-gray-800 mb-3">
        <span>{icon}</span>
        {title}
      </h3>
      <div>{children}</div>
    </div>
  );
}
