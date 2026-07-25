import { cn } from "@/lib/utils";

interface StatCardProps {
  label: string;
  value: string;
  subInfo?: string;
  trend?: "up" | "down";
}

export function StatCard({ label, value, subInfo, trend }: StatCardProps) {
  return (
    <div className="rounded-xl border border-border bg-card p-5">
      <p className="text-caption mb-1">{label}</p>
      <p className="text-[28px] font-semibold text-primary">{value}</p>
      {subInfo && (
        <p className="text-caption mt-1 flex items-center gap-1">
          {trend === "up" && <span className="text-success">↑</span>}
          {trend === "down" && <span className="text-danger">↓</span>}
          {subInfo}
        </p>
      )}
    </div>
  );
}
