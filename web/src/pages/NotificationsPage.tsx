import { useRef, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Send, Bell, RotateCcw, Users, CreditCard, Activity, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { useToast } from "@/hooks/use-toast";
import { notificationsService, NotificationLog, Plan } from "@/services/notifications";

type AudienceMode = "all" | "plan" | "status";

const SUB_STATUSES = ["ACTIVE", "EXPIRED", "SUSPENDED", "CANCELLED"] as const;

function timeAgo(dateStr: string) {
  const diff = Date.now() - new Date(dateStr).getTime();
  const m = Math.floor(diff / 60000);
  if (m < 1) return "just now";
  if (m < 60) return `${m}m ago`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h}h ago`;
  return `${Math.floor(h / 24)}d ago`;
}

function formatAudience(audience: string): string {
  if (audience === "all") return "All Users";
  if (audience.startsWith("plan:") && audience.includes("+status:")) {
    const [p, s] = audience.split("+status:");
    return `${p.replace("plan:", "")} · ${s}`;
  }
  if (audience.startsWith("plan:")) return `Plan: ${audience.replace("plan:", "")}`;
  if (audience.startsWith("status:")) return `Status: ${audience.replace("status:", "")}`;
  return audience;
}

export default function NotificationsPage() {
  const { toast } = useToast();
  const qc = useQueryClient();
  const formRef = useRef<HTMLDivElement>(null);

  const [title, setTitle] = useState("");
  const [message, setMessage] = useState("");
  const [audienceMode, setAudienceMode] = useState<AudienceMode>("all");
  const [selectedPlan, setSelectedPlan] = useState("");
  const [selectedStatus, setSelectedStatus] = useState("");
  const [histPage, setHistPage] = useState(1);
  const [confirmClear, setConfirmClear] = useState(false);

  const { data: plansData } = useQuery({
    queryKey: ["plans-for-notif"],
    queryFn: notificationsService.listPlans,
  });
  const plans: Plan[] = plansData ?? [];

  const { data: histData, isLoading: histLoading } = useQuery({
    queryKey: ["notifications", histPage],
    queryFn: () => notificationsService.listNotifications(histPage),
  });

  const broadcast = useMutation({
    mutationFn: () => {
      const planId    = audienceMode === "plan"   ? selectedPlan   : undefined;
      const subStatus = audienceMode === "status" ? selectedStatus : undefined;
      return notificationsService.broadcast(title.trim(), message.trim(), planId, subStatus);
    },
    onSuccess: () => {
      toast({ title: "Notification sent", description: "Delivered to the selected audience." });
      setTitle("");
      setMessage("");
      qc.invalidateQueries({ queryKey: ["notifications"] });
    },
    onError: () => {
      toast({ title: "Failed to send", variant: "destructive" });
    },
  });

  const deleteOne = useMutation({
    mutationFn: (id: number) => notificationsService.deleteNotification(id),
    onSuccess: () => {
      toast({ title: "Notification removed" });
      qc.invalidateQueries({ queryKey: ["notifications"] });
    },
    onError: () => {
      toast({ title: "Failed to remove", variant: "destructive" });
    },
  });

  const clearAll = useMutation({
    mutationFn: () => notificationsService.clearNotifications(),
    onSuccess: () => {
      toast({ title: "History cleared" });
      setHistPage(1);
      qc.invalidateQueries({ queryKey: ["notifications"] });
    },
    onError: () => {
      toast({ title: "Failed to clear history", variant: "destructive" });
    },
  });

  const canSend =
    title.trim().length > 0 &&
    message.trim().length > 0 &&
    (audienceMode === "all" ||
      (audienceMode === "plan"   && selectedPlan   !== "") ||
      (audienceMode === "status" && selectedStatus !== ""));

  const prefill = (log: NotificationLog) => {
    setTitle(log.title);
    setMessage(log.body.replace(/ · Tap to (view|watch)$/i, ""));
    formRef.current?.scrollIntoView({ behavior: "smooth", block: "start" });
  };

  const logs: NotificationLog[] = histData?.logs ?? [];
  const total = histData?.total ?? 0;
  const totalPages = Math.ceil(total / 20);

  const AUDIENCE_OPTIONS: { value: AudienceMode; label: string; icon: React.ComponentType<{ className?: string }>; desc: string }[] = [
    { value: "all",    label: "All Users",  icon: Users,      desc: "Every user with a registered device" },
    { value: "plan",   label: "By Plan",    icon: CreditCard, desc: "Users subscribed to a specific plan" },
    { value: "status", label: "By Status",  icon: Activity,   desc: "Users with a matching subscription status" },
  ];

  return (
    <div className="space-y-8">
      {/* Send form */}
      <div ref={formRef}>
        <div>
          <h1 className="text-2xl font-bold text-foreground">Push Notifications</h1>
          <p className="text-sm text-muted-foreground mt-1">
            Send a push notification to a targeted audience.
          </p>
        </div>

        <div className="max-w-lg bg-card border border-border rounded-xl p-6 space-y-5 mt-6">
          {/* Card header */}
          <div className="flex items-center gap-3 pb-4 border-b border-border">
            <div className="w-10 h-10 rounded-lg bg-primary/10 flex items-center justify-center">
              <Bell className="h-5 w-5 text-primary" />
            </div>
            <div>
              <p className="text-sm font-semibold text-foreground">Compose Notification</p>
              <p className="text-xs text-muted-foreground">Choose your audience, then write your message</p>
            </div>
          </div>

          {/* Audience picker */}
          <div className="space-y-2">
            <Label>Audience</Label>
            <div className="grid grid-cols-3 gap-2">
              {AUDIENCE_OPTIONS.map(({ value, label, icon: Icon }) => (
                <button
                  key={value}
                  type="button"
                  onClick={() => {
                    setAudienceMode(value);
                    setSelectedPlan("");
                    setSelectedStatus("");
                  }}
                  className={`flex flex-col items-center gap-1.5 rounded-lg border px-3 py-2.5 text-xs font-medium transition-colors ${
                    audienceMode === value
                      ? "border-primary bg-primary/5 text-primary"
                      : "border-border text-muted-foreground hover:border-primary/40 hover:text-foreground"
                  }`}
                >
                  <Icon className="h-4 w-4" />
                  {label}
                </button>
              ))}
            </div>

            {/* Plan dropdown */}
            {audienceMode === "plan" && (
              <select
                className="mt-1 h-10 w-full rounded-lg border border-border bg-card px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                value={selectedPlan}
                onChange={e => setSelectedPlan(e.target.value)}
              >
                <option value="">Select a plan…</option>
                {plans.filter(p => p.is_active).map(p => (
                  <option key={p.id} value={p.id}>{p.name}</option>
                ))}
              </select>
            )}

            {/* Status dropdown */}
            {audienceMode === "status" && (
              <select
                className="mt-1 h-10 w-full rounded-lg border border-border bg-card px-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                value={selectedStatus}
                onChange={e => setSelectedStatus(e.target.value)}
              >
                <option value="">Select a status…</option>
                {SUB_STATUSES.map(s => (
                  <option key={s} value={s}>{s.charAt(0) + s.slice(1).toLowerCase()}</option>
                ))}
              </select>
            )}

            <p className="text-xs text-muted-foreground">
              {AUDIENCE_OPTIONS.find(o => o.value === audienceMode)?.desc}
            </p>
          </div>

          {/* Title */}
          <div className="space-y-2">
            <Label htmlFor="notif-title">Title</Label>
            <Input
              id="notif-title"
              placeholder="e.g. New content available!"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              maxLength={100}
            />
            <p className="text-xs text-muted-foreground text-right">{title.length}/100</p>
          </div>

          {/* Message */}
          <div className="space-y-2">
            <Label htmlFor="notif-message">Message</Label>
            <Textarea
              id="notif-message"
              placeholder="e.g. Check out the new module we just published."
              value={message}
              onChange={(e) => setMessage(e.target.value)}
              maxLength={300}
              rows={4}
            />
            <p className="text-xs text-muted-foreground text-right">{message.length}/300</p>
          </div>

          <Button
            className="w-full gap-2"
            disabled={!canSend || broadcast.isPending}
            onClick={() => broadcast.mutate()}
          >
            <Send className="h-4 w-4" />
            {broadcast.isPending ? "Sending…" : "Send Notification"}
          </Button>
        </div>
      </div>

      {/* History */}
      <div>
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-semibold">Sent Notifications</h2>
          {logs.length > 0 && (
            <Button
              size="sm"
              variant="outline"
              className="gap-1.5 text-destructive hover:text-destructive"
              onClick={() => setConfirmClear(true)}
              disabled={clearAll.isPending}
            >
              <Trash2 className="h-3.5 w-3.5" />
              Clear All
            </Button>
          )}
        </div>
        {histLoading ? (
          <div className="space-y-2">
            {[...Array(3)].map((_, i) => <Skeleton key={i} className="h-14 w-full rounded-xl" />)}
          </div>
        ) : logs.length === 0 ? (
          <p className="text-sm text-muted-foreground py-8 text-center">No notifications sent yet.</p>
        ) : (
          <>
            <div className="rounded-xl border border-border overflow-hidden">
              <table className="w-full text-sm">
                <thead className="bg-muted/40">
                  <tr>
                    <th className="text-left px-4 py-3 font-medium text-muted-foreground">Title</th>
                    <th className="text-left px-4 py-3 font-medium text-muted-foreground">Message</th>
                    <th className="text-left px-4 py-3 font-medium text-muted-foreground">Audience</th>
                    <th className="text-left px-4 py-3 font-medium text-muted-foreground">Sent</th>
                    <th className="text-left px-4 py-3 font-medium text-muted-foreground">Failed</th>
                    <th className="text-left px-4 py-3 font-medium text-muted-foreground">Time</th>
                    <th className="text-left px-4 py-3 font-medium text-muted-foreground w-px whitespace-nowrap">Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {logs.map((log) => (
                    <tr key={log.id} className="border-t border-border hover:bg-muted/20 transition-colors">
                      <td className="px-4 py-3 font-medium max-w-[160px] truncate">{log.title}</td>
                      <td className="px-4 py-3 text-muted-foreground max-w-[200px] truncate">{log.body.replace(/ · Tap to (view|watch)$/i, "")}</td>
                      <td className="px-4 py-3">
                        <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-muted text-foreground">
                          {formatAudience(log.audience)}
                        </span>
                      </td>
                      <td className="px-4 py-3 text-emerald-600 font-medium">{log.sent_count}</td>
                      <td className="px-4 py-3 text-destructive font-medium">{log.failed_count}</td>
                      <td className="px-4 py-3 text-muted-foreground text-xs whitespace-nowrap">
                        {timeAgo(log.created_at)}
                      </td>
                      <td className="px-4 py-3 w-px whitespace-nowrap">
                        <div className="flex items-center gap-1.5">
                          <Button
                            size="sm"
                            variant="outline"
                            className="h-7 px-2 gap-1"
                            onClick={() => prefill(log)}
                            title="Pre-fill form to re-send"
                          >
                            <RotateCcw className="h-3 w-3" />
                            Re-send
                          </Button>
                          <Button
                            size="sm"
                            variant="outline"
                            className="h-7 w-7 p-0 text-destructive hover:text-destructive"
                            onClick={() => deleteOne.mutate(log.id)}
                            disabled={deleteOne.isPending}
                            title="Remove from history"
                          >
                            <Trash2 className="h-3 w-3" />
                          </Button>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>

            {totalPages > 1 && (
              <div className="flex items-center justify-between mt-4">
                <p className="text-xs text-muted-foreground">Page {histPage} of {totalPages}</p>
                <div className="flex gap-2">
                  <Button size="sm" variant="outline" disabled={histPage === 1} onClick={() => setHistPage((p) => p - 1)}>
                    Previous
                  </Button>
                  <Button size="sm" variant="outline" disabled={histPage >= totalPages} onClick={() => setHistPage((p) => p + 1)}>
                    Next
                  </Button>
                </div>
              </div>
            )}
          </>
        )}
      </div>

      {/* Clear-all confirmation */}
      {confirmClear && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-foreground/40">
          <div className="bg-card rounded-2xl shadow-xl w-full max-w-md p-6 text-center">
            <div className="w-12 h-12 rounded-full bg-destructive/10 flex items-center justify-center mx-auto mb-4">
              <Trash2 className="h-5 w-5 text-destructive" />
            </div>
            <h2 className="text-lg font-semibold mb-2">Clear all notifications?</h2>
            <p className="text-sm text-muted-foreground mb-6">
              This permanently removes the entire sent-notification history. This can't be undone.
            </p>
            <div className="flex justify-center gap-3">
              <Button variant="ghost" onClick={() => setConfirmClear(false)}>Cancel</Button>
              <Button
                variant="destructive"
                disabled={clearAll.isPending}
                onClick={() => { clearAll.mutate(); setConfirmClear(false); }}
              >
                {clearAll.isPending ? "Clearing…" : "Clear All"}
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
