import { cn } from "@/lib/utils";

type BadgeVariant = "success" | "warning" | "danger" | "neutral" | "brand" | "info" | "purple";

interface StatusBadgeProps {
  variant: BadgeVariant;
  children: React.ReactNode;
  className?: string;
}

const variantStyles: Record<BadgeVariant, string> = {
  success: "bg-success-bg text-success-foreground",
  warning: "bg-warning-bg text-warning-foreground",
  danger: "bg-danger-bg text-danger-foreground",
  neutral: "bg-muted text-muted-foreground",
  brand: "bg-accent text-accent-foreground",
  info: "bg-blue-50 text-blue-700",
  purple: "bg-purple-50 text-purple-700",
};

export function StatusBadge({ variant, children, className }: StatusBadgeProps) {
  return (
    <span className={cn("inline-flex items-center rounded-full px-2.5 py-0.5 text-badge", variantStyles[variant], className)}>
      {children}
    </span>
  );
}
