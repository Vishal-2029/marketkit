import { api } from "@/lib/api";

export interface MarketPlanPayload {
  name?: string;
  description?: string;
  price_minor?: number;
  duration_days?: number;
  fee_discount_pct?: number;
  featured_seller?: boolean;
  is_active?: boolean;
}

export const marketPlansService = {
  list: () => api.get("/market/plans").then(r => r.data.data),
  create: (data: MarketPlanPayload) => api.post("/market/plans", data).then(r => r.data.data),
  update: (id: string, data: MarketPlanPayload) => api.put(`/market/plans/${id}`, data).then(r => r.data.data),
  delete: (id: string) => api.delete(`/market/plans/${id}`),
};
