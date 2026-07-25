import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { PageHeader } from "@/components/PageHeader";
import { StatusBadge } from "@/components/StatusBadge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { X, CheckCircle2, Clock, Copy, Landmark, Smartphone, Settings2 } from "lucide-react";
import { toast } from "sonner";
import { withdrawalsService, walletSettingsService, type Withdrawal } from "@/services/wallet";
import { useAuth } from "@/contexts/AuthContext";

function fmt(paise: number) {
  return "₹" + (paise / 100).toLocaleString("en-IN");
}

function copy(text: string, label: string) {
  navigator.clipboard.writeText(text);
  toast.success(`${label} copied.`);
}

function CopyRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex justify-between items-center text-sm">
      <span className="text-muted-foreground">{label}</span>
      <button
        className="flex items-center gap-1.5 font-mono text-xs text-foreground hover:text-primary transition-colors"
        onClick={() => copy(value, label)}
        title="Copy"
      >
        {value}
        <Copy className="h-3 w-3" />
      </button>
    </div>
  );
}

function SettingsCard() {
  const qc = useQueryClient();
  const { admin } = useAuth();
  const isSuper = !!admin?.is_super;

  const { data } = useQuery({
    queryKey: ["wallet-settings"],
    queryFn: walletSettingsService.get,
  });

  const [editing, setEditing] = useState(false);
  const [fee, setFee] = useState("");
  const [minW, setMinW] = useState("");

  const saveMut = useMutation({
    mutationFn: () =>
      walletSettingsService.update({
        fee_percent: Number(fee),
        min_withdrawal_in_paise: Math.round(Number(minW) * 100),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["wallet-settings"] });
      setEditing(false);
      toast.success("Platform settings updated.");
    },
    onError: (err: any) => toast.error(err?.response?.data?.error ?? "Update failed."),
  });

  return (
    <div className="rounded-xl border border-border bg-card p-4 mb-4">
      <div className="flex items-center justify-between mb-3">
        <div className="flex items-center gap-2">
          <Settings2 className="h-4 w-4 text-primary" />
          <p className="text-sm font-semibold text-foreground">Platform Settings</p>
        </div>
        {isSuper && !editing && data && (
          <Button
            variant="outline"
            size="sm"
            onClick={() => {
              setFee(String(data.fee_percent));
              setMinW(String(data.min_withdrawal_in_paise / 100));
              setEditing(true);
            }}
          >
            Edit
          </Button>
        )}
      </div>
      {!editing ? (
        <div className="flex gap-8 text-sm">
          <div>
            <p className="text-muted-foreground">Platform fee on sales</p>
            <p className="text-lg font-bold text-foreground">{data ? `${data.fee_percent}%` : "—"}</p>
          </div>
          <div>
            <p className="text-muted-foreground">Minimum withdrawal</p>
            <p className="text-lg font-bold text-foreground">{data ? fmt(data.min_withdrawal_in_paise) : "—"}</p>
          </div>
        </div>
      ) : (
        <div className="flex flex-wrap items-end gap-3">
          <div>
            <label className="text-xs text-muted-foreground">Fee percent (0–50)</label>
            <Input className="mt-1 w-32" type="number" value={fee} onChange={e => setFee(e.target.value)} />
          </div>
          <div>
            <label className="text-xs text-muted-foreground">Min withdrawal (₹)</label>
            <Input className="mt-1 w-32" type="number" value={minW} onChange={e => setMinW(e.target.value)} />
          </div>
          <div className="flex gap-2">
            <Button size="sm" onClick={() => saveMut.mutate()} disabled={saveMut.isPending}>
              {saveMut.isPending ? "Saving…" : "Save"}
            </Button>
            <Button size="sm" variant="ghost" onClick={() => setEditing(false)}>Cancel</Button>
          </div>
        </div>
      )}
      {!isSuper && (
        <p className="text-xs text-muted-foreground mt-2">Only super-admins can change these settings.</p>
      )}
    </div>
  );
}

