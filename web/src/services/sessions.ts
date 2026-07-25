import { api } from "@/lib/api";

export const sessionsService = {
  list: () => api.get("/sessions").then(r => r.data.data),
  revoke: (id: string) => api.delete(`/sessions/${id}`),
  revokeAll: () => api.delete("/sessions"),
  backfillLocations: () => api.post("/sessions/backfill-locations").then(r => r.data.data),
};
