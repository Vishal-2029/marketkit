import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { PageHeader } from "@/components/PageHeader";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Search, Download, ChevronDown, ChevronRight,
  LogIn, LogOut, CreditCard, Shield, ShieldAlert, Activity, UserX, VideoOff, Edit3, Settings,
  Ban, UserCheck, AlertCircle
} from "lucide-react";
import { toast } from "sonner";
import { auditLogsService } from "@/services/auditLogs";
import { Skeleton } from "@/components/ui/skeleton";
import React from "react";

interface AuditLog {
  id: string;
  event_type: string;
  actor_user_id?: string;
  actor_admin_id?: string;
  ip_address?: string;
  device?: string;
  details?: Record<string, unknown>;
  created_at: string;
  actor_user?: { name: string; email: string };
  actor_admin?: { name: string; email: string };
}

type IconComponent = React.ComponentType<{ className?: string }>;

const EVENT_CFG: Record<string, { Icon: IconComponent; pill: string; dot: string; label: string }> = {
  LOGIN:             { Icon: LogIn,       pill: "bg-sky-50 text-sky-700 ring-1 ring-sky-200",          dot: "bg-sky-400",     label: "Admin Login" },
  LOGOUT:            { Icon: LogOut,      pill: "bg-slate-100 text-slate-600 ring-1 ring-slate-200",   dot: "bg-slate-400",   label: "Admin Logout" },
  PLAN_PURCHASE:     { Icon: CreditCard,  pill: "bg-emerald-50 text-emerald-700 ring-1 ring-emerald-200", dot: "bg-emerald-400", label: "Plan Purchase" },
  PLAN_CHANGED:      { Icon: Edit3,       pill: "bg-teal-50 text-teal-700 ring-1 ring-teal-200",       dot: "bg-teal-400",    label: "Plan Changed" },
  MANUAL_ACTIVATION: { Icon: Settings,    pill: "bg-indigo-50 text-indigo-700 ring-1 ring-indigo-200", dot: "bg-indigo-400",  label: "Manual Activation" },
  ADMIN_ACTION:      { Icon: Shield,      pill: "bg-amber-50 text-amber-700 ring-1 ring-amber-200",    dot: "bg-amber-400",   label: "Admin Action" },
  FORCE_LOGOUT:      { Icon: ShieldAlert, pill: "bg-red-50 text-red-700 ring-1 ring-red-200",          dot: "bg-red-400",     label: "Force Logout" },
  USER_DELETED:      { Icon: UserX,       pill: "bg-rose-50 text-rose-700 ring-1 ring-rose-200",       dot: "bg-rose-400",    label: "User Deleted" },
  USER_SUSPENDED:    { Icon: Ban,         pill: "bg-pink-50 text-pink-700 ring-1 ring-pink-200",       dot: "bg-pink-400",    label: "User Suspended" },
  USER_REACTIVATED:  { Icon: UserCheck,   pill: "bg-lime-50 text-lime-700 ring-1 ring-lime-200",       dot: "bg-lime-400",    label: "User Reactivated" },
  VIDEO_DELETED:     { Icon: VideoOff,    pill: "bg-orange-50 text-orange-700 ring-1 ring-orange-200", dot: "bg-orange-400",  label: "Video Deleted" },
  PAYMENT_FAILED:    { Icon: AlertCircle, pill: "bg-red-50 text-red-700 ring-1 ring-red-200",          dot: "bg-red-400",     label: "Payment Failed" },
};

const DEFAULT_CFG = { Icon: Activity, pill: "bg-muted text-muted-foreground ring-1 ring-border", dot: "bg-muted-foreground", label: "" };

function getInitials(name: string) {
  return name.split(" ").map(n => n[0] ?? "").join("").toUpperCase().slice(0, 2);
}

