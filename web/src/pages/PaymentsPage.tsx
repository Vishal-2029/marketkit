import { formatMoney } from "@/lib/currency";
import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { PageHeader } from "@/components/PageHeader";
import { StatusBadge } from "@/components/StatusBadge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { Search, Download, X, AlertTriangle, RotateCcw, CheckCircle2, Eye } from "lucide-react";
import { toast } from "sonner";
import { paymentsService } from "@/services/payments";
import { refundsService } from "@/services/refunds";
import { useAuth } from "@/contexts/AuthContext";

interface Payment {
  id: string;
  user?: { name: string; email: string };
  plan?: { name: string };
  amount_minor: number;
  provider: string;
  status: "SUCCESS" | "FAILED" | "REFUNDED" | "PENDING";
  provider_payment_id?: string;
  paid_at?: string;
  created_at: string;
  notes?: string;
  is_free_plan?: boolean;
  gateway_response?: Record<string, unknown>;
}

const statusVariant = (s: string) => {
  if (s === "SUCCESS") return "success" as const;
  if (s === "FAILED") return "danger" as const;
  if (s === "REFUNDED") return "warning" as const;
  return "neutral" as const;
};

const statusLabel = (s: string) => s.charAt(0) + s.slice(1).toLowerCase();

const fmt = (v: number) => formatMoney(v);

