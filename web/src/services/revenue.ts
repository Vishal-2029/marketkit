import { api } from "@/lib/api";

export interface RenewalStats {
  total_expired_90d: number;
  renewals_90d: number;
  renewal_rate_pct: number;
  churn_rate_pct: number;
  forecast_by_plan: { plan_name: string; expiring_count: number; expected_value_paise: number }[];
}

export const revenueService = {
  summary: () => api.get("/revenue/summary").then(r => r.data.data),
  monthly: (year?: number) => api.get("/revenue/monthly", { params: { year } }).then(r => r.data.data),
  byPlan: () => api.get("/revenue/by-plan").then(r => r.data.data),
  forecast: () => api.get("/revenue/forecast").then(r => r.data.data),
  renewalStats: () => api.get("/revenue/renewal-stats").then(r => r.data.data as RenewalStats),
};