export default function WithdrawalsPage() {
  const qc = useQueryClient();

  const [statusFilter, setStatusFilter] = useState("APPROVED");
  const [page, setPage] = useState(1);
  const [detail, setDetail] = useState<Withdrawal | null>(null);
  const [note, setNote] = useState("");

  const { data, isLoading } = useQuery({
    queryKey: ["withdrawals", page, statusFilter],
    queryFn: () => withdrawalsService.list({ page, status: statusFilter || undefined }),
  });

  const withdrawals: Withdrawal[] = data?.data ?? [];
  const meta = data?.meta;

  const settleMut = useMutation({
    mutationFn: ({ id, note }: { id: string; note: string }) =>
      withdrawalsService.settle(id, note),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["withdrawals"] });
      setDetail(null);
      setNote("");
      toast.success("Withdrawal marked as settled.");
    },
    onError: (err: any) => toast.error(err?.response?.data?.error ?? "Settling failed."),
  });

  return (
    <div>
      <PageHeader
        title="Withdrawals"
        subtitle="Wallet payout requests — transfer the money manually, then mark settled"
      />

      <SettingsCard />

      {/* Filter tabs */}
      <div className="flex items-center gap-2 mb-4">
        {["APPROVED", "SETTLED", ""].map(s => (
          <button
            key={s}
            onClick={() => { setStatusFilter(s); setPage(1); }}
            className={`px-3 py-1.5 rounded-lg text-sm font-medium transition-colors ${
              statusFilter === s
                ? "bg-primary text-primary-foreground"
                : "bg-muted text-muted-foreground hover:text-foreground"
            }`}
          >
            {s === "" ? "All" : s === "APPROVED" ? "To Pay" : "Settled"}
          </button>
        ))}
      </div>

      <div className="rounded-xl border border-border bg-card overflow-hidden">
        <table className="w-full">
          <thead>
            <tr className="bg-table-header">
              <th className="text-table-header text-left px-4 py-3">User</th>
              <th className="text-table-header text-left px-4 py-3">Amount</th>
              <th className="text-table-header text-left px-4 py-3">Method</th>
              <th className="text-table-header text-left px-4 py-3 hidden md:table-cell">Destination</th>
              <th className="text-table-header text-left px-4 py-3">Status</th>
              <th className="text-table-header text-left px-4 py-3 hidden lg:table-cell">Requested</th>
            </tr>
          </thead>
          <tbody>
            {isLoading
              ? Array(4).fill(0).map((_, i) => (
                <tr key={i}><td colSpan={6} className="px-4 py-3"><Skeleton className="h-8" /></td></tr>
              ))
              : withdrawals.map(w => (
                <tr
                  key={w.id}
                  className="border-b border-border last:border-0 hover:bg-table-hover transition-colors cursor-pointer"
                  onClick={() => { setDetail(w); setNote(""); }}
                >
                  <td className="px-4 py-3">
                    <p className="text-sm font-medium text-foreground">{w.user_name ?? "—"}</p>
                    <p className="text-caption">{w.user_email ?? ""}</p>
                  </td>
                  <td className="px-4 py-3">
                    <p className="text-sm font-bold text-foreground">{fmt(w.amount_in_paise)}</p>
                  </td>
                  <td className="px-4 py-3">
                    <span className="inline-flex items-center gap-1.5 text-sm text-foreground">
                      {w.method === "UPI"
                        ? <Smartphone className="h-3.5 w-3.5 text-primary" />
                        : <Landmark className="h-3.5 w-3.5 text-primary" />}
                      {w.method}
                    </span>
                  </td>
                  <td className="px-4 py-3 hidden md:table-cell">
                    <p className="text-xs font-mono text-muted-foreground">
                      {w.method === "UPI" ? w.upi_id : `${w.bank_account_number} · ${w.bank_ifsc}`}
                    </p>
                  </td>
                  <td className="px-4 py-3">
                    <StatusBadge variant={w.status === "SETTLED" ? "success" : "warning"} className="gap-1">
                      {w.status === "SETTLED"
                        ? <CheckCircle2 className="h-3.5 w-3.5" />
                        : <Clock className="h-3.5 w-3.5" />}
                      {w.status === "SETTLED" ? "Settled" : "To Pay"}
                    </StatusBadge>
                  </td>
                  <td className="px-4 py-3 text-sm text-muted-foreground hidden lg:table-cell">
                    {new Date(w.created_at).toLocaleDateString("en-IN", { day: "2-digit", month: "short", year: "numeric" })}
                  </td>
                </tr>
              ))
            }
            {!isLoading && withdrawals.length === 0 && (
              <tr>
                <td colSpan={6} className="px-4 py-10 text-center text-sm text-muted-foreground">
                  {statusFilter === "APPROVED" ? "No withdrawals waiting to be paid." : "No withdrawals found."}
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

      {/* ── Detail / Settle Drawer ─────────────────────────────────────────── */}
      {detail && (
        <div className="fixed inset-0 z-50 flex justify-end">
          <div className="absolute inset-0" onClick={() => setDetail(null)} />
          <div className="relative w-[480px] bg-card border-l border-border h-full overflow-y-auto shadow-xl">
            <div className="p-6 border-b border-border flex items-center justify-between">
              <div>
                <h2 className="text-section-title">Withdrawal</h2>
                <StatusBadge variant={detail.status === "SETTLED" ? "success" : "warning"} className="mt-1 gap-1">
                  {detail.status === "SETTLED" ? <CheckCircle2 className="h-3.5 w-3.5" /> : <Clock className="h-3.5 w-3.5" />}
                  {detail.status === "SETTLED" ? "Settled" : "To Pay"}
                </StatusBadge>
              </div>
              <Button variant="ghost" size="icon" onClick={() => setDetail(null)}><X className="h-4 w-4" /></Button>
            </div>

            <div className="p-6 space-y-5">
              <div className="rounded-xl border border-border p-4 space-y-2">
                <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wide mb-2">Request</p>
                <div className="flex justify-between text-sm">
                  <span className="text-muted-foreground">User</span>
                  <span className="font-medium text-foreground">{detail.user_name ?? "—"}</span>
                </div>
                <div className="flex justify-between text-sm">
                  <span className="text-muted-foreground">Email</span>
                  <span className="text-foreground">{detail.user_email ?? "—"}</span>
                </div>
                <div className="flex justify-between text-sm">
                  <span className="text-muted-foreground">Amount</span>
                  <span className="font-bold text-foreground text-base">{fmt(detail.amount_in_paise)}</span>
                </div>
                <div className="flex justify-between text-sm">
                  <span className="text-muted-foreground">Requested</span>
                  <span className="text-foreground">{new Date(detail.created_at).toLocaleString()}</span>
                </div>
              </div>

              {/* Payout destination — copy-friendly for the manual transfer */}
              <div className="rounded-xl border border-border p-4 space-y-2">
                <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wide mb-2">
                  Pay via {detail.method}
                </p>
                {detail.method === "UPI" ? (
                  <CopyRow label="UPI ID" value={detail.upi_id ?? "—"} />
                ) : (
                  <>
                    <CopyRow label="Account No." value={detail.bank_account_number ?? "—"} />
                    <CopyRow label="IFSC" value={detail.bank_ifsc ?? "—"} />
                    <div className="flex justify-between text-sm">
                      <span className="text-muted-foreground">Holder Name</span>
                      <span className="text-foreground">{detail.bank_holder_name ?? "—"}</span>
                    </div>
                  </>
                )}
              </div>

              {detail.status === "SETTLED" ? (
                <div className="rounded-xl border border-success/30 bg-success/5 p-4 space-y-1">
                  <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wide mb-2">Settled</p>
                  {detail.settled_at && (
                    <p className="text-xs text-muted-foreground">{new Date(detail.settled_at).toLocaleString()}</p>
                  )}
                  {detail.note && <p className="text-sm text-foreground mt-1">{detail.note}</p>}
                </div>
              ) : (
                <div className="space-y-3 pt-1">
                  <div>
                    <label className="text-sm font-medium text-foreground">
                      Note <span className="text-muted-foreground font-normal">— optional (e.g. UTR / reference no.)</span>
                    </label>
                    <Input
                      className="mt-1"
                      placeholder="e.g. Paid via UPI, ref 4218..."
                      value={note}
                      onChange={e => setNote(e.target.value)}
                    />
                  </div>
                  <Button
                    className="w-full gap-1.5"
                    onClick={() => settleMut.mutate({ id: detail.id, note })}
                    disabled={settleMut.isPending}
                  >
                    <CheckCircle2 className="h-4 w-4" />
                    {settleMut.isPending ? "Saving…" : "Mark as Settled"}
                  </Button>
                  <p className="text-xs text-muted-foreground">
                    The balance was already deducted when the user requested this — settle only after you have
                    actually transferred {fmt(detail.amount_in_paise)}.
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
