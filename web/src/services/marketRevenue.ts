import { api } from "@/lib/api";

export interface MarketRevenueSummary {
  platform_revenue_minor: number;
  plan_revenue_minor: number;
  fee_revenue_minor: number;
  gross_sales_minor: number;
  seller_payouts_minor: number;
  plan_count: number;
  sale_count: number;
}

export interface MarketRevenueMonth {
  month: number;
  plan_revenue_minor: number;
  fee_revenue_minor: number;
  total_revenue_minor: number;
}

export const marketRevenueService = {
  summary: () => api.get("/market/revenue/summary").then(r => r.data.data as MarketRevenueSummary),
  monthly: (year?: number) =>
    api.get("/market/revenue/monthly", { params: { year } }).then(r => r.data.data as MarketRevenueMonth[]),
};
