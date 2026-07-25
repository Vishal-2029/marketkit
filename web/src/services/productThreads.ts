import { api } from "@/lib/api";

export interface ProductThreadMessage {
  id: string;
  user_name: string;
  content: string;
  created_at: string;
  is_admin: boolean;
}

export const productThreadsService = {
  listMessages: (purchaseId: string) =>
    api
      .get(`/market/purchases/${purchaseId}/messages`)
      .then((r) => r.data.data as ProductThreadMessage[]),
  reply: (purchaseId: string, content: string) =>
    api
      .post(`/market/purchases/${purchaseId}/messages`, { content })
      .then((r) => r.data.data as ProductThreadMessage),
};