export default function PaymentsPage() {
  const qc = useQueryClient();
  const { admin } = useAuth();
  const isSuper = !!admin?.is_super;

  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [page, setPage] = useState(1);
  const [detail, setDetail] = useState<Payment | null>(null);

  // Refund request state
  const [refundTarget, setRefundTarget] = useState<Payment | null>(null);
  const [refundReason, setRefundReason] = useState("");

  const { data, isLoading } = useQuery({
    queryKey: ["payments", page, search, statusFilter],
    queryFn: () => paymentsService.list({ page, limit: 20, search: search || undefined, status: statusFilter || undefined }),
  });

  const payments: Payment[] = data?.data ?? [];
  const meta = data?.meta;

  const activateMut = useMutation({
    mutationFn: (id: string) => paymentsService.activate(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["payments"] });
      toast.success("Plan activated.");
      setDetail(null);
    },
    onError: () => toast.error("Failed to activate plan."),
  });

  const refundRequestMut = useMutation({
    mutationFn: ({ id, reason }: { id: string; reason: string }) =>
      refundsService.request(id, reason),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["refunds"] });
      qc.invalidateQueries({ queryKey: ["payments"] });
      setRefundTarget(null);
      setRefundReason("");
      toast.success("Refund request submitted. Awaiting super admin approval.");
    },
    onError: (err: any) => toast.error(err?.response?.data?.error ?? "Failed to submit refund request."),
  });

  const openRefundModal = (p: Payment, e?: React.MouseEvent) => {
    e?.stopPropagation();
    setRefundReason("");
    setRefundTarget(p);
  };

  const closeRefundModal = () => {
    setRefundTarget(null);
    setRefundReason("");
  };

  const exportCSV = () => {
    const headers = ["User", "Email", "Plan", "Amount", "Gateway", "Status", "Date", "Reference", "Refund ID", "Refunded At", "Refund Reason"];
    const lines = [
      headers.join(","),
      ...payments.map(p => {
        const gr = p.gateway_response ?? {};
        return [
          p.user?.name ?? "",
          p.user?.email ?? "",
          p.plan?.name ?? "",
          fmt(p.amount_minor),
          p.provider,
          p.status,
          new Date(p.created_at).toLocaleString(),
          p.provider_payment_id ?? p.id,
          p.status === "REFUNDED" ? (gr["refund_id"] as string ?? "") : "",
          p.status === "REFUNDED" && gr["refunded_at"] ? new Date(gr["refunded_at"] as string).toLocaleString() : "",
          p.status === "REFUNDED" ? (gr["refund_reason"] as string ?? "") : "",
        ].map(v => `"${String(v).replace(/"/g, '""')}"`).join(",");
      }),
    ];
    const blob = new Blob([lines.join("\n")], { type: "text/csv" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url; a.download = "payments.csv"; a.click();
    URL.revokeObjectURL(url);
    toast.success("CSV exported.");
  };

  const canRefund = (p: Payment) =>
    p.status === "SUCCESS" && p.provider === "razorpay";

  return (
    <div>
      <PageHeader title="Payments" subtitle="All payment transactions">
        <Button variant="outline" onClick={exportCSV}><Download className="h-4 w-4" /> Export CSV</Button>
      </PageHeader>

      <div className="flex items-center gap-3 mb-6">
        <div className="relative flex-1 max-w-sm">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input
            placeholder="Search by name or email..."
            value={search}
            onChange={e => { setSearch(e.target.value); setPage(1); }}
            className="pl-9 bg-muted border-0"
          />
        </div>
        <select
          className="h-10 rounded-lg border border-border bg-card px-3 text-sm"
          value={statusFilter}
          onChange={e => { setStatusFilter(e.target.value); setPage(1); }}
        >
          <option value="">All Status</option>
          <option value="SUCCESS">Success</option>
          <option value="FAILED">Failed</option>
          <option value="REFUNDED">Refunded</option>
          <option value="PENDING">Pending</option>
        </select>
      </div>

      <div className="flex items-center gap-2 mb-4 px-4 py-2.5 rounded-xl bg-primary/5 border border-primary/10 text-sm text-muted-foreground">
        <RotateCcw className="h-3.5 w-3.5 text-primary shrink-0" />
        Click <span className="font-medium text-foreground mx-1">Refund</span> on any SUCCESS · RAZORPAY payment to submit a refund request for super-admin review.
      </div>

      <div className="rounded-xl border border-border bg-card overflow-hidden">
        <table className="w-full">
          <thead>
            <tr className="bg-table-header">
              <th className="text-table-header text-left px-4 py-3">User</th>
              <th className="text-table-header text-left px-4 py-3 hidden md:table-cell">Plan</th>
              <th className="text-table-header text-left px-4 py-3">Amount</th>
              <th className="text-table-header text-left px-4 py-3 hidden md:table-cell">Gateway</th>
              <th className="text-table-header text-left px-4 py-3">Status</th>
              <th className="text-table-header text-left px-4 py-3 hidden lg:table-cell">Date</th>
              <th className="text-table-header text-left px-4 py-3">Actions</th>
            </tr>
          </thead>
          <tbody>
            {isLoading
              ? Array(5).fill(0).map((_, i) => (
                <tr key={i}><td colSpan={7} className="px-4 py-3"><Skeleton className="h-8" /></td></tr>
              ))
              : payments.map(p => (
                <tr
                  key={p.id}
                  className="border-b border-border last:border-0 hover:bg-table-hover transition-colors cursor-pointer"
                  onClick={() => setDetail(p)}
                >
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-3">
                      <div className="w-8 h-8 rounded-full bg-accent flex items-center justify-center shrink-0">
                        <span className="text-xs font-semibold text-accent-foreground">{(p.user?.name ?? "?")[0]}</span>
                      </div>
                      <div>
                        <div className="flex items-center gap-1.5">
                          <p className="text-sm font-medium text-foreground">{p.user?.name ?? "—"}</p>
                          {p.is_free_plan && (
                            <span className="inline-flex items-center px-1.5 py-0.5 rounded text-[10px] font-semibold bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400 leading-none">
                              FREE
                            </span>
                          )}
                        </div>
                        <p className="text-caption">{p.user?.email ?? ""}</p>
                      </div>
                    </div>
                  </td>
                  <td className="px-4 py-3 hidden md:table-cell">
                    <StatusBadge variant="brand">{p.plan?.name ?? "—"}</StatusBadge>
                  </td>
                  <td className="px-4 py-3 text-sm font-semibold text-foreground">{fmt(p.amount_minor)}</td>
                  <td className="px-4 py-3 hidden md:table-cell">
                    <StatusBadge variant={p.provider === "MANUAL" ? "neutral" : "brand"}>{p.provider}</StatusBadge>
                  </td>
                  <td className="px-4 py-3">
                    <StatusBadge variant={statusVariant(p.status)}>{statusLabel(p.status)}</StatusBadge>
                  </td>
                  <td className="px-4 py-3 text-sm text-muted-foreground hidden lg:table-cell">
                    {new Date(p.created_at).toLocaleString()}
                  </td>
                  {/* ── Actions column ── */}
                  <td className="px-4 py-3" onClick={e => e.stopPropagation()}>
                    <div className="flex items-center gap-1.5">
                      <Button
                        variant="ghost"
                        size="sm"
                        className="h-8 px-2 text-xs gap-1"
                        onClick={e => { e.stopPropagation(); setDetail(p); }}
                      >
                        <Eye className="h-3.5 w-3.5" /> View
                      </Button>
                      {canRefund(p) && (
                        <Button
                          variant="danger-outline"
                          size="sm"
                          className="h-8 px-2 text-xs gap-1"
                          onClick={e => openRefundModal(p, e)}
                        >
                          <RotateCcw className="h-3.5 w-3.5" /> Refund
                        </Button>
                      )}
                      {p.status === "REFUNDED" && (
                        <span className="text-xs text-muted-foreground italic">Refunded</span>
                      )}
                    </div>
                  </td>
                </tr>
              ))
            }
            {!isLoading && payments.length === 0 && (
              <tr><td colSpan={7} className="px-4 py-8 text-center text-sm text-muted-foreground">No payments found.</td></tr>
            )}
          </tbody>
        </table>
        {meta && meta.pages > 1 && (
          <div className="bg-table-header px-4 py-3 flex items-center justify-between">
            <span className="text-caption">Page {page} of {meta.pages} · {meta.total} total</span>
            <div className="flex gap-1">
              <Button variant="outline" size="sm" disabled={page === 1} onClick={() => setPage(p => p - 1)}>Previous</Button>
              <Button variant="outline" size="sm" disabled={page === meta.pages} onClick={() => setPage(p => p + 1)}>Next</Button>
            </div>
          </div>
        )}
      </div>

      {/* ── Payment Detail Drawer ──────────────────────────────────────────────── */}
      {detail && (
        <div className="fixed inset-0 z-50 flex justify-end">
          <div className="absolute inset-0" onClick={() => setDetail(null)} />
          <div className="relative w-[480px] bg-card border-l border-border h-full overflow-y-auto shadow-xl">
            <div className="p-6 border-b border-border flex items-center justify-between">
              <h2 className="text-section-title">Payment Details</h2>
              <Button variant="ghost" size="icon" onClick={() => setDetail(null)}><X className="h-4 w-4" /></Button>
            </div>
            <div className="p-6 space-y-5">
              <div className="flex items-center gap-3">
                <div className="w-12 h-12 rounded-full bg-accent flex items-center justify-center shrink-0">
                  <span className="text-lg font-semibold text-accent-foreground">{(detail.user?.name ?? "?")[0]}</span>
                </div>
                <div>
                  <div className="flex items-center gap-2">
                    <p className="text-lg font-semibold text-foreground">{detail.user?.name ?? "—"}</p>
                    {detail.is_free_plan && (
                      <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-semibold bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400">FREE</span>
                    )}
                  </div>
                  <p className="text-sm text-muted-foreground">{detail.user?.email ?? ""}</p>
                </div>
              </div>

              <div className="grid grid-cols-2 gap-4">
                <div className="rounded-xl border border-border p-3">
                  <p className="text-caption">Amount</p>
                  <p className="text-lg font-bold text-foreground">{fmt(detail.amount_minor)}</p>
                </div>
                <div className="rounded-xl border border-border p-3">
                  <p className="text-caption">Status</p>
                  <StatusBadge variant={statusVariant(detail.status)} className="mt-1">{statusLabel(detail.status)}</StatusBadge>
                </div>
              </div>

              <div className="rounded-xl border border-border p-3 space-y-2">
                <div className="flex justify-between text-sm">
                  <span className="text-muted-foreground">Plan</span>
                  <span className="font-medium text-foreground">{detail.plan?.name ?? "—"}</span>
                </div>
                <div className="flex justify-between text-sm">
                  <span className="text-muted-foreground">Gateway</span>
                  <span className="font-medium text-foreground">{detail.provider}</span>
                </div>
                {detail.paid_at && (
                  <div className="flex justify-between text-sm">
                    <span className="text-muted-foreground">Paid At</span>
                    <span className="font-medium text-foreground">{new Date(detail.paid_at).toLocaleString()}</span>
                  </div>
                )}
              </div>

              {detail.provider_payment_id && (
                <div className="rounded-xl border border-border p-3">
                  <p className="text-caption mb-1">Razorpay Payment ID</p>
                  <p className="text-sm font-mono text-foreground">{detail.provider_payment_id}</p>
                </div>
              )}

              {detail.notes && (
                <div className="rounded-xl border border-border p-3">
                  <p className="text-caption mb-1">Notes / Reason</p>
                  <p className="text-sm text-muted-foreground">{detail.notes}</p>
                </div>
              )}

              <div className="space-y-2 pt-1">
                {detail.status === "FAILED" && (
                  <Button className="w-full" onClick={() => activateMut.mutate(detail.id)} disabled={activateMut.isPending}>
                    Manually Activate Plan
                  </Button>
                )}

                {canRefund(detail) && (
                  <Button
                    variant="danger-outline"
                    className="w-full gap-2"
                    onClick={() => { setDetail(null); openRefundModal(detail); }}
                  >
                    <RotateCcw className="h-4 w-4" /> Request Refund
                  </Button>
                )}

                {detail.status === "SUCCESS" && detail.provider !== "razorpay" && (
                  <div className="flex items-center gap-2 px-3 py-2.5 rounded-lg bg-muted border border-border">
                    <RotateCcw className="h-4 w-4 text-muted-foreground shrink-0" />
                    <p className="text-xs text-muted-foreground">Refunds are only available for Razorpay payments.</p>
                  </div>
                )}

                {detail.status === "REFUNDED" && (
                  <div className="rounded-lg bg-warning/10 border border-warning/20 px-3 py-2.5 space-y-1">
                    <div className="flex items-center gap-2">
                      <CheckCircle2 className="h-4 w-4 text-warning shrink-0" />
                      <p className="text-sm text-warning font-medium">This payment has been refunded.</p>
                    </div>
                    {detail.gateway_response?.["refund_id"] && (
                      <p className="text-xs text-muted-foreground pl-6">Refund ID: <span className="font-mono text-foreground">{detail.gateway_response["refund_id"] as string}</span></p>
                    )}
                    {detail.gateway_response?.["refunded_at"] && (
                      <p className="text-xs text-muted-foreground pl-6">Refunded at: {new Date(detail.gateway_response["refunded_at"] as string).toLocaleString()}</p>
                    )}
                    {detail.gateway_response?.["refund_reason"] && (
                      <p className="text-xs text-muted-foreground pl-6">Reason: {detail.gateway_response["refund_reason"] as string}</p>
                    )}
                  </div>
                )}
              </div>
            </div>
          </div>
        </div>
      )}

      {/* ── Refund Request Modal ──────────────────────────────────────────── */}
      {refundTarget && (
        <div className="fixed inset-0 z-[60] flex items-center justify-center bg-foreground/40">
          <div className="bg-card rounded-2xl shadow-xl w-full max-w-md p-6 max-h-[90vh] overflow-y-auto">
            <div className="flex items-center justify-between mb-5">
               <div>
                 <h2 className="text-section-title">Request Refund</h2>
                 <p className="text-xs text-muted-foreground mt-0.5">
                   {refundTarget.user?.name} · {refundTarget.plan?.name} · {fmt(refundTarget.amount_minor)}
                 </p>
               </div>
               <Button variant="ghost" size="icon" onClick={closeRefundModal}>
                 <X className="h-4 w-4" />
               </Button>
            </div>

            <div className="space-y-4">
              <div className="flex items-start gap-2 p-3 rounded-xl bg-primary/5 border border-primary/10">
                <AlertTriangle className="h-4 w-4 text-primary shrink-0 mt-0.5" />
                <p className="text-xs text-muted-foreground leading-relaxed">
                  Refunds are restricted. You can only request a refund for the following reasons. Refunds will be automatically denied if the user has already watched a video or if the subscription has expired.
                </p>
              </div>

              <div>
                <label className="text-sm font-medium text-foreground">
                  Refund Reason <span className="text-danger">*</span>
                </label>
                <select
                  className="mt-1 w-full h-10 rounded-md border border-input bg-transparent px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2"
                  value={refundReason}
                  onChange={e => setRefundReason(e.target.value)}
                >
                  <option value="" disabled>Select a reason...</option>
                  <option value="Payment Successful But Access Not Given">Payment Successful But Access Not Given</option>
                  <option value="Technical Issue on Admin Side">Technical Issue on Admin Side</option>
                </select>
              </div>

              <div className="flex justify-end gap-2 pt-2">
                <Button variant="ghost" onClick={closeRefundModal}>Cancel</Button>
                <Button
                  variant="destructive"
                  onClick={() => refundRequestMut.mutate({
                    id: refundTarget.id,
                    reason: refundReason,
                  })}
                  disabled={!refundReason || refundRequestMut.isPending}
                  className="gap-1.5"
                >
                  <RotateCcw className="h-4 w-4" />
                  {refundRequestMut.isPending ? "Submitting..." : "Submit Request"}
                </Button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
