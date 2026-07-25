import { api } from "@/lib/api";

export interface AuditLogParams {
  page?: number;
  limit?: number;
  event_type?: string;
  start_date?: string;
  end_date?: string;
}

export const auditLogsService = {
  list: (params?: AuditLogParams) => api.get("/audit-logs", { params }).then(r => r.data),
};
