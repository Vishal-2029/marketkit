import { currencySymbol, formatMoney } from "@/lib/currency";
import { useQuery } from "@tanstack/react-query";
import { PageHeader } from "@/components/PageHeader";
import { StatCard } from "@/components/StatCard";
import { StatusBadge } from "@/components/StatusBadge";
import {
  LineChart, Line, BarChart, Bar, PieChart, Pie, Cell,
  XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer,
} from "recharts";
import { dashboardService } from "@/services/dashboard";
import { Skeleton } from "@/components/ui/skeleton";
import { PRIMARY, SECONDARY, ACTIVE, GRID, SERIES } from "@/lib/chartColors";

// Donut slices — shared brand ramp, see lib/chartColors.
const PLAN_COLORS = SERIES;
const TOOLTIP_STYLE = {
  contentStyle: {
    background: "hsl(0 0% 100%)",
    border: "1px solid hsl(240 6% 90%)",
    borderRadius: "10px",
    fontSize: "12px",
    color: "hsl(240 6% 12%)",
  },
};

const MONTH_NAMES = ["","Jan","Feb","Mar","Apr","May","Jun","Jul","Aug","Sep","Oct","Nov","Dec"];

const fmt = (v: number) => formatMoney(v);

export default function DashboardPage() {
  const stats = useQuery({ queryKey: ["dashboard-stats"], queryFn: dashboardService.getStats });
  const recentPayments = useQuery({ queryKey: ["dashboard-payments"], queryFn: dashboardService.getRecentPayments });
  const recentActivity = useQuery({ queryKey: ["dashboard-activity"], queryFn: dashboardService.getRecentActivity });
  const userGrowth = useQuery({ queryKey: ["dashboard-growth"], queryFn: dashboardService.getUserGrowth });
  const planDist = useQuery({ queryKey: ["dashboard-plans"], queryFn: dashboardService.getPlanDistribution });
  const monthlyRev = useQuery({ queryKey: ["dashboard-revenue"], queryFn: dashboardService.getMonthlyRevenue });

  const growthData = (userGrowth.data ?? []).map((r: { month: number; year: number; count: number }) => ({
    month: MONTH_NAMES[r.month],
    users: r.count,
  }));

  const revenueData = (monthlyRev.data ?? []).map((r: { month: number; revenue_minor: number }) => ({
    month: MONTH_NAMES[r.month],
    revenue: Math.round(r.revenue_minor / 100000),
  }));

  const planData = (planDist.data ?? []).map((r: { plan_name: string; subscribers: number }, i: number) => ({
    name: r.plan_name,
    value: Number(r.subscribers),
    color: PLAN_COLORS[i % PLAN_COLORS.length],
  }));

  const s = stats.data;

  return (
    <div>
      <PageHeader title="Dashboard" subtitle="Here's what's happening today" />

      {/* KPI Cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
        {stats.isLoading ? (
          Array(4).fill(0).map((_, i) => <Skeleton key={i} className="h-24 rounded-xl" />)
        ) : (
          <>
            <StatCard label="Total Users" value={Number(s?.total_users ?? 0).toLocaleString()} trend="up" />
            <StatCard label="Active Subscriptions" value={Number(s?.active_subs ?? 0).toLocaleString()} />
            <StatCard label="Videos Published" value={Number(s?.published_videos ?? 0).toLocaleString()} trend="up" />
            <StatCard label="Revenue (This Month)" value={fmt(s?.month_revenue_minor ?? 0)} trend="up" />
          </>
        )}
      </div>

      {/* User Growth Chart */}
      <div className="rounded-xl border border-border bg-card p-5 mb-6">
        <h2 className="text-section-title mb-4">User Growth (Last 12 Months)</h2>
        {userGrowth.isLoading ? <Skeleton className="h-48" /> : (
          <ResponsiveContainer width="100%" height={220}>
            <LineChart data={growthData} margin={{ top: 5, right: 20, left: 0, bottom: 5 }}>
              <CartesianGrid strokeDasharray="3 3" stroke={GRID} />
              <XAxis dataKey="month" tick={{ fontSize: 12, fill: "hsl(0 0% 42%)" }} axisLine={false} tickLine={false} />
              <YAxis tick={{ fontSize: 12, fill: "hsl(0 0% 42%)" }} axisLine={false} tickLine={false} />
              <Tooltip {...TOOLTIP_STYLE} formatter={(v: number) => [v.toLocaleString(), "Total Users"]} />
              <Line type="monotone" dataKey="users" stroke={PRIMARY} strokeWidth={2.5} dot={{ fill: PRIMARY, r: 4 }} activeDot={{ r: 6, fill: ACTIVE }} />
            </LineChart>
          </ResponsiveContainer>
        )}
      </div>

      {/* Recent Payments + Plan Donut */}
      <div className="grid grid-cols-1 lg:grid-cols-5 gap-6 mb-6">
        <div className="lg:col-span-3 rounded-xl border border-border bg-card">
          <div className="p-4 border-b border-border">
            <h2 className="text-section-title">Recent Payments</h2>
          </div>
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead>
                <tr className="bg-table-header">
                  <th className="text-table-header text-left px-4 py-3">User</th>
                  <th className="text-table-header text-left px-4 py-3">Plan</th>
                  <th className="text-table-header text-left px-4 py-3">Amount</th>
                  <th className="text-table-header text-left px-4 py-3">Status</th>
                </tr>
              </thead>
              <tbody>
                {recentPayments.isLoading
                  ? Array(4).fill(0).map((_, i) => (
                    <tr key={i}><td colSpan={4} className="px-4 py-3"><Skeleton className="h-4" /></td></tr>
                  ))
                  : (recentPayments.data ?? []).map((p: { id: string; user?: { name: string }; plan?: { name: string }; amount_minor: number; status: string }) => (
                    <tr key={p.id} className="border-b border-border last:border-0 hover:bg-table-hover transition-colors">
                      <td className="px-4 py-3 text-sm font-medium">{p.user?.name ?? "—"}</td>
                      <td className="px-4 py-3"><StatusBadge variant="brand">{p.plan?.name ?? "—"}</StatusBadge></td>
                      <td className="px-4 py-3 text-sm font-semibold">{fmt(p.amount_minor)}</td>
                      <td className="px-4 py-3">
                        <StatusBadge variant={p.status === "SUCCESS" ? "success" : "danger"}>{p.status}</StatusBadge>
                      </td>
                    </tr>
                  ))
                }
              </tbody>
            </table>
          </div>
          <div className="p-3 border-t border-border">
            <a href="/payments" className="text-sm text-primary hover:underline">View all payments →</a>
          </div>
        </div>

        <div className="lg:col-span-2 rounded-xl border border-border bg-card p-4">
          <h2 className="text-section-title mb-2">Plan Distribution</h2>
          {planDist.isLoading ? <Skeleton className="h-48" /> : (
            <>
              <ResponsiveContainer width="100%" height={200}>
                <PieChart>
                  <Pie data={planData} cx="50%" cy="50%" innerRadius={55} outerRadius={85} paddingAngle={2} dataKey="value">
                    {planData.map((entry: { color: string }, i: number) => <Cell key={i} fill={entry.color} />)}
                  </Pie>
                  <Tooltip {...TOOLTIP_STYLE} formatter={(v: number, name: string) => [v + " subscribers", name]} />
                </PieChart>
              </ResponsiveContainer>
              <div className="space-y-1 mt-1">
                {planData.slice(0, 4).map((p: { name: string; value: number; color: string }) => (
                  <div key={p.name} className="flex items-center gap-2">
                    <div className="w-2.5 h-2.5 rounded-full shrink-0" style={{ background: p.color }} />
                    <span className="text-xs text-muted-foreground truncate flex-1">{p.name}</span>
                    <span className="text-xs font-medium">{p.value}</span>
                  </div>
                ))}
              </div>
            </>
          )}
        </div>
      </div>

      {/* Monthly Revenue */}
      <div className="rounded-xl border border-border bg-card p-5 mb-6">
        <h2 className="text-section-title mb-4">Monthly Revenue ({currencySymbol()}K)</h2>
        {monthlyRev.isLoading ? <Skeleton className="h-48" /> : (
          <ResponsiveContainer width="100%" height={200}>
            <BarChart data={revenueData} margin={{ top: 5, right: 10, left: 0, bottom: 5 }}>
              <CartesianGrid strokeDasharray="3 3" stroke={GRID} />
              <XAxis dataKey="month" tick={{ fontSize: 11, fill: "hsl(0 0% 42%)" }} axisLine={false} tickLine={false} />
              <YAxis tick={{ fontSize: 11, fill: "hsl(0 0% 42%)" }} axisLine={false} tickLine={false} unit="K" />
              <Tooltip {...TOOLTIP_STYLE} formatter={(v: number) => [`${currencySymbol()}${v}K`, "Revenue"]} />
              <Bar dataKey="revenue" fill={PRIMARY} radius={[4, 4, 0, 0]} />
            </BarChart>
          </ResponsiveContainer>
        )}
      </div>

      {/* Recent Activity */}
      <div className="rounded-xl border border-border bg-card">
        <div className="p-4 border-b border-border flex items-center justify-between">
          <h2 className="text-section-title">Recent Activity</h2>
          <a href="/audit-logs" className="text-sm font-medium text-primary hover:underline">
            View All Activity →
          </a>
        </div>
        <div className="divide-y divide-border">
          {recentActivity.isLoading
            ? Array(5).fill(0).map((_, i) => <div key={i} className="px-4 py-3"><Skeleton className="h-4" /></div>)
            : (recentActivity.data ?? []).slice(0, 10).map((a: { id: string; event_type: string; details?: Record<string, string>; created_at: string }) => (
              <div key={a.id} className="px-4 py-3 flex items-center gap-3 hover:bg-table-hover transition-colors">
                <span className="text-lg">📋</span>
                <p className="text-sm text-foreground flex-1">{a.event_type.replace(/_/g, " ")}{a.details?.email ? ` — ${a.details.email}` : ""}</p>
                <span className="text-caption whitespace-nowrap">{new Date(a.created_at).toLocaleDateString()}</span>
              </div>
            ))
          }
        </div>
      </div>
    </div>
  );
}
