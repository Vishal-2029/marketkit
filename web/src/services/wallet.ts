import { api } from "@/lib/api";

export interface Withdrawal {
  id: string;
  amount_in_paise: number;
  method: "UPI" | "BANK";
  upi_id?: string;
  bank_account_number?: string;
  bank_ifsc?: string;
  bank_holder_name?: string;
  status: "APPROVED" | "SETTLED";
  settled_by?: string;
  settled_at?: string;
  note: string;
  created_at: string;
  user_name?: string;
  user_email?: string;
}

export interface WalletTransaction {
  id: string;
  type: "TOPUP" | "PURCHASE_DEBIT" | "SALE_CREDIT" | "WITHDRAWAL" | string;
  amount_in_paise: number;
  balance_after_in_paise: number;
  reference_id?: string;
  created_at: string;
}

export interface WalletSettings {
  fee_percent: number;
  min_withdrawal_in_paise: number;
}

export const withdrawalsService = {
  list: (params?: { page?: number; status?: string }) =>
    api.get("/wallet/withdrawals", { params }).then(r => r.data),
  settle: (id: string, note?: string) =>
    api.post(`/wallet/withdrawals/${id}/settle`, { note: note ?? "" }).then(r => r.data.data),
};

export const walletService = {
  listUserTransactions: (userId: string, params?: { page?: number; limit?: number }) =>
    api.get(`/wallet/users/${userId}/transactions`, { params }).then(r => r.data),
};

export const walletSettingsService = {
  get: () => api.get("/wallet/settings").then(r => r.data.data as WalletSettings),
  update: (settings: WalletSettings) =>
    api.put("/wallet/settings", settings).then(r => r.data.data as WalletSettings),
};
