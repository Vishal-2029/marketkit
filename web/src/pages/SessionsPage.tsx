import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { PageHeader } from "@/components/PageHeader";
import { Button } from "@/components/ui/button";
import { RefreshCw, MapPin } from "lucide-react";
import { toast } from "sonner";
import { sessionsService } from "@/services/sessions";
import { Skeleton } from "@/components/ui/skeleton";

function lastSeen(dateStr: string): string {
  const diff = Date.now() - new Date(dateStr).getTime();
  const m = Math.floor(diff / 60_000);
  if (m < 1) return "just now";
  if (m < 60) return `${m} min${m === 1 ? "" : "s"} ago`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h} hr${h === 1 ? "" : "s"} ago`;
  return `${Math.floor(h / 24)}d ago`;
}

function isRecent(dateStr: string): boolean {
  return Date.now() - new Date(dateStr).getTime() < 5 * 60_000;
}

function formatDevice(device?: string): string {
  if (!device) return "—";
  if (device.toLowerCase().includes("dart") || device.toLowerCase().includes("dart:io")) {
    return "MarketKit";
  }
  if (device.startsWith("Unknown OS · ")) {
    return device.replace("Unknown OS · ", "");
  }
  return device;
}

interface Session {
  id: string;
  user?: { name: string; email: string };
  device_info: string;
  ip_address: string;
  location: string;
  created_at: string;
  last_active_at: string;
}

export default function SessionsPage() {
  const qc = useQueryClient();
  const [confirmId, setConfirmId] = useState<string | null>(null);
  const [confirmAll, setConfirmAll] = useState(false);
  const [refreshing, setRefreshing] = useState(false);

  const { data: sessions = [], isLoading, isFetching, refetch } = useQuery({
    queryKey: ["sessions"],
    queryFn: sessionsService.list,
    refetchInterval: 30_000,
  });

  const handleRefresh = async () => {
    setRefreshing(true);
    try {
      await refetch();
      toast.success("Session list refreshed.");
    } catch {
      toast.error("Failed to refresh sessions.");
    } finally {
      setRefreshing(false);
    }
  };

  const revokeMut = useMutation({
    mutationFn: (id: string) => sessionsService.revoke(id),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ["sessions"] }); toast.success("Session terminated."); },
    onError: () => toast.error("Failed to revoke session."),
  });

  const revokeAllMut = useMutation({
    mutationFn: sessionsService.revokeAll,
    onSuccess: () => { qc.invalidateQueries({ queryKey: ["sessions"] }); toast.success("All sessions terminated."); },
    onError: () => toast.error("Failed to revoke all sessions."),
  });

  const backfillMut = useMutation({
    mutationFn: sessionsService.backfillLocations,
    onSuccess: (res: { message?: string }) => {
      qc.invalidateQueries({ queryKey: ["sessions"] });
      toast.success(res?.message ? res.message.charAt(0).toUpperCase() + res.message.slice(1) : "Locations updated.");
    },
    onError: () => toast.error("Failed to fetch locations."),
  });

  const handleConfirm = () => {
    if (confirmAll) revokeAllMut.mutate();
    else if (confirmId) revokeMut.mutate(confirmId);
    setConfirmId(null);
    setConfirmAll(false);
  };

  const pendingSession = sessions.find((s: Session) => s.id === confirmId);

  return (
    <div>
      <PageHeader title="Session Monitor" subtitle="Currently active sessions across all users">
        <div className="flex items-center gap-3">
          <div className="flex items-center gap-2">
            <div className="w-2 h-2 rounded-full bg-success pulse-dot" />
            <span className="text-sm font-medium text-success-foreground">Live</span>
          </div>
          <Button
            variant="outline"
            size="sm"
            onClick={handleRefresh}
            disabled={isFetching || refreshing}
          >
            <RefreshCw className="h-3 w-3" /> {(isFetching || refreshing) ? "Refreshing..." : "Refresh"}
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={() => backfillMut.mutate()}
            disabled={backfillMut.isPending || sessions.length === 0}
          >
            <MapPin className="h-3 w-3" /> {backfillMut.isPending ? "Fetching..." : "Fetch Locations"}
          </Button>
          <Button variant="danger-outline" size="sm" onClick={() => setConfirmAll(true)} disabled={sessions.length === 0}>
            Force Logout All
          </Button>
        </div>
      </PageHeader>

      <div className="rounded-xl border border-border bg-card overflow-hidden">
        <table className="w-full">
          <thead>
            <tr className="bg-table-header">
              <th className="text-table-header text-left px-4 py-3">User</th>
              <th className="text-table-header text-left px-4 py-3">Device</th>
              <th className="text-table-header text-left px-4 py-3">IP Address</th>
              <th className="text-table-header text-left px-4 py-3">Location</th>
              <th className="text-table-header text-left px-4 py-3">Started</th>
              <th className="text-table-header text-left px-4 py-3">Last Active</th>
              <th className="text-table-header text-left px-4 py-3">Actions</th>
            </tr>
          </thead>
          <tbody>
            {isLoading
              ? Array(3).fill(0).map((_, i) => (
                <tr key={i}><td colSpan={7} className="px-4 py-3"><Skeleton className="h-8" /></td></tr>
              ))
              : (sessions as Session[]).map(s => (
                <tr key={s.id} className="border-b border-border last:border-0 hover:bg-table-hover transition-colors">
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-3">
                      <div className="w-8 h-8 rounded-full bg-accent flex items-center justify-center">
                        <span className="text-xs font-semibold text-accent-foreground">{s.user?.name?.[0] ?? "?"}</span>
                      </div>
                      <div>
                        <p className="text-sm font-medium text-foreground">{s.user?.name ?? "—"}</p>
                        <p className="text-caption">{s.user?.email ?? "—"}</p>
                      </div>
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    <p className="text-sm text-foreground">
                      {formatDevice(s.device_info)}
                    </p>
                  </td>
                  <td className="px-4 py-3 text-sm font-mono text-muted-foreground">{s.ip_address || "—"}</td>
                  <td className="px-4 py-3">
                    <p className="text-sm text-foreground">
                      {s.location || "—"}
                    </p>
                  </td>
                  <td className="px-4 py-3 text-sm text-muted-foreground">{new Date(s.created_at).toLocaleString()}</td>
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-2">
                      <div className={`w-2 h-2 rounded-full shrink-0 ${isRecent(s.last_active_at) ? "bg-success pulse-dot" : "bg-muted-foreground/40"}`} />
                      <div>
                        <p className="text-sm text-foreground">{lastSeen(s.last_active_at)}</p>
                        <p className="text-xs text-muted-foreground">{new Date(s.last_active_at).toLocaleString()}</p>
                      </div>
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    <Button variant="danger-outline" size="sm" onClick={() => setConfirmId(s.id)}>Force Logout</Button>
                  </td>
                </tr>
              ))
            }
            {!isLoading && sessions.length === 0 && (
              <tr><td colSpan={7} className="px-4 py-8 text-center text-sm text-muted-foreground">No active sessions.</td></tr>
            )}
          </tbody>
        </table>
      </div>

      {(confirmId || confirmAll) && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-foreground/40">
          <div className="bg-card rounded-2xl shadow-xl w-full max-w-md p-6 text-center">
            <div className="w-12 h-12 rounded-full bg-danger-bg flex items-center justify-center mx-auto mb-4">
              <span className="text-danger text-xl">⚠️</span>
            </div>
            <h2 className="text-section-title mb-2">
              {confirmAll ? "Force Logout All Users?" : `Force Logout ${pendingSession?.user?.name}?`}
            </h2>
            <p className="text-sm text-muted-foreground mb-6">
              {confirmAll ? "This will immediately disconnect all active sessions." : "This will immediately disconnect their device."}
            </p>
            <div className="flex justify-center gap-3">
              <Button variant="ghost" onClick={() => { setConfirmId(null); setConfirmAll(false); }}>Cancel</Button>
              <Button variant="destructive" onClick={handleConfirm}>Force Logout</Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
