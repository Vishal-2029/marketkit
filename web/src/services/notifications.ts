import { api } from "@/lib/api";

export interface NotificationLog {
  id: number;
  title: string;
  body: string;
  audience: string;
  sent_count: number;
  failed_count: number;
  created_at: string;
}

export interface Plan {
  id: string;
  name: string;
  is_active: boolean;
}

export const notificationsService = {
  broadcast: (title: string, message: string, planId?: string, subStatus?: string) =>
    api.post("/admin/notifications/broadcast", {
      title,
      message,
      ...(planId    && { plan_id: planId }),
      ...(subStatus && { sub_status: subStatus }),
    }),
  listNotifications: (page = 1) =>
    api
      .get(`/admin/notifications?page=${page}`)
      .then((r) => r.data.data as { logs: NotificationLog[]; total: number }),
  deleteNotification: (id: number) => api.delete(`/admin/notifications/${id}`),
  clearNotifications: () => api.delete("/admin/notifications"),
  listPlans: () =>
    api.get("/plans/").then((r) => r.data.data as Plan[]),
};
