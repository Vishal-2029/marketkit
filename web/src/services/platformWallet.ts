import { api } from "@/lib/api";

export interface PlatformWalletBreakdown {
  learning_plan_minor: number;
  market_plan_minor: number;
  platform_fee_minor: number;
  withdrawal_minor: number;
}

export interface PlatformWalletSummary {
  balance_minor: number;
  breakdown: PlatformWalletBreakdown;
}

export interface PlatformLedgerRow {
  id: string;
  type: "CREDIT" | "DEBIT";
  source: "LEARNING_PLAN" | "MARKET_PLAN" | "PLATFORM_FEE" | "WITHDRAWAL" | string;
  amount_minor: number;
  balance_after_minor: number;
  reference_id?: string;
  metadata?: Record<string, unknown>;
  created_at: string;
}

export const platformWalletService = {
  get: () => api.get("/platform-wallet").then(r => r.data.data as PlatformWalletSummary),
  transactions: (params?: { page?: number; limit?: number }) =>
    api.get("/platform-wallet/transactions", { params }).then(r => r.data),
  withdraw: (payload: { amount_minor: number; note?: string }) =>
    api.post("/platform-wallet/withdrawals", payload).then(r => r.data.data),
  // Auth is a Bearer header, not a cookie, so exports must be fetched via
  // axios (which attaches it) rather than a plain <a href> to the endpoint.
  exportCsv: () =>
    api.get("/platform-wallet/transactions", { params: { format: "csv" }, responseType: "blob" })
      .then(r => r.data as Blob),
  exportPdf: () =>
    api.get("/platform-wallet/transactions", { params: { format: "pdf" }, responseType: "blob" })
      .then(r => r.data as Blob),
};
