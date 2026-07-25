import { api } from "@/lib/api";

export interface AdminProfile {
  id: string;
  email: string;
  first_name: string;
  last_name: string;
  phone: string;
  avatar_url?: string | null;
  created_at?: string;
}

export const authService = {
  getMe: () => api.get("/auth/me").then((r) => r.data.data as AdminProfile),
  updateProfile: (data: { first_name?: string; last_name?: string; phone?: string }) =>
    api.patch("/auth/me", data).then((r) => r.data.data as AdminProfile),
  changePassword: (current_password: string, new_password: string) =>
    api.post("/auth/change-password", { current_password, new_password }).then((r) => r.data),
  uploadAvatar: (file: Blob) => {
    const fd = new FormData();
    fd.append("file", file, "avatar.jpg");
    return api.post("/auth/avatar", fd).then((r) => r.data.data as { avatar_url: string });
  },
  removeAvatar: () => api.delete("/auth/avatar").then((r) => r.data),
};
