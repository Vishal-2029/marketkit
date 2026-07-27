import { api } from "@/lib/api";

export interface PaymentParams {
  page?: number;
  limit?: number;
  search?: string;
  status?: string;
  provider?: string;
}

export const paymentsService = {
  list: (params?: PaymentParams) => api.get("/payments", { params }).then(r => r.data),
  get: (id: string) => api.get(`/payments/${id}`).then(r => r.data.data),
  createManual: (data: { user_id: string; plan_id: string; notes?: string }) =>
    api.post("/payments/manual", data).then(r => r.data.data),
  activate: (id: string) => api.post(`/payments/${id}/activate`),
};
