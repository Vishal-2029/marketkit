import { formatMoney } from "@/lib/currency";
import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { PageHeader } from "@/components/PageHeader";
import { StatusBadge } from "@/components/StatusBadge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { X, CheckCircle2, XCircle, Clock, RotateCcw, AlertTriangle } from "lucide-react";
import { toast } from "sonner";
import { refundsService, type RefundRequest } from "@/services/refunds";
import { useAuth } from "@/contexts/AuthContext";

const fmt = (v: number) => formatMoney(v);

const statusVariant = (s: string) => {
  if (s === "APPROVED") return "success" as const;
  if (s === "REJECTED") return "danger" as const;
  return "warning" as const;
};

const statusIcon = (s: string) => {
  if (s === "APPROVED") return <CheckCircle2 className="h-3.5 w-3.5" />;
  if (s === "REJECTED") return <XCircle className="h-3.5 w-3.5" />;
  return <Clock className="h-3.5 w-3.5" />;
};

export default function RefundsPage() {
  const qc = useQueryClient();
  const { admin } = useAuth();
  const isSuper = !!admin?.is_super;

  const [statusFilter, setStatusFilter] = useState("PENDING");
  const [page, setPage] = useState(1);
  const [detail, setDetail] = useState<RefundRequest | null>(null);
  const [reviewNote, setReviewNote] = useState("");
  const [showReject, setShowReject] = useState(false);

  const { data, isLoading } = useQuery({
    queryKey: ["refunds", page, statusFilter],
    queryFn: () => refundsService.list({ page, status: statusFilter || undefined }),
  });

  const requests: RefundRequest[] = data?.data ?? [];
  const meta = data?.meta;

  const approveMut = useMutation({
    mutationFn: ({ id, note }: { id: string; note: string }) =>
      refundsService.approve(id, note),
    onSuccess: (data: any) => {
      qc.invalidateQueries({ queryKey: ["refunds"] });
      qc.invalidateQueries({ queryKey: ["payments"] });
      setDetail(null);
      setReviewNote("");
      toast.success(`Refund approved. Razorpay ID: ${data?.refund_id ?? ""} · ${data?.amount ?? ""} refunded.`);
    },
    onError: (err: any) => toast.error(err?.response?.data?.error ?? "Approval failed."),
  });

  const rejectMut = useMutation({
    mutationFn: ({ id, note }: { id: string; note: string }) =>
      refundsService.reject(id, note),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["refunds"] });
      setDetail(null);
      setReviewNote("");
      setShowReject(false);
      toast.success("Refund request rejected.");
    },
    onError: (err: any) => toast.error(err?.response?.data?.error ?? "Rejection failed."),
  });

  const pendingCount = requests.filter(r => r.status === "PENDING").length;

  return (
    <div>
      <PageHeader
        title="Refund Requests"
        subtitle="Manage and review refund requests raised by admins"
      />

      {/* Super admin info banner */}
      {isSuper && pendingCount > 0 && statusFilter === "PENDING" && (
        <div className="flex items-center gap-2 mb-4 px-4 py-2.5 rounded-xl bg-warning/10 border border-warning/20 text-sm">
          <Clock className="h-4 w-4 text-warning shrink-0" />
          <span className="text-warning font-medium">{pendingCount} pending request{pendingCount > 1 ? "s" : ""} awaiting your review.</span>
        </div>
      )}
      {!isSuper && (
        <div className="flex items-center gap-2 mb-4 px-4 py-2.5 rounded-xl bg-primary/5 border border-primary/10 text-sm text-muted-foreground">
          <RotateCcw className="h-3.5 w-3.5 text-primary shrink-0" />
          To request a refund, open the <span className="font-medium text-foreground mx-1">Payments</span> page and click <span className="font-medium text-foreground mx-1">Request Refund</span> on an eligible payment.
        </div>
      )}

      {/* Filter tabs */}
      <div className="flex items-center gap-2 mb-4">
        {["PENDING", "APPROVED", "REJECTED", ""].map(s => (
          <button
            key={s}
            onClick={() => { setStatusFilter(s); setPage(1); }}
            className={`px-3 py-1.5 rounded-lg text-sm font-medium transition-colors ${
              statusFilter === s
                ? "bg-primary text-primary-foreground"
                : "bg-muted text-muted-foreground hover:text-foreground"
            }`}
          >
            {s === "" ? "All" : s.charAt(0) + s.slice(1).toLowerCase()}
          </button>
        ))}
      </div>

      <div className="rounded-xl border border-border bg-card overflow-hidden">
        <table className="w-full">
          <thead>
            <tr className="bg-table-header">
              <th className="text-table-header text-left px-4 py-3">User</th>
              <th className="text-table-header text-left px-4 py-3 hidden md:table-cell">Plan · Amount</th>
              <th className="text-table-header text-left px-4 py-3">Reason</th>
              <th className="text-table-header text-left px-4 py-3 hidden md:table-cell">Requested By</th>
              <th className="text-table-header text-left px-4 py-3">Status</th>
              <th className="text-table-header text-left px-4 py-3 hidden lg:table-cell">Date</th>
              {isSuper && <th className="text-table-header text-left px-4 py-3">Actions</th>}
            </tr>
          </thead>
          <tbody>
            {isLoading
              ? Array(4).fill(0).map((_, i) => (
                <tr key={i}><td colSpan={7} className="px-4 py-3"><Skeleton className="h-8" /></td></tr>
              ))
              : requests.map(r => (
                <tr
                  key={r.id}
                  className="border-b border-border last:border-0 hover:bg-table-hover transition-colors cursor-pointer"
                  onClick={() => { setDetail(r); setReviewNote(""); setShowReject(false); }}
                >
                  <td className="px-4 py-3">
                    <p className="text-sm font-medium text-foreground">{r.payment?.user?.name ?? "—"}</p>
                    <p className="text-caption">{r.payment?.user?.email ?? ""}</p>
                  </td>
                  <td className="px-4 py-3 hidden md:table-cell">
                    <p className="text-sm text-foreground">{r.payment?.plan?.name ?? "—"}</p>
                    <p className="text-caption font-semibold">{r.payment ? fmt(r.payment.amount_minor) : "—"}</p>
                  </td>
                  <td className="px-4 py-3">
                    <p className="text-sm text-muted-foreground truncate max-w-[200px]">{r.reason}</p>
                  </td>
                  <td className="px-4 py-3 hidden md:table-cell">
                    <p className="text-sm text-foreground">
                      {r.requested_by_admin
                        ? `${r.requested_by_admin.first_name} ${r.requested_by_admin.last_name}`
                        : "—"}
                    </p>
                  </td>
                  <td className="px-4 py-3">
                    <StatusBadge variant={statusVariant(r.status)} className="gap-1">
                      {statusIcon(r.status)}
                      {r.status.charAt(0) + r.status.slice(1).toLowerCase()}
                    </StatusBadge>
                  </td>
                  <td className="px-4 py-3 text-sm text-muted-foreground hidden lg:table-cell">
                    {new Date(r.created_at).toLocaleDateString("en-IN", { day: "2-digit", month: "short", year: "numeric" })}
                  </td>
                  {isSuper && (
                    <td className="px-4 py-3" onClick={e => e.stopPropagation()}>
                      {r.status === "PENDING" && (
                        <div className="flex gap-1.5">
                          <Button
                            size="sm"
                            className="h-8 px-2 text-xs gap-1"
                            onClick={e => { e.stopPropagation(); setDetail(r); setReviewNote(""); setShowReject(false); }}
                          >
                            <CheckCircle2 className="h-3.5 w-3.5" /> Review
                          </Button>
                        </div>
                      )}
                      {r.status !== "PENDING" && (
                        <span className="text-xs text-muted-foreground italic">
                          {r.status === "APPROVED" ? "Approved" : "Rejected"}
                        </span>
                      )}
                    </td>
                  )}
                </tr>
              ))
            }
            {!isLoading && requests.length === 0 && (
              <tr>
                <td colSpan={7} className="px-4 py-10 text-center text-sm text-muted-foreground">
                  {statusFilter === "PENDING" ? "No pending refund requests." : "No refund requests found."}
                </td>
              </tr>
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

      {/* ── Detail / Review Drawer ─────────────────────────────────────────── */}
      {detail && (
        <div className="fixed inset-0 z-50 flex justify-end">
          <div className="absolute inset-0" onClick={() => setDetail(null)} />
          <div className="relative w-[480px] bg-card border-l border-border h-full overflow-y-auto shadow-xl">
            <div className="p-6 border-b border-border flex items-center justify-between">
              <div>
                <h2 className="text-section-title">Refund Request</h2>
                <StatusBadge variant={statusVariant(detail.status)} className="mt-1 gap-1">
                  {statusIcon(detail.status)}
                  {detail.status.charAt(0) + detail.status.slice(1).toLowerCase()}
                </StatusBadge>
              </div>
              <Button variant="ghost" size="icon" onClick={() => setDetail(null)}><X className="h-4 w-4" /></Button>
            </div>

            <div className="p-6 space-y-5">
              {/* User + payment */}
              <div className="rounded-xl border border-border p-4 space-y-2">
                <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wide mb-2">Payment</p>
                <div className="flex justify-between text-sm">
                  <span className="text-muted-foreground">User</span>
                  <span className="font-medium text-foreground">{detail.payment?.user?.name ?? "—"}</span>
                </div>
                <div className="flex justify-between text-sm">
                  <span className="text-muted-foreground">Email</span>
                  <span className="text-foreground">{detail.payment?.user?.email ?? "—"}</span>
                </div>
                <div className="flex justify-between text-sm">
                  <span className="text-muted-foreground">Plan</span>
                  <span className="text-foreground">{detail.payment?.plan?.name ?? "—"}</span>
                </div>
                <div className="flex justify-between text-sm">
                  <span className="text-muted-foreground">Amount</span>
                  <span className="font-bold text-foreground text-base">
                    {detail.payment ? fmt(detail.payment.amount_minor) : "—"}
                  </span>
                </div>
                {detail.payment?.provider_payment_id && (
                  <div className="flex justify-between text-sm">
                    <span className="text-muted-foreground">Razorpay ID</span>
                    <span className="font-mono text-xs text-foreground">{detail.payment.provider_payment_id}</span>
                  </div>
                )}
              </div>

              {/* Reason */}
              <div className="rounded-xl border border-border p-4">
                <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wide mb-2">Refund Reason</p>
                <p className="text-sm text-foreground leading-relaxed">{detail.reason}</p>
              </div>

              {/* Requested by */}
              <div className="rounded-xl border border-border p-4 space-y-1">
                <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wide mb-2">Requested By</p>
                <p className="text-sm font-medium text-foreground">
                  {detail.requested_by_admin
                    ? `${detail.requested_by_admin.first_name} ${detail.requested_by_admin.last_name}`
                    : "—"}
                </p>
                <p className="text-xs text-muted-foreground">{detail.requested_by_admin?.email}</p>
                <p className="text-xs text-muted-foreground">
                  {new Date(detail.created_at).toLocaleString()}
                </p>
              </div>

              {/* Review info (if already reviewed) */}
              {detail.status !== "PENDING" && (
                <div className={`rounded-xl border p-4 space-y-1 ${detail.status === "APPROVED" ? "border-success/30 bg-success/5" : "border-danger/30 bg-danger/5"}`}>
                  <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wide mb-2">
                    {detail.status === "APPROVED" ? "Approved By" : "Rejected By"}
                  </p>
                  <p className="text-sm font-medium text-foreground">
                    {detail.reviewed_by_admin
                      ? `${detail.reviewed_by_admin.first_name} ${detail.reviewed_by_admin.last_name}`
                      : "—"}
                  </p>
                  {detail.reviewed_at && (
                    <p className="text-xs text-muted-foreground">{new Date(detail.reviewed_at).toLocaleString()}</p>
                  )}
                  {detail.review_note && (
                    <p className="text-sm text-foreground mt-1">{detail.review_note}</p>
                  )}
                  {detail.status === "APPROVED" && detail.refund_id && (
                    <p className="text-xs font-mono text-muted-foreground mt-1">Refund ID: {detail.refund_id}</p>
                  )}
                </div>
              )}

              {/* Super admin actions */}
              {isSuper && detail.status === "PENDING" && (
                <div className="space-y-3 pt-1">
                  {!showReject ? (
                    <>
                      {/* Approve section */}
                      <div>
                        <label className="text-sm font-medium text-foreground">
                          Approval Note <span className="text-muted-foreground font-normal">— optional</span>
                        </label>
                        <Input
                          className="mt-1"
                          placeholder="e.g. Verified — access was not given due to technical issue"
                          value={reviewNote}
                          onChange={e => setReviewNote(e.target.value)}
                        />
                      </div>

                      <div className="flex items-start gap-2 p-3 rounded-xl bg-warning/10 border border-warning/20">
                        <AlertTriangle className="h-4 w-4 text-warning shrink-0 mt-0.5" />
                        <p className="text-xs text-warning leading-relaxed">
                          Approving will call Razorpay and issue a <strong>full refund of {detail.payment ? fmt(detail.payment.amount_minor) : ""}</strong> and cancel the subscription. This cannot be undone.
                        </p>
                      </div>

                      <div className="flex gap-2">
                        <Button
                          className="flex-1 gap-1.5"
                          onClick={() => approveMut.mutate({ id: detail.id, note: reviewNote })}
                          disabled={approveMut.isPending}
                        >
                          <CheckCircle2 className="h-4 w-4" />
                          {approveMut.isPending ? "Processing…" : "Approve & Refund"}
                        </Button>
                        <Button
                          variant="danger-outline"
                          className="flex-1 gap-1.5"
                          onClick={() => setShowReject(true)}
                        >
                          <XCircle className="h-4 w-4" /> Reject
                        </Button>
                      </div>
                    </>
                  ) : (
                    <>
                      <div>
                        <label className="text-sm font-medium text-foreground">Rejection Reason *</label>
                        <Input
                          className="mt-1"
                          placeholder="e.g. Subscription was already used"
                          value={reviewNote}
                          onChange={e => setReviewNote(e.target.value)}
                          autoFocus
                        />
                      </div>
                      <div className="flex gap-2">
                        <Button variant="ghost" className="flex-1" onClick={() => { setShowReject(false); setReviewNote(""); }}>
                          Cancel
                        </Button>
                        <Button
                          variant="destructive"
                          className="flex-1 gap-1.5"
                          disabled={!reviewNote.trim() || rejectMut.isPending}
                          onClick={() => rejectMut.mutate({ id: detail.id, note: reviewNote })}
                        >
                          <XCircle className="h-4 w-4" />
                          {rejectMut.isPending ? "Rejecting…" : "Confirm Reject"}
                        </Button>
                      </div>
                    </>
                  )}
                </div>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
