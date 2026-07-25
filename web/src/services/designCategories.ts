import { api } from "@/lib/api";

export interface DesignCategory {
  id: string;
  parent_id: string | null;
  name: string;
  display_order: number;
  is_other: boolean;
  created_at: string;
  design_count: number;
  photo_url?: string;
}

export interface DesignCategoryPayload {
  name?: string;
  parent_id?: string | null;
  display_order?: number;
}

export const designCategoriesService = {
  list: () => api.get("/market/categories").then(r => r.data.data as DesignCategory[]),
  create: (data: DesignCategoryPayload) =>
    api.post("/market/categories", data).then(r => r.data.data as DesignCategory),
  update: (id: string, data: DesignCategoryPayload) =>
    api.put(`/market/categories/${id}`, data).then(r => r.data.data as DesignCategory),
  delete: (id: string) => api.delete(`/market/categories/${id}`),
  uploadPhoto: (id: string, file: File) => {
    const form = new FormData();
    form.append("photo", file);
    return api.post(`/market/categories/${id}/photo`, form, {
      headers: { "Content-Type": "multipart/form-data" },
    }).then(r => r.data.data as DesignCategory);
  },
};
