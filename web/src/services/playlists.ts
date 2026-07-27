import type { ContentCategory } from "@/lib/featureCatalog";
import { api } from "@/lib/api";

export interface AdminPlaylist {
  id: string;
  name: string;
  description: string;
  thumbnail_url: string;
  category: "" | ContentCategory;
  video_count: number;
  created_at: string;
}

export const playlistsService = {
  list: () => api.get("/admin/playlists").then(r => r.data.data as AdminPlaylist[]),
  create: (data: { name: string; description: string; thumbnail_url: string; category: string }) =>
    api.post("/admin/playlists", data).then(r => r.data.data as AdminPlaylist),
  update: (id: string, data: Partial<{ name: string; description: string; thumbnail_url: string; category: string }>) =>
    api.patch(`/admin/playlists/${id}`, data).then(r => r.data.data as AdminPlaylist),
  delete: (id: string) => api.delete(`/admin/playlists/${id}`),
  setVideos: (id: string, video_ids: string[]) =>
    api.put(`/admin/playlists/${id}/videos`, { video_ids }).then(r => r.data),
  getVideos: (id: string) =>
    api.get(`/admin/playlists/${id}`).then(r => r.data.data),
};
