type Status = "not_started" | "in_progress" | "mastered";

interface StatusIconProps {
  status: Status;
  size?: "sm" | "md";
}

const DOT_STYLES: Record<Status, string> = {
  not_started: "bg-gray-300",
  in_progress: "bg-yellow-400",
  mastered: "bg-green-500",
};

const CHECK_STYLES: Record<Status, string> = {
  not_started: "border-gray-300 text-gray-300",
  in_progress: "border-yellow-400 text-yellow-400",
  mastered: "border-green-500 text-green-500 bg-green-50",
};

export function StatusIcon({ status, size = "sm" }: StatusIconProps) {
  const dim = size === "sm" ? "w-2 h-2" : "w-5 h-5";
  if (size === "sm") {
    return <span className={`inline-block rounded-full ${dim} ${DOT_STYLES[status]}`} />;
  }
  return (
    <span className={`inline-flex items-center justify-center rounded-full border-2 ${dim} ${CHECK_STYLES[status]}`}>
      {status === "mastered" && <svg className="w-3 h-3" fill="currentColor" viewBox="0 0 20 20"><path fillRule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clipRule="evenodd" /></svg>}
    </span>
  );
}
