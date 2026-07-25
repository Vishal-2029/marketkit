import { api } from "@/lib/api";

export interface MarketPlanPayment {
  id: string;
  user_id: string;
  plan_id: string;
  status: string;
  start_date: string;
  expiry_date: string;
  amount_in_paise: number;
  razorpay_order_id?: string;
  razorpay_payment_id?: string;
  paid_at?: string;
  created_at: string;
  gateway: "RAZORPAY" | "WALLET" | string;
  user?: { id: string; name: string; email: string };
  plan?: { id: string; name: string };
}

export const marketPlanPaymentsService = {
  list: (params?: {
    page?: number;
    limit?: number;
    search?: string;
    status?: string;
  }) =>
    api
      .get("/market/plan-payments", { params })
      .then(
        (r) =>
          r.data as {
            data: MarketPlanPayment[];
            meta: {
              page: number;
              limit: number;
              total: number;
              pages: number;
            };
          }
      ),
};
