import { api } from "@/lib/api";

export interface VideoParams {
  page?: number;
  limit?: number;
  search?: string;
  category?: string;
  status?: string;
  is_free?: boolean;
}


export interface EngagementStats {
  like_count: number;
  dislike_count: number;
  comment_count: number;
}

export interface VideoComment {
  id: string;
  user_name: string;
  content: string;
  created_at: string;
  thread_user_id: string;
  is_admin: boolean;
}

export const videosService = {
  list: (params?: VideoParams) => api.get("/videos", { params }).then(r => r.data),
  get: (id: string) => api.get(`/videos/${id}`).then(r => r.data.data),
  create: (data: object, onProgress?: (percent: number) => void) =>
    api.post("/videos", data, {
      timeout: 0,
      onUploadProgress: onProgress
        ? (e) => onProgress(e.total ? Math.round((e.loaded / e.total) * 100) : 0)
        : undefined,
    }).then(r => r.data.data),
  update: (id: string, data: object) => api.patch(`/videos/${id}`, data).then(r => r.data.data),
  edit: (id: string, fd: FormData) => api.patch(`/videos/${id}`, fd).then(r => r.data.data),
  delete: (id: string) => api.delete(`/videos/${id}`),
  retry: (id: string) => api.post(`/videos/${id}/retry`),
  fetchStreamUrl: (id: string): Promise<string> =>
    api.get(`/videos/${id}/stream-url`).then(r => r.data.data.url),

  presignUpload: (filename: string, contentType: string): Promise<{ direct: boolean; upload_url?: string; file_key?: string }> =>
    api.get("/videos/presign-upload", { params: { filename, content_type: contentType } }).then(r => r.data.data),

  uploadPhotos: (id: string, data: FormData) =>
    api.post(`/videos/${id}/photos`, data, { timeout: 0 }).then(r => r.data.data),

  finalize: (data: FormData, onProgress?: (percent: number) => void) =>
    api.post("/videos/finalize", data, {
      timeout: 0,
      onUploadProgress: onProgress
        ? (e) => onProgress(e.total ? Math.round((e.loaded / e.total) * 100) : 0)
        : undefined,
    }).then(r => r.data.data),

  // Engagement (admin)
  getEngagement: (id: string): Promise<EngagementStats> =>
    api.get(`/videos/${id}/engagement`).then(r => r.data.data),
  getComments: (id: string, page = 1): Promise<VideoComment[]> =>
    api.get(`/videos/${id}/comments`, { params: { page } }).then(r => r.data.data),
  deleteComment: (videoId: string, commentId: string) =>
    api.delete(`/videos/${videoId}/comments/${commentId}`),
  replyToComment: (videoId: string, userId: string, content: string): Promise<VideoComment> =>
    api.post(`/videos/${videoId}/comments/reply`, { user_id: userId, content }).then(r => r.data.data),
};
