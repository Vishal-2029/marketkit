import { api } from "@/lib/api";

export interface UserParams {
  page?: number;
  limit?: number;
  search?: string;
  status?: string;
  app_mode?: string;
}

export const usersService = {
  list: (params?: UserParams) => api.get("/users", { params }).then(r => r.data),
  get: (id: string) => api.get(`/users/${id}`).then(r => r.data.data),
  create: (data: { name: string; email: string; phone?: string; password?: string; free_plan_id?: string; duration_days?: number }) =>
    api.post("/users", data).then(r => r.data.data),
  sendOtp: (email: string, name: string) =>
    api.post("/users/send-otp", { email, name }).then(r => r.data),
  verifyOtp: (email: string, otp: string) =>
    api.post("/users/verify-otp", { email, otp }).then(r => r.data),
  update: (id: string, data: { name?: string; status?: string }) =>
    api.patch(`/users/${id}`, data).then(r => r.data.data),
  delete: (id: string) => api.delete(`/users/${id}`),
  forceLogout: (id: string) => api.post(`/users/${id}/force-logout`),
  changePlan: (id: string, plan_id: string) =>
    api.post(`/users/${id}/change-plan`, { plan_id }),
  assignFreePlan: (id: string, plan_id: string, duration_days?: number) =>
    api.post(`/users/${id}/assign-free-plan`, { plan_id, ...(duration_days ? { duration_days } : {}) }),
  cancelFreePlan: (id: string) => api.post(`/users/${id}/cancel-free-plan`),
};
