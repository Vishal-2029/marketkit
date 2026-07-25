import { api } from "@/lib/api";

const STATIC_BASE = (() => {
  const url = import.meta.env.VITE_API_URL;
  if (!url) return window.location.origin;
  try { return new URL(url).origin; } catch { return window.location.origin; }
})();

// Builds the full URL for a photo given its file_key (e.g. "photos/uuid.jpg")
export const photoUrl = (fileKey: string) => `${STATIC_BASE}/uploads/${fileKey}`;

export const photosService = {
  list: (params?: { page?: number; limit?: number }) =>
    api.get("/photos", { params }).then(r => r.data),
  update: (id: string, data: { title?: string; category?: string; is_published?: boolean }) =>
    api.patch(`/photos/${id}`, data).then(r => r.data.data),
  delete: (id: string) => api.delete(`/photos/${id}`),
};

export interface PhotoCategoryItem { id: string; name: string; }

export const photoCategoriesService = {
  list: () => api.get("/photo-categories").then(r => r.data.data as PhotoCategoryItem[]),
  create: (name: string) => api.post("/photo-categories", { name }).then(r => r.data.data as PhotoCategoryItem),
  delete: (id: string) => api.delete(`/photo-categories/${id}`),
};
