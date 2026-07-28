import { currencySymbol, formatMoney } from "@/lib/currency";
import { useQuery } from "@tanstack/react-query";
import { PageHeader } from "@/components/PageHeader";
import { StatCard } from "@/components/StatCard";
import { BarChart, Bar, LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from "recharts";
import { revenueService, RenewalStats } from "@/services/revenue";
import { Skeleton } from "@/components/ui/skeleton";

const MONTH_NAMES = ["","Jan","Feb","Mar","Apr","May","Jun","Jul","Aug","Sep","Oct","Nov","Dec"];
const TOOLTIP_STYLE = {
  contentStyle: { background: "hsl(0 0% 100%)", border: "1px solid hsl(36 14% 90%)", borderRadius: "10px", fontSize: "12px", color: "hsl(240 6% 12%)" },
};

const fmt = (v: number) => formatMoney(v);

export default function RevenuePage() {
  const summary = useQuery({ queryKey: ["revenue-summary"], queryFn: revenueService.summary });
  const monthly = useQuery({ queryKey: ["revenue-monthly"], queryFn: () => revenueService.monthly() });
  const byPlan = useQuery({ queryKey: ["revenue-by-plan"], queryFn: revenueService.byPlan });
  const forecast = useQuery({ queryKey: ["revenue-forecast"], queryFn: revenueService.forecast });
  const renewal = useQuery({ queryKey: ["revenue-renewal"], queryFn: revenueService.renewalStats });

  const s = summary.data;

  const monthlyData = (monthly.data ?? []).map((r: { month: number; revenue_minor: number }) => ({
    month: MONTH_NAMES[r.month],
    amount: Math.round(r.revenue_minor / 100000),
  }));

  const cumulativeData = (() => {
    let total = 0;
    return monthlyData.map((m: { month: string; amount: number }) => {
      total += m.amount;
      return { month: m.month, cumulative: total };
    });
  })();

  const totalRevForPct = (byPlan.data ?? []).reduce((sum: number, r: { revenue_minor: number }) => sum + Number(r.revenue_minor), 0);

  return (
    <div>
      <PageHeader
        title="Learning Revenue"
        subtitle="Learning subscription revenue only — excludes Product Market (see Product Market Revenue under PRODUCT MARKET)"
      />

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
        {summary.isLoading ? Array(4).fill(0).map((_,i)=><Skeleton key={i} className="h-24 rounded-xl"/>) : (
          <>
            <StatCard label="Total Revenue" value={fmt(s?.total_revenue_minor ?? 0)} />
            <StatCard label="This Month" value={fmt(s?.month_revenue_minor ?? 0)} trend="up" />
            <StatCard label="Active Subscriptions" value={Number(s?.active_subscriptions ?? 0).toLocaleString()} />
            <StatCard label="Avg. Revenue Per User" value={fmt(s?.arpu_minor ?? 0)} />
          </>
        )}
      </div>

      <div className="rounded-xl border border-border bg-card p-6 mb-8">
        <h2 className="text-section-title mb-6">Monthly Revenue ({currencySymbol()}K)</h2>
        {monthly.isLoading ? <Skeleton className="h-48" /> : (
          <ResponsiveContainer width="100%" height={240}>
            <BarChart data={monthlyData} margin={{ top: 5, right: 20, left: 0, bottom: 5 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="hsl(36 14% 90%)" />
              <XAxis dataKey="month" tick={{ fontSize: 12, fill: "hsl(0 0% 42%)" }} axisLine={false} tickLine={false} />
              <YAxis tick={{ fontSize: 12, fill: "hsl(0 0% 42%)" }} axisLine={false} tickLine={false} unit="K" />
              <Tooltip {...TOOLTIP_STYLE} formatter={(v: number) => [`${currencySymbol()}${v}K`, "Revenue"]} />
              <Bar dataKey="amount" fill="#b8965a" radius={[5, 5, 0, 0]} />
            </BarChart>
          </ResponsiveContainer>
        )}
      </div>

      <div className="rounded-xl border border-border bg-card p-6 mb-8">
        <h2 className="text-section-title mb-6">Cumulative Revenue Growth ({currencySymbol()}K)</h2>
        {monthly.isLoading ? <Skeleton className="h-44" /> : (
          <ResponsiveContainer width="100%" height={220}>
            <LineChart data={cumulativeData} margin={{ top: 5, right: 20, left: 0, bottom: 5 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="hsl(36 14% 90%)" />
              <XAxis dataKey="month" tick={{ fontSize: 12, fill: "hsl(0 0% 42%)" }} axisLine={false} tickLine={false} />
              <YAxis tick={{ fontSize: 12, fill: "hsl(0 0% 42%)" }} axisLine={false} tickLine={false} unit="K" />
              <Tooltip {...TOOLTIP_STYLE} formatter={(v: number) => [`${currencySymbol()}${v}K`, "Cumulative"]} />
              <Line type="monotone" dataKey="cumulative" stroke="#8a6d3b" strokeWidth={2.5} dot={{ fill: "#8a6d3b", r: 4 }} activeDot={{ r: 6, fill: "#b8965a" }} />
            </LineChart>
          </ResponsiveContainer>
        )}
      </div>

      <div className="rounded-xl border border-border bg-card overflow-hidden mb-8">
        <div className="p-4 border-b border-border"><h2 className="text-section-title">Revenue by Plan</h2></div>
        <table className="w-full">
          <thead>
            <tr className="bg-table-header">
              <th className="text-table-header text-left px-4 py-3">Plan</th>
              <th className="text-table-header text-left px-4 py-3">Subscribers</th>
              <th className="text-table-header text-left px-4 py-3">Revenue</th>
              <th className="text-table-header text-left px-4 py-3">% of Total</th>
            </tr>
          </thead>
          <tbody>
            {byPlan.isLoading
              ? Array(3).fill(0).map((_,i)=><tr key={i}><td colSpan={4} className="px-4 py-3"><Skeleton className="h-4"/></td></tr>)
              : (byPlan.data ?? []).map((r: { plan_name: string; subscribers: number; revenue_minor: number }) => {
                const pct = totalRevForPct > 0 ? Math.round((Number(r.revenue_minor) / totalRevForPct) * 100) : 0;
                return (
                  <tr key={r.plan_name} className="border-b border-border last:border-0 hover:bg-table-hover transition-colors">
                    <td className="px-4 py-3 text-sm font-medium text-foreground">{r.plan_name}</td>
                    <td className="px-4 py-3 text-sm text-foreground">{r.subscribers}</td>
                    <td className="px-4 py-3 text-sm font-semibold text-foreground">{fmt(Number(r.revenue_minor))}</td>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-2">
                        <div className="w-20 h-2 rounded-full bg-muted overflow-hidden">
                          <div className="h-full rounded-full bg-primary" style={{ width: `${pct}%` }} />
                        </div>
                        <span className="text-sm text-muted-foreground">{pct}%</span>
                      </div>
                    </td>
                  </tr>
                );
              })
            }
          </tbody>
        </table>
      </div>

      <div className="rounded-xl border border-border bg-card overflow-hidden">
        <div className="p-5 border-b border-border">
          <h2 className="text-section-title">Renewal Forecast (Next 30 Days)</h2>
          <p className="text-xs text-muted-foreground mt-0.5">Rates based on subscription behaviour in the last 90 days</p>
        </div>

        {/* Rate metrics */}
        {renewal.isLoading ? (
          <div className="p-5"><Skeleton className="h-20" /></div>
        ) : (() => {
          const rs: RenewalStats = renewal.data ?? { total_expired_90d: 0, renewals_90d: 0, renewal_rate_pct: 0, churn_rate_pct: 0, forecast_by_plan: [] };
          const forecastRows = forecast.data ?? [];
          const totalExpectedMinor = forecastRows.reduce((s: number, r: { value_minor: number }) => s + r.value_minor, 0);
          const adjustedMinor = Math.round(totalExpectedMinor * rs.renewal_rate_pct / 100);
          const hasHistory = rs.total_expired_90d > 0;

          return (
            <>
              <div className="grid grid-cols-3 divide-x divide-border border-b border-border">
                <div className="p-5 text-center">
                  <p className="text-2xl font-bold text-foreground">
                    {hasHistory ? `${rs.renewal_rate_pct}%` : "—"}
                  </p>
                  <p className="text-xs text-muted-foreground mt-1">Renewal Rate</p>
                  {hasHistory && (
                    <p className="text-[11px] text-muted-foreground mt-0.5">
                      {rs.renewals_90d} of {rs.total_expired_90d} renewed
                    </p>
                  )}
                </div>
                <div className="p-5 text-center">
                  <p className={`text-2xl font-bold ${hasHistory && rs.churn_rate_pct > 30 ? "text-danger" : "text-foreground"}`}>
                    {hasHistory ? `${rs.churn_rate_pct}%` : "—"}
                  </p>
                  <p className="text-xs text-muted-foreground mt-1">Churn Rate</p>
                  {hasHistory && (
                    <p className="text-[11px] text-muted-foreground mt-0.5">
                      {rs.total_expired_90d - rs.renewals_90d} did not renew
                    </p>
                  )}
                </div>
                <div className="p-5 text-center">
                  <p className="text-2xl font-bold text-foreground">{forecastRows.length}</p>
                  <p className="text-xs text-muted-foreground mt-1">Expiring in 30 days</p>
                  {forecastRows.length > 0 && (
                    <p className="text-[11px] text-muted-foreground mt-0.5">
                      {fmt(totalExpectedMinor)} at stake
                    </p>
                  )}
                </div>
              </div>

              {/* Expected value summary */}
              <div className="px-5 py-4 border-b border-border bg-muted/30 flex flex-wrap gap-6">
                <div>
                  <p className="text-xs text-muted-foreground mb-0.5">If all renew</p>
                  <p className="text-base font-semibold text-foreground">{fmt(totalExpectedMinor)}</p>
                </div>
                <div>
                  <p className="text-xs text-muted-foreground mb-0.5">
                    Adjusted ({hasHistory ? `${rs.renewal_rate_pct}% rate` : "no history"})
                  </p>
                  <p className={`text-base font-semibold ${hasHistory ? "text-primary" : "text-muted-foreground"}`}>
                    {hasHistory ? fmt(adjustedMinor) : "—"}
                  </p>
                </div>
                {!hasHistory && (
                  <p className="text-xs text-muted-foreground self-center italic">
                    No expiry history in the last 90 days — rate unavailable
                  </p>
                )}
              </div>

              {/* Per-plan breakdown */}
              {rs.forecast_by_plan.length > 0 && (
                <table className="w-full">
                  <thead>
                    <tr className="bg-table-header">
                      <th className="text-table-header text-left px-5 py-3">Plan</th>
                      <th className="text-table-header text-left px-5 py-3">Expiring</th>
                      <th className="text-table-header text-left px-5 py-3">Expected (full)</th>
                      <th className="text-table-header text-left px-5 py-3">Adjusted</th>
                    </tr>
                  </thead>
                  <tbody>
                    {rs.forecast_by_plan.map(row => {
                      const adj = hasHistory ? Math.round(row.expected_value_minor * rs.renewal_rate_pct / 100) : null;
                      return (
                        <tr key={row.plan_name} className="border-b border-border last:border-0 hover:bg-table-hover transition-colors">
                          <td className="px-5 py-3 text-sm font-medium text-foreground">{row.plan_name}</td>
                          <td className="px-5 py-3 text-sm text-foreground">{row.expiring_count}</td>
                          <td className="px-5 py-3 text-sm text-foreground">{fmt(row.expected_value_minor)}</td>
                          <td className="px-5 py-3 text-sm font-medium text-primary">
                            {adj !== null ? fmt(adj) : <span className="text-muted-foreground">—</span>}
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              )}

              {rs.forecast_by_plan.length === 0 && forecastRows.length === 0 && (
                <p className="text-sm text-muted-foreground text-center py-8">No subscriptions expiring in the next 30 days.</p>
              )}
            </>
          );
        })()}
      </div>
    </div>
  );
}
