import { api } from "@/lib/api";

export interface AdminParams {
  page?: number;
  limit?: number;
  search?: string;
}

export interface AdminRecord {
  id: string;
  first_name: string;
  last_name: string;
  email: string;
  phone?: string;
  is_super?: boolean;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export const adminsService = {
  list: (params?: AdminParams) => api.get("/admins", { params }).then((r) => r.data),
  create: (payload: { first_name: string; last_name?: string; phone?: string; email: string; password: string; is_super?: boolean }) =>
    api.post("/admins", payload).then((r) => r.data),
  delete: (id: string) => api.delete(`/admins/${id}`),
  setSuper: (id: string, isSuper: boolean) => api.patch(`/admins/${id}/super`, { is_super: isSuper }).then((r) => r.data),
};
