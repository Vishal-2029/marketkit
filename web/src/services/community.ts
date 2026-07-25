import { api } from "@/lib/api";

export interface CommunityReply {
  id: string;
  anon_name: string;
  content: string;
  is_flagged: boolean;
  created_at: string;
  image_url?: string | null;
}

export interface CommunityPost {
  id: string;
  anon_name: string;
  category: string;
  title: string;
  content: string;
  reply_count: number;
  is_flagged: boolean;
  created_at: string;
  image_urls: string[];
  user_name?: string;
  user_email?: string;
  replies?: CommunityReply[];
}

export const communityService = {
  listPosts: (page = 1) =>
    api.get(`/community/posts?page=${page}`).then((r) => r.data.data as { posts: CommunityPost[]; total: number }),
  getPost: (id: string) =>
    api.get(`/community/posts/${id}`).then((r) => r.data.data as CommunityPost),
  createPost: (data: FormData) =>
    api.post("/community/posts", data).then((r) => r.data),
  deletePost: (id: string) => api.delete(`/community/posts/${id}`),
  deleteReply: (id: string) => api.delete(`/community/replies/${id}`),
  flagPost: (id: string) => api.patch(`/community/posts/${id}/flag`),
  createReply: (postId: string, content: string) =>
    api.post(`/community/posts/${postId}/replies`, { content }).then((r) => r.data),
};
