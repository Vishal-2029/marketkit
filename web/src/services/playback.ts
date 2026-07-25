import { api } from "@/lib/api";

export const playbackService = {
  summary: () => api.get("/playback/summary").then(r => r.data.data),
  topVideos: (period?: "7d" | "30d") =>
    api.get("/playback/top-videos", { params: { period } }).then(r => r.data.data),
  byUser: () => api.get("/playback/by-user").then(r => r.data.data),
  dailyTrend: () => api.get("/playback/daily-trend").then(r => r.data.data),
};
