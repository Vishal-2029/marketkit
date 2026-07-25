import { api } from "@/lib/api";

export interface PlanPayload {
  name?: string;
  description?: string;
  price_in_paise?: number;
  has_willcom?: boolean;
  has_e4?: boolean;
  has_mecad?: boolean;
  duration_days?: number;
  is_active?: boolean;
}

export const plansService = {
  list: () => api.get("/plans").then(r => r.data.data),
  create: (data: PlanPayload) => api.post("/plans", data).then(r => r.data.data),
  update: (id: string, data: PlanPayload) => api.patch(`/plans/${id}`, data).then(r => r.data.data),
  delete: (id: string) => api.delete(`/plans/${id}`),
};
