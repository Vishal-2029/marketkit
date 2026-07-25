import { api } from "@/lib/api";

export const dashboardService = {
  getStats: () => api.get("/dashboard/stats").then(r => r.data.data),
  getRecentPayments: () => api.get("/dashboard/recent-payments").then(r => r.data.data),
  getRecentActivity: () => api.get("/dashboard/recent-activity").then(r => r.data.data),
  getUserGrowth: () => api.get("/dashboard/user-growth").then(r => r.data.data),
  getPlanDistribution: () => api.get("/dashboard/plan-distribution").then(r => r.data.data),
  getMonthlyRevenue: (year?: number) =>
    api.get("/revenue/monthly", { params: { year } }).then(r => r.data.data),
};
