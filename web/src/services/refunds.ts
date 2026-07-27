import { api } from "@/lib/api";

export interface RefundRequest {
  id: string;
  payment_id: string;
  reason: string;
  status: "PENDING" | "APPROVED" | "REJECTED";
  review_note: string;
  refund_id: string;
  created_at: string;
  reviewed_at?: string;
  payment?: {
    amount_in_paise: number;
    provider: string;
    provider_payment_id?: string;
    user?: { name: string; email: string };
    plan?: { name: string };
  };
  requested_by_admin?: { first_name: string; last_name: string; email: string };
  reviewed_by_admin?: { first_name: string; last_name: string; email: string };
}

export const refundsService = {
  list: (params?: { page?: number; status?: string }) =>
    api.get("/refunds", { params }).then(r => r.data),
  request: (paymentId: string, reason: string) =>
    api.post(`/payments/${paymentId}/request-refund`, { reason }).then(r => r.data.data),
  approve: (id: string, note?: string) =>
    api.post(`/refunds/${id}/approve`, { note: note ?? "" }).then(r => r.data.data),
  reject: (id: string, note: string) =>
    api.post(`/refunds/${id}/reject`, { note }).then(r => r.data.data),
};
