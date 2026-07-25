import { api } from "@/lib/api";

export interface Design {
  id: string;
  title: string;
  description: string;
  price_in_paise: number;
  file_name: string;
  file_format: string;
  file_size_bytes: number;
  preview_urls: string[];
  is_active: boolean;
  sales_count: number;
  created_at: string;
  seller_name?: string;
  seller_email?: string;
  category_id?: string | null;
  category?: { id: string; parent_id: string | null; name: string; is_other: boolean } | null;
}

export interface DesignPurchase {
  id: string;
  design_id: string;
  amount_in_paise: number;
  currency: string;
  status: string;
  paid_at?: string;
  created_at: string;
  buyer_name?: string;
  buyer_email?: string;
  seller_name?: string;
  seller_email?: string;
  design_title?: string;
}

export interface PaginatedMeta {
  page: number;
  limit: number;
  total: number;
  pages: number;
}

export interface MarketUser {
  id: string;
  name: string;
  email: string;
  phone: string;
  design_count: number;
  purchase_count: number;
  total_income_in_paise: number;
  seller_income_in_paise: number;
  platform_income_in_paise: number;
}

export interface MarketUserDesign {
  id: string;
  title: string;
  price_in_paise: number;
  preview_urls: string[];
  sell_count: number;
  user_profit_in_paise: number;
  pf_profit_in_paise: number;
}

export interface MarketUserPurchase {
  id: string;
  design_id: string;
  design_title: string;
  preview_url?: string;
  amount_in_paise: number;
  fee_in_paise: number;
  seller_name: string;
  gateway: string;
  paid_at?: string;
}

export const designsService = {
  listDesigns: (page = 1, search = "", categoryId?: string) =>
    api
      .get("/market/designs", {
        params: { page, search: search || undefined, category_id: categoryId || undefined },
      })
      .then((r) => r.data as { data: Design[]; meta: PaginatedMeta }),
  deleteDesign: (id: string) => api.delete(`/market/designs/${id}`),
  listPurchases: (page = 1) =>
    api
      .get("/market/purchases", { params: { page } })
      .then((r) => r.data as { data: DesignPurchase[]; meta: PaginatedMeta }),
  listMarketUsers: (role: "seller" | "buyer" | undefined, page = 1, search = "") =>
    api
      .get("/market/users", { params: { role, page, search: search || undefined } })
      .then((r) => r.data as { data: MarketUser[]; meta: PaginatedMeta }),
  getMarketUserDesigns: (id: string) =>
    api
      .get(`/market/users/${id}/designs`)
      .then(
        (r) =>
          r.data.data as {
            user: { id: string; name: string; email: string; phone: string };
            designs_sold: MarketUserDesign[];
            purchases: MarketUserPurchase[];
          }
      ),
};
