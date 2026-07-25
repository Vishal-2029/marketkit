import { api } from "@/lib/api";

export interface MarketRevenueSummary {
  platform_revenue_paise: number;
  plan_revenue_paise: number;
  fee_revenue_paise: number;
  gross_sales_paise: number;
  seller_payouts_paise: number;
  plan_count: number;
  sale_count: number;
}

export interface MarketRevenueMonth {
  month: number;
  plan_revenue_paise: number;
  fee_revenue_paise: number;
  total_revenue_paise: number;
}

export const marketRevenueService = {
  summary: () => api.get("/market/revenue/summary").then(r => r.data.data as MarketRevenueSummary),
  monthly: (year?: number) =>
    api.get("/market/revenue/monthly", { params: { year } }).then(r => r.data.data as MarketRevenueMonth[]),
};
