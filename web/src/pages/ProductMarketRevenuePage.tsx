import { currencySymbol, formatMoney, toMinor } from "@/lib/currency";
import { useQuery } from "@tanstack/react-query";
import { PageHeader } from "@/components/PageHeader";
import { StatCard } from "@/components/StatCard";
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer } from "recharts";
import { marketRevenueService } from "@/services/marketRevenue";
import { Skeleton } from "@/components/ui/skeleton";
import { PRIMARY, SECONDARY, ACTIVE, GRID, SERIES } from "@/lib/chartColors";

const MONTH_NAMES = ["", "Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];
const TOOLTIP_STYLE = {
  contentStyle: { background: "hsl(0 0% 100%)", border: "1px solid hsl(240 6% 90%)", borderRadius: "10px", fontSize: "12px", color: "hsl(240 6% 12%)" },
};

const fmt = (v: number) => formatMoney(v);

export default function ProductMarketRevenuePage() {
  const summary = useQuery({ queryKey: ["market-revenue-summary"], queryFn: marketRevenueService.summary });
  const monthly = useQuery({ queryKey: ["market-revenue-monthly"], queryFn: () => marketRevenueService.monthly() });

  const s = summary.data;

  const monthlyData = (monthly.data ?? []).map(r => ({
    month: MONTH_NAMES[r.month],
    "Plan revenue": Math.round(r.plan_revenue_minor / 100),
    "Fee revenue": Math.round(r.fee_revenue_minor / 100),
  }));

  return (
    <div>
      <PageHeader
        title="Product Market Revenue"
        subtitle="Platform revenue from the Product Market only — plan subscriptions and sale fees. Learning revenue never appears here."
      />

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
        {summary.isLoading ? Array(4).fill(0).map((_, i) => <Skeleton key={i} className="h-24 rounded-xl" />) : (
          <>
            <StatCard label="Platform Revenue" value={fmt(s?.platform_revenue_minor ?? 0)}
              subInfo="Plan revenue + sale fees" />
            <StatCard label="Plan Revenue" value={fmt(s?.plan_revenue_minor ?? 0)}
              subInfo={`${s?.plan_count ?? 0} paid subscriptions`} />
            <StatCard label="Product Sale Fees" value={fmt(s?.fee_revenue_minor ?? 0)}
              subInfo={`${s?.sale_count ?? 0} sales`} />
            <StatCard label="Gross Sales Volume" value={fmt(s?.gross_sales_minor ?? 0)}
              subInfo="Context only — not platform revenue" />
          </>
        )}
      </div>

      <div className="rounded-xl border border-border bg-card p-6 mb-8">
        <h2 className="text-section-title mb-1">Monthly Platform Revenue ({currencySymbol()})</h2>
        <p className="text-xs text-muted-foreground mb-6">Plan revenue and product-sale fees — the two sources credited to the platform wallet</p>
        {monthly.isLoading ? <Skeleton className="h-48" /> : (
          <ResponsiveContainer width="100%" height={260}>
            <BarChart data={monthlyData} margin={{ top: 5, right: 20, left: 0, bottom: 5 }}>
              <CartesianGrid strokeDasharray="3 3" stroke={GRID} />
              <XAxis dataKey="month" tick={{ fontSize: 12, fill: "hsl(0 0% 42%)" }} axisLine={false} tickLine={false} />
              <YAxis tick={{ fontSize: 12, fill: "hsl(0 0% 42%)" }} axisLine={false} tickLine={false} />
              <Tooltip {...TOOLTIP_STYLE} formatter={(v: number) => formatMoney(toMinor(v))} />
              <Legend wrapperStyle={{ fontSize: "12px" }} />
              <Bar dataKey="Plan revenue" stackId="a" fill={PRIMARY} radius={[0, 0, 0, 0]} />
              <Bar dataKey="Fee revenue" stackId="a" fill={SECONDARY} radius={[5, 5, 0, 0]} />
            </BarChart>
          </ResponsiveContainer>
        )}
      </div>

      <div className="rounded-xl border border-border bg-card p-6">
        <h2 className="text-section-title mb-4">Seller Payouts (context)</h2>
        <p className="text-xs text-muted-foreground mb-4">
          Net amount paid out to sellers from product sales — this is the sellers' money, not platform revenue.
        </p>
        <p className="text-2xl font-bold text-foreground">{fmt(s?.seller_payouts_minor ?? 0)}</p>
      </div>
    </div>
  );
}