function relativeTime(dateStr: string) {
  const diff = Date.now() - new Date(dateStr).getTime();
  const m = Math.floor(diff / 60000);
  if (m < 1) return "just now";
  if (m < 60) return `${m}m ago`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h}h ago`;
  const d = Math.floor(h / 24);
  return d < 7 ? `${d}d ago` : new Date(dateStr).toLocaleDateString();
}

function EventBadge({ type }: { type: string }) {
  const cfg = EVENT_CFG[type] ?? DEFAULT_CFG;
  const Icon = cfg.Icon;
  return (
    <span className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium whitespace-nowrap ${cfg.pill}`}>
      <Icon className="h-3 w-3 shrink-0" />
      {cfg.label || type.replace(/_/g, " ")}
    </span>
  );
}

function ActorCell({ log }: { log: AuditLog }) {
  const isAdmin = !!log.actor_admin;
  const name  = log.actor_user?.name  ?? log.actor_admin?.name  ?? "System";
  const email = log.actor_user?.email ?? log.actor_admin?.email ?? "";
  const initials = getInitials(name);
  return (
    <div className="flex items-center gap-2.5 min-w-0">
      <div className={`h-8 w-8 rounded-full flex items-center justify-center text-xs font-semibold shrink-0 ${isAdmin ? "bg-amber-100 text-amber-700" : "bg-primary/10 text-primary"}`}>
        {initials}
      </div>
      <div className="min-w-0">
        <p className="text-sm font-medium text-foreground truncate">{name}</p>
        {email && <p className="text-xs text-muted-foreground truncate">{email}</p>}
        {isAdmin && <span className="text-[10px] font-semibold uppercase tracking-wide text-amber-600">Admin</span>}
      </div>
    </div>
  );
}

function DetailPanel({ details }: { details?: Record<string, unknown> }) {
  if (!details || Object.keys(details).length === 0) {
    return <p className="text-xs text-muted-foreground italic">No additional details.</p>;
  }
  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 gap-x-8 gap-y-1.5">
      {Object.entries(details).map(([k, v]) => (
        <div key={k} className="flex gap-2 text-xs">
          <span className="font-medium text-foreground capitalize shrink-0">{k.replace(/_/g, " ")}:</span>
          <span className="text-muted-foreground break-all">{String(v)}</span>
        </div>
      ))}
    </div>
  );
}

const ALL_EVENTS = "All Events";
const EVENT_TYPES = [
  "LOGIN", "LOGOUT", "PLAN_PURCHASE", "PLAN_CHANGED", 
  "MANUAL_ACTIVATION", "ADMIN_ACTION", "FORCE_LOGOUT", 
  "USER_DELETED", "USER_SUSPENDED", "USER_REACTIVATED",
  "VIDEO_DELETED", "PAYMENT_FAILED"
];

