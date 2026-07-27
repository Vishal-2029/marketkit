import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { PageHeader } from "@/components/PageHeader";
import { StatusBadge } from "@/components/StatusBadge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { Search, Download, Eye, X } from "lucide-react";
import { toast } from "sonner";
import {
  marketPlanPaymentsService,
  MarketPlanPayment,
} from "@/services/marketPlanPayments";

const statusVariant = (s: string) => {
  if (s === "ACTIVE") return "success" as const;
  if (s === "CANCELLED") return "danger" as const;
  if (s === "EXPIRED") return "warning" as const;
  if (s === "PENDING") return "neutral" as const;
  return "neutral" as const;
};

const statusLabel = (s: string) =>
  s.charAt(0) + s.slice(1).toLowerCase();

function fmt(paise: number) {
  return "₹" + (paise / 100).toLocaleString("en-IN");
}

export default function MarketPlanPaymentsPage() {
  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [page, setPage] = useState(1);
  const [detail, setDetail] = useState<MarketPlanPayment | null>(null);

  const { data, isLoading } = useQuery({
    queryKey: ["market-plan-payments", page, search, statusFilter],
    queryFn: () =>
      marketPlanPaymentsService.list({
        page,
        limit: 20,
        search: search || undefined,
        status: statusFilter || undefined,
      }),
  });

  const payments: MarketPlanPayment[] = data?.data ?? [];
  const meta = data?.meta;

  const exportCSV = () => {
    const headers = [
      "User",
      "Email",
      "Plan",
      "Amount",
      "Gateway",
      "Status",
      "Paid At",
      "Expiry",
      "Razorpay Payment ID",
    ];
    const lines = [
      headers.join(","),
      ...payments.map((p) =>
        [
          p.user?.name ?? "",
          p.user?.email ?? "",
          p.plan?.name ?? "",
          fmt(p.amount_in_paise),
          p.provider,
          p.status,
          p.paid_at ? new Date(p.paid_at).toLocaleString() : "",
          p.expiry_date ? new Date(p.expiry_date).toLocaleString() : "",
          p.provider_payment_id ?? "",
        ]
          .map((v) => `"${String(v).replace(/"/g, '""')}"`)
          .join(",")
      ),
    ];
    const blob = new Blob([lines.join("\n")], { type: "text/csv" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = "market-plan-payments.csv";
    a.click();
    URL.revokeObjectURL(url);
    toast.success("CSV exported.");
  };

  return (
    <div>
      <PageHeader
        title="Plan Payments"
        subtitle="Product Market plan subscription transactions"
      >
        <Button variant="outline" onClick={exportCSV}>
          <Download className="h-4 w-4" /> Export CSV
        </Button>
      </PageHeader>

      <div className="flex items-center gap-3 mb-6">
        <div className="relative flex-1 max-w-sm">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input
            placeholder="Search by name or email..."
            value={search}
            onChange={(e) => {
              setSearch(e.target.value);
              setPage(1);
            }}
            className="pl-9 bg-muted border-0"
          />
        </div>
        <select
          className="h-10 rounded-lg border border-border bg-card px-3 text-sm"
          value={statusFilter}
          onChange={(e) => {
            setStatusFilter(e.target.value);
            setPage(1);
          }}
        >
          <option value="">All Status</option>
          <option value="ACTIVE">Active</option>
          <option value="EXPIRED">Expired</option>
          <option value="CANCELLED">Cancelled</option>
          <option value="PENDING">Pending</option>
        </select>
      </div>

      <div className="rounded-xl border border-border bg-card overflow-hidden">
        <table className="w-full">
          <thead>
            <tr className="bg-table-header">
              <th className="text-table-header text-left px-4 py-3">User</th>
              <th className="text-table-header text-left px-4 py-3 hidden md:table-cell">
                Plan
              </th>
              <th className="text-table-header text-left px-4 py-3">Amount</th>
              <th className="text-table-header text-left px-4 py-3 hidden md:table-cell">
                Gateway
              </th>
              <th className="text-table-header text-left px-4 py-3">Status</th>
              <th className="text-table-header text-left px-4 py-3 hidden lg:table-cell">
                Paid
              </th>
              <th className="text-table-header text-left px-4 py-3">Actions</th>
            </tr>
          </thead>
          <tbody>
            {isLoading
              ? Array(5)
                  .fill(0)
                  .map((_, i) => (
                    <tr key={i}>
                      <td colSpan={7} className="px-4 py-3">
                        <Skeleton className="h-8" />
                      </td>
                    </tr>
                  ))
              : payments.length === 0
                ? (
                  <tr>
                    <td
                      colSpan={7}
                      className="px-4 py-10 text-center text-sm text-muted-foreground"
                    >
                      No plan payments found.
                    </td>
                  </tr>
                )
                : payments.map((p) => (
                    <tr
                      key={p.id}
                      className="border-b border-border last:border-0 hover:bg-table-hover transition-colors cursor-pointer"
                      onClick={() => setDetail(p)}
                    >
                      <td className="px-4 py-3">
                        <div className="flex items-center gap-3">
                          <div className="w-8 h-8 rounded-full bg-accent flex items-center justify-center shrink-0">
                            <span className="text-xs font-semibold text-accent-foreground">
                              {(p.user?.name ?? "?")[0]}
                            </span>
                          </div>
                          <div>
                            <p className="text-sm font-medium text-foreground">
                              {p.user?.name ?? "—"}
                            </p>
                            <p className="text-caption">{p.user?.email ?? ""}</p>
                          </div>
                        </div>
                      </td>
                      <td className="px-4 py-3 hidden md:table-cell">
                        <StatusBadge variant="brand">
                          {p.plan?.name ?? "—"}
                        </StatusBadge>
                      </td>
                      <td className="px-4 py-3 text-sm font-semibold text-foreground">
                        {fmt(p.amount_in_paise)}
                      </td>
                      <td className="px-4 py-3 hidden md:table-cell">
                        <StatusBadge
                          variant={
                            p.provider === "WALLET" ? "neutral" : "brand"
                          }
                        >
                          {p.provider}
                        </StatusBadge>
                      </td>
                      <td className="px-4 py-3">
                        <StatusBadge variant={statusVariant(p.status)}>
                          {statusLabel(p.status)}
                        </StatusBadge>
                      </td>
                      <td className="px-4 py-3 text-sm text-muted-foreground hidden lg:table-cell">
                        {p.paid_at
                          ? new Date(p.paid_at).toLocaleString()
                          : "—"}
                      </td>
                      <td
                        className="px-4 py-3"
                        onClick={(e) => e.stopPropagation()}
                      >
                        <Button
                          variant="ghost"
                          size="sm"
                          className="h-8 px-2 text-xs gap-1"
                          onClick={() => setDetail(p)}
                        >
                          <Eye className="h-3.5 w-3.5" /> View
                        </Button>
                      </td>
                    </tr>
                  ))}
          </tbody>
        </table>
      </div>

      {meta && meta.pages > 1 && (
        <div className="flex items-center justify-between mt-4">
          <p className="text-sm text-muted-foreground">
            Page {meta.page} of {meta.pages} · {meta.total} total
          </p>
          <div className="flex gap-2">
            <Button
              variant="outline"
              size="sm"
              disabled={page <= 1}
              onClick={() => setPage((p) => p - 1)}
            >
              Previous
            </Button>
            <Button
              variant="outline"
              size="sm"
              disabled={page >= meta.pages}
              onClick={() => setPage((p) => p + 1)}
            >
              Next
            </Button>
          </div>
        </div>
      )}

      {detail && (
        <div className="fixed inset-0 z-50 flex justify-end bg-black/40">
          <div className="w-full max-w-md h-full bg-card border-l border-border shadow-xl overflow-y-auto">
            <div className="flex items-center justify-between px-5 py-4 border-b border-border">
              <h2 className="text-lg font-semibold">Plan payment</h2>
              <Button
                variant="ghost"
                size="icon"
                onClick={() => setDetail(null)}
              >
                <X className="h-4 w-4" />
              </Button>
            </div>
            <div className="p-5 space-y-4 text-sm">
              <div>
                <p className="text-muted-foreground mb-1">User</p>
                <p className="font-medium">{detail.user?.name ?? "—"}</p>
                <p className="text-caption">{detail.user?.email ?? ""}</p>
              </div>
              <div>
                <p className="text-muted-foreground mb-1">Plan</p>
                <p className="font-medium">{detail.plan?.name ?? "—"}</p>
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <p className="text-muted-foreground mb-1">Amount</p>
                  <p className="font-semibold">{fmt(detail.amount_in_paise)}</p>
                </div>
                <div>
                  <p className="text-muted-foreground mb-1">Gateway</p>
                  <StatusBadge
                    variant={detail.provider === "WALLET" ? "neutral" : "brand"}
                  >
                    {detail.provider}
                  </StatusBadge>
                </div>
                <div>
                  <p className="text-muted-foreground mb-1">Status</p>
                  <StatusBadge variant={statusVariant(detail.status)}>
                    {statusLabel(detail.status)}
                  </StatusBadge>
                </div>
                <div>
                  <p className="text-muted-foreground mb-1">Paid at</p>
                  <p>
                    {detail.paid_at
                      ? new Date(detail.paid_at).toLocaleString()
                      : "—"}
                  </p>
                </div>
                <div>
                  <p className="text-muted-foreground mb-1">Start</p>
                  <p>{new Date(detail.start_date).toLocaleString()}</p>
                </div>
                <div>
                  <p className="text-muted-foreground mb-1">Expiry</p>
                  <p>{new Date(detail.expiry_date).toLocaleString()}</p>
                </div>
              </div>
              {detail.provider_payment_id && (
                <div>
                  <p className="text-muted-foreground mb-1">
                    Razorpay payment ID
                  </p>
                  <p className="font-mono text-xs break-all">
                    {detail.provider_payment_id}
                  </p>
                </div>
              )}
              {detail.provider_order_id && (
                <div>
                  <p className="text-muted-foreground mb-1">
                    Razorpay order ID
                  </p>
                  <p className="font-mono text-xs break-all">
                    {detail.provider_order_id}
                  </p>
                </div>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