export default function AuditLogsPage() {
  const [search, setSearch] = useState("");
  const [eventFilter, setEventFilter] = useState(ALL_EVENTS);
  const [expanded, setExpanded] = useState<string | null>(null);
  const [page, setPage] = useState(1);
  const [isExporting, setIsExporting] = useState(false);

  const { data, isLoading } = useQuery({
    queryKey: ["audit-logs", page, eventFilter],
    queryFn: () => auditLogsService.list({
      page,
      limit: 50,
      event_type: eventFilter !== ALL_EVENTS ? eventFilter : undefined,
    }),
  });

  const logs: AuditLog[] = data?.data ?? [];
  const meta = data?.meta;

  const filtered = logs.filter(l => {
    if (!search) return true;
    const actor = l.actor_user?.name ?? l.actor_admin?.name ?? "";
    const email = l.actor_user?.email ?? l.actor_admin?.email ?? "";
    const q = search.toLowerCase();
    return actor.toLowerCase().includes(q) || email.toLowerCase().includes(q) || l.event_type.toLowerCase().includes(q);
  });

  const exportCSV = async () => {
    setIsExporting(true);
    try {
      const PAGE_SIZE = 200;
      let pg = 1;
      let allLogs: AuditLog[] = [];
      while (true) {
        const result = await auditLogsService.list({
          page: pg,
          limit: PAGE_SIZE,
          event_type: eventFilter !== ALL_EVENTS ? eventFilter : undefined,
        });
        const batch: AuditLog[] = result.data ?? [];
        allLogs = [...allLogs, ...batch];
        if (batch.length < PAGE_SIZE || allLogs.length >= (result.meta?.total ?? 0)) break;
        pg++;
      }
      const toExport = allLogs.filter(l => {
        if (!search) return true;
        const actor = l.actor_user?.name ?? l.actor_admin?.name ?? "";
        const email = l.actor_user?.email ?? l.actor_admin?.email ?? "";
        const q = search.toLowerCase();
        return actor.toLowerCase().includes(q) || email.toLowerCase().includes(q) || l.event_type.toLowerCase().includes(q);
      });
      const headers = ["Timestamp","Event","Actor","Email","IP","Device","Details"];
      const lines = [
        headers.join(","),
        ...toExport.map(l => [
          new Date(l.created_at).toLocaleString(),
          l.event_type,
          l.actor_user?.name ?? l.actor_admin?.name ?? "—",
          l.actor_user?.email ?? l.actor_admin?.email ?? "",
          l.ip_address ?? "",
          l.device ?? "",
          JSON.stringify(l.details ?? {}),
        ].map(v => `"${String(v).replace(/"/g,'""')}"`).join(",")),
      ];
      const blob = new Blob([lines.join("\n")], { type: "text/csv" });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url; a.download = "audit-logs.csv"; a.click();
      URL.revokeObjectURL(url);
      toast.success(`Exported ${toExport.length} audit log${toExport.length === 1 ? "" : "s"}.`);
    } catch {
      toast.error("Export failed. Please try again.");
    } finally {
      setIsExporting(false);
    }
  };

  return (
    <div>
      <PageHeader title="Audit Logs" subtitle="Full history of all system and admin-relevant events">
        <Button variant="outline" size="sm" onClick={exportCSV} disabled={isExporting} className="gap-2">
          {isExporting
            ? <><div className="h-4 w-4 border-2 border-current border-t-transparent rounded-full animate-spin" /> Exporting…</>
            : <><Download className="h-4 w-4" /> Export CSV</>
          }
        </Button>
      </PageHeader>

      {/* Search bar */}
      <div className="relative mb-3 w-full sm:max-w-sm">
        <Search className="absolute left-3.5 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground pointer-events-none" />
        <Input
          placeholder="Search by name, email or event…"
          value={search}
          onChange={e => { setSearch(e.target.value); setPage(1); }}
          className="h-11 w-full rounded-lg bg-card pl-10 pr-3 text-sm shadow-sm focus-visible:ring-offset-0"
        />
      </div>

      {/* Event filter chips */}
      <div className="flex gap-2 mb-5 overflow-x-auto pb-1 scrollbar-none">
        {[ALL_EVENTS, ...EVENT_TYPES].map(et => {
          const cfg = et === ALL_EVENTS ? null : (EVENT_CFG[et] ?? DEFAULT_CFG);
          const active = eventFilter === et;
          return (
            <button
              key={et}
              onClick={() => { setEventFilter(et); setPage(1); }}
              className={`inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full text-xs font-medium border transition-colors shrink-0 ${
                active
                  ? "bg-primary text-primary-foreground border-primary"
                  : "bg-card text-muted-foreground border-border hover:border-primary/50 hover:text-foreground"
              }`}
            >
              {cfg && <cfg.Icon className="h-3 w-3" />}
              {et === ALL_EVENTS ? "All" : cfg?.label ?? et}
            </button>
          );
        })}
      </div>

      {/* Count bar */}
      {!isLoading && (
        <p className="text-xs text-muted-foreground mb-3">
          Showing <span className="font-medium text-foreground">{filtered.length}</span> {filtered.length === 1 ? "entry" : "entries"}
          {meta ? ` · Page ${page} of ${meta.pages}` : ""}
        </p>
      )}

      {/* Table */}
      <div className="rounded-xl border border-border bg-card overflow-hidden shadow-sm">
        <table className="w-full">
          <thead>
            <tr className="bg-muted/60 border-b border-border">
              <th className="w-8 px-4 py-3" />
              <th className="text-left px-4 py-3 text-xs font-semibold uppercase tracking-wide text-muted-foreground">Event</th>
              <th className="text-left px-4 py-3 text-xs font-semibold uppercase tracking-wide text-muted-foreground">Actor</th>
              <th className="text-left px-4 py-3 text-xs font-semibold uppercase tracking-wide text-muted-foreground hidden md:table-cell">Device / IP</th>
              <th className="text-right px-4 py-3 text-xs font-semibold uppercase tracking-wide text-muted-foreground">Time</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-border">
            {isLoading
              ? Array(8).fill(0).map((_, i) => (
                <tr key={i}>
                  <td colSpan={5} className="px-4 py-4">
                    <Skeleton className="h-4 w-full" />
                  </td>
                </tr>
              ))
              : filtered.map(l => {
                const isExp = expanded === l.id;
                const cfg = EVENT_CFG[l.event_type] ?? DEFAULT_CFG;
                return (
                  <React.Fragment key={l.id}>
                    <tr
                      className="hover:bg-muted/30 transition-colors cursor-pointer group"
                      onClick={() => setExpanded(isExp ? null : l.id)}
                    >
                      <td className="px-4 py-3.5">
                        <div className={`w-1.5 h-1.5 rounded-full ${cfg.dot}`} />
                      </td>
                      <td className="px-4 py-3.5">
                        <EventBadge type={l.event_type} />
                      </td>
                      <td className="px-4 py-3.5">
                        <ActorCell log={l} />
                      </td>
                      <td className="px-4 py-3.5 hidden md:table-cell">
                        <p className="text-xs text-muted-foreground">{l.device || "—"}</p>
                        <p className="text-xs font-mono text-muted-foreground/70">{l.ip_address || "—"}</p>
                      </td>
                      <td className="px-4 py-3.5 text-right">
                        <div className="flex items-center justify-end gap-2">
                          <div className="text-right">
                            <p className="text-xs font-medium text-foreground">{relativeTime(l.created_at)}</p>
                            <p className="text-[11px] text-muted-foreground">{new Date(l.created_at).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}</p>
                          </div>
                          {isExp
                            ? <ChevronDown className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
                            : <ChevronRight className="h-3.5 w-3.5 text-muted-foreground/40 group-hover:text-muted-foreground shrink-0 transition-colors" />
                          }
                        </div>
                      </td>
                    </tr>

                    {isExp && (
                      <tr>
                        <td colSpan={5} className="bg-muted/30 border-b border-border">
                          <div className="px-10 py-4 space-y-3">
                            <div className="flex items-center gap-2 mb-2">
                              <span className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Event Details</span>
                              <div className="flex-1 h-px bg-border" />
                              <span className="text-[11px] text-muted-foreground font-mono">{new Date(l.created_at).toLocaleString()}</span>
                            </div>
                            <DetailPanel details={l.details} />
                          </div>
                        </td>
                      </tr>
                    )}
                  </React.Fragment>
                );
              })
            }
            {!isLoading && filtered.length === 0 && (
              <tr>
                <td colSpan={5} className="px-4 py-16 text-center">
                  <Activity className="h-8 w-8 text-muted-foreground/30 mx-auto mb-3" />
                  <p className="text-sm font-medium text-muted-foreground">No logs found</p>
                  <p className="text-xs text-muted-foreground/60 mt-1">Try changing your filters or search query</p>
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      {/* Pagination */}
      {meta && meta.pages > 1 && (
        <div className="flex items-center justify-between mt-4">
          <p className="text-sm text-muted-foreground">Page {page} of {meta.pages}</p>
          <div className="flex gap-2">
            <Button variant="outline" size="sm" disabled={page === 1} onClick={() => setPage(p => p - 1)}>
              ← Previous
            </Button>
            <Button variant="outline" size="sm" disabled={page === meta.pages} onClick={() => setPage(p => p + 1)}>
              Next →
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
