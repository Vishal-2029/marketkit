import { useEffect, useRef, useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Trash2, Flag, ChevronLeft, ChevronRight, Plus, X, ImagePlus, MessageSquareReply } from "lucide-react";
import { PageHeader } from "@/components/PageHeader";
import { communityService, CommunityPost } from "@/services/community";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogDescription, DialogFooter } from "@/components/ui/dialog";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";

const PAGE_SIZE = 50;
const CATEGORIES = ["GENERAL", "ANNOUNCEMENT", "TIP"];
const MAX_IMAGES = 3;
const MAX_IMAGE_BYTES = 5 * 1024 * 1024;

export default function CommunityPage() {
  const qc = useQueryClient();
  const [page, setPage] = useState(1);
  const [showForm, setShowForm] = useState(false);
  const [form, setForm] = useState({ category: "GENERAL", title: "", content: "" });
  const [images, setImages] = useState<File[]>([]);
  const [imagePreviews, setImagePreviews] = useState<string[]>([]);
  const [imageError, setImageError] = useState("");
  const fileRef = useRef<HTMLInputElement>(null);
  const [replyPostId, setReplyPostId] = useState<string | null>(null);
  const [replyText, setReplyText] = useState("");
  const [selectedPost, setSelectedPost] = useState<CommunityPost | null>(null);

  const { data, isLoading, error } = useQuery({
    queryKey: ["community-posts", page],
    queryFn: () => communityService.listPosts(page),
  });

  // Full post detail (including the reply thread), fetched when a row is opened.
  const { data: postDetail, isLoading: detailLoading } = useQuery({
    queryKey: ["community-post", selectedPost?.id],
    queryFn: () => communityService.getPost(selectedPost!.id),
    enabled: !!selectedPost,
  });

  const deletePost = useMutation({
    mutationFn: (id: string) => communityService.deletePost(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["community-posts"] }),
  });

  const flagPost = useMutation({
    mutationFn: (id: string) => communityService.flagPost(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["community-posts"] }),
  });

  const createReply = useMutation({
    mutationFn: ({ postId, content }: { postId: string; content: string }) =>
      communityService.createReply(postId, content),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["community-posts"] });
      qc.invalidateQueries({ queryKey: ["community-post"] });
      setReplyPostId(null);
      setReplyText("");
    },
  });

  const deleteReply = useMutation({
    mutationFn: (id: string) => communityService.deleteReply(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["community-posts"] });
      qc.invalidateQueries({ queryKey: ["community-post"] });
    },
  });

  const createPost = useMutation({
    mutationFn: () => {
      const fd = new FormData();
      fd.append("category", form.category);
      fd.append("title", form.title);
      fd.append("content", form.content);
      images.forEach((img) => fd.append("images", img));
      return communityService.createPost(fd);
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["community-posts"] });
      setPage(1);
      setShowForm(false);
      setForm({ category: "GENERAL", title: "", content: "" });
      setImages([]);
      setImageError("");
    },
  });

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setImageError("");
    const incoming = Array.from(e.target.files ?? []);
    const combined = [...images, ...incoming].slice(0, MAX_IMAGES);
    for (const f of combined) {
      if (f.size > MAX_IMAGE_BYTES) {
        setImageError(`"${f.name}" exceeds 5 MB limit.`);
        return;
      }
    }
    setImages(combined);
    if (fileRef.current) fileRef.current.value = "";
  };

  useEffect(() => {
    const urls = images.map(img => URL.createObjectURL(img));
    setImagePreviews(urls);
    return () => { urls.forEach(URL.revokeObjectURL); };
  }, [images]);

  const removeImage = (idx: number) => setImages((prev) => prev.filter((_, i) => i !== idx));

  const posts: CommunityPost[] = data?.posts ?? [];

  return (
    <div>
      <div className="flex items-start justify-between mb-6">
        <PageHeader
          title="Community"
          subtitle="Moderate community questions and replies"
        />
        <Button
          size="sm"
          className="mt-1 shrink-0"
          onClick={() => setShowForm((v) => !v)}
        >
          {showForm ? <X className="h-4 w-4 mr-1" /> : <Plus className="h-4 w-4 mr-1" />}
          {showForm ? "Cancel" : "New Post"}
        </Button>
      </div>

      {/* New Post Form */}
      {showForm && (
        <div className="rounded-xl border border-border bg-muted/20 p-5 mb-6 space-y-4">
          <p className="text-sm font-medium">Create Admin Post</p>
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-1">
              <label className="text-xs text-muted-foreground">Category</label>
              <select
                className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm"
                value={form.category}
                onChange={(e) => setForm((f) => ({ ...f, category: e.target.value }))}
              >
                {CATEGORIES.map((c) => (
                  <option key={c} value={c}>{c}</option>
                ))}
              </select>
            </div>
            <div className="space-y-1">
              <label className="text-xs text-muted-foreground">Title</label>
              <input
                className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm"
                placeholder="Post title"
                value={form.title}
                onChange={(e) => setForm((f) => ({ ...f, title: e.target.value }))}
              />
            </div>
          </div>
          <div className="space-y-1">
            <label className="text-xs text-muted-foreground">Content</label>
            <textarea
              className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm min-h-[100px] resize-y"
              placeholder="Write your message..."
              value={form.content}
              onChange={(e) => setForm((f) => ({ ...f, content: e.target.value }))}
            />
          </div>

          {/* Image upload */}
          <div className="space-y-2">
            <label className="text-xs text-muted-foreground">Photos (up to 3, max 5 MB each)</label>
            <div className="flex gap-2 flex-wrap">
              {images.map((img, idx) => (
                <div key={idx} className="relative group w-20 h-20">
                  <img
                    src={imagePreviews[idx] ?? ""}
                    alt=""
                    className="w-20 h-20 rounded-lg object-cover border border-border"
                  />
                  <button
                    type="button"
                    onClick={() => removeImage(idx)}
                    className="absolute -top-1.5 -right-1.5 bg-destructive text-white rounded-full w-5 h-5 flex items-center justify-center text-xs opacity-0 group-hover:opacity-100 transition-opacity"
                  >
                    ×
                  </button>
                </div>
              ))}
              {images.length < MAX_IMAGES && (
                <button
                  type="button"
                  onClick={() => fileRef.current?.click()}
                  className="w-20 h-20 rounded-lg border-2 border-dashed border-border flex flex-col items-center justify-center gap-1 text-muted-foreground hover:border-primary hover:text-primary transition-colors"
                >
                  <ImagePlus className="h-5 w-5" />
                  <span className="text-[10px]">Add photo</span>
                </button>
              )}
            </div>
            <input
              ref={fileRef}
              type="file"
              accept="image/*"
              multiple
              className="hidden"
              onChange={handleFileChange}
            />
            {imageError && <p className="text-xs text-destructive">{imageError}</p>}
          </div>

          <Button
            size="sm"
            disabled={!form.title.trim() || !form.content.trim() || createPost.isPending}
            onClick={() => createPost.mutate()}
          >
            {createPost.isPending ? "Posting..." : "Post"}
          </Button>
          {createPost.isError && (
            <p className="text-xs text-destructive">
              Failed to create post: {(createPost.error as Error).message}
            </p>
          )}
        </div>
      )}

      {/* API error banner */}
      {error && (
        <div className="rounded-lg border border-destructive/50 bg-destructive/10 text-destructive px-4 py-3 text-sm mb-4">
          Failed to load posts: {(error as Error).message}
        </div>
      )}

      {isLoading ? (
        <div className="space-y-3">
          {[...Array(5)].map((_, i) => (
            <Skeleton key={i} className="h-20 w-full rounded-xl" />
          ))}
        </div>
      ) : posts.length === 0 ? (
        <div className="text-center py-16 text-muted-foreground">
          No community posts yet.
        </div>
      ) : (
        <>
          <p className="text-sm text-muted-foreground mb-4">{data?.total ?? 0} total posts</p>
          <div className="rounded-xl border border-border overflow-hidden">
            <table className="w-full text-sm">
              <thead className="bg-muted/40">
                <tr>
                  <th className="text-left px-4 py-3 font-medium text-muted-foreground">Author</th>
                  <th className="text-left px-4 py-3 font-medium text-muted-foreground">Real Name</th>
                  <th className="text-left px-4 py-3 font-medium text-muted-foreground">Category</th>
                  <th className="text-left px-4 py-3 font-medium text-muted-foreground">Title</th>
                  <th className="text-left px-4 py-3 font-medium text-muted-foreground">Images</th>
                  <th className="text-left px-4 py-3 font-medium text-muted-foreground">Replies</th>
                  <th className="text-left px-4 py-3 font-medium text-muted-foreground">Posted</th>
                  <th className="text-left px-4 py-3 font-medium text-muted-foreground">Actions</th>
                </tr>
              </thead>
              <tbody>
                {posts.map((post) => (
                  <tr key={post.id} className={cn("border-t border-border hover:bg-muted/20 cursor-pointer", post.is_flagged && "bg-destructive/5")}
                    onClick={() => setSelectedPost(post)}>
                    <td className="px-4 py-3 text-xs text-primary font-medium">
                      {post.anon_name}
                      {post.is_flagged && (
                        <span className="ml-2 text-[10px] bg-destructive/20 text-destructive px-1.5 py-0.5 rounded-full">Flagged</span>
                      )}
                    </td>
                    <td className="px-4 py-3">
                      {post.user_name ? (
                        <div>
                          <p className="text-xs font-medium">{post.user_name}</p>
                          {post.user_email && <p className="text-[11px] text-muted-foreground">{post.user_email}</p>}
                        </div>
                      ) : <span className="text-muted-foreground text-xs">—</span>}
                    </td>
                    <td className="px-4 py-3">
                      <span className="text-xs bg-primary/10 text-primary px-2 py-1 rounded-full">{post.category}</span>
                    </td>
                    <td className="px-4 py-3 max-w-[220px]">
                      <p className="font-medium truncate">{post.title}</p>
                      <p className="text-muted-foreground text-xs truncate mt-0.5">{post.content}</p>
                    </td>
                    <td className="px-4 py-3">
                      {post.image_urls?.length > 0 ? (
                        <div className="flex gap-1">
                          {post.image_urls.slice(0, 3).map((url, i) => (
                            <a key={i} href={url} target="_blank" rel="noreferrer">
                              <img src={url} alt="" className="w-10 h-10 rounded object-cover border border-border hover:opacity-80 transition-opacity" />
                            </a>
                          ))}
                        </div>
                      ) : <span className="text-muted-foreground text-xs">—</span>}
                    </td>
                    <td className="px-4 py-3 text-muted-foreground">{post.reply_count}</td>
                    <td className="px-4 py-3 text-muted-foreground text-xs">{new Date(post.created_at).toLocaleDateString()}</td>
                    <td className="px-4 py-3">
                      <div className="flex flex-col gap-2">
                        <div className="flex gap-2">
                          <Button size="sm" variant="outline" className="h-7 px-2"
                            onClick={(e) => { e.stopPropagation(); setReplyPostId(replyPostId === post.id ? null : post.id); setReplyText(""); }}
                            title="Reply as Admin">
                            <MessageSquareReply className="h-3 w-3" />
                          </Button>
                          <Button size="sm" variant="outline" className="h-7 px-2"
                            onClick={(e) => { e.stopPropagation(); flagPost.mutate(post.id); }}
                            title={post.is_flagged ? "Unflag" : "Flag"}>
                            <Flag className="h-3 w-3" />
                          </Button>
                          <Button size="sm" variant="destructive" className="h-7 px-2"
                            onClick={(e) => { e.stopPropagation(); if (confirm("Delete this post and all its replies?")) deletePost.mutate(post.id); }}>
                            <Trash2 className="h-3 w-3" />
                          </Button>
                        </div>
                        {replyPostId === post.id && (
                          <div className="flex gap-2 items-start min-w-[260px]">
                            <textarea
                              className="flex-1 rounded-md border border-border bg-background px-3 py-2 text-sm min-h-[64px] resize-y"
                              placeholder="Write your admin reply..."
                              value={replyText}
                              onChange={(e) => setReplyText(e.target.value)}
                              autoFocus
                            />
                            <div className="flex flex-col gap-1 shrink-0">
                              <Button size="sm"
                                disabled={!replyText.trim() || createReply.isPending}
                                onClick={() => createReply.mutate({ postId: post.id, content: replyText })}>
                                {createReply.isPending ? "…" : "Send"}
                              </Button>
                              <Button size="sm" variant="outline"
                                onClick={() => { setReplyPostId(null); setReplyText(""); }}>
                                Cancel
                              </Button>
                            </div>
                          </div>
                        )}
                        {replyPostId === post.id && createReply.isError && (
                          <p className="text-xs text-destructive">Failed: {(createReply.error as Error).message}</p>
                        )}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          <Dialog open={!!selectedPost} onOpenChange={(open) => { if (!open) { setSelectedPost(null); setReplyText(""); } }}>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>{selectedPost?.title ?? "Post details"}</DialogTitle>
                <DialogDescription>
                  Review the selected community post, see details, and send an admin reply.
                </DialogDescription>
              </DialogHeader>

              <div className="space-y-4 py-2">
                <div className="grid gap-3 sm:grid-cols-2">
                  <div className="rounded-xl border border-border bg-muted/20 p-4">
                    <p className="text-xs uppercase tracking-[0.2em] text-muted-foreground">Author</p>
                    <p className="mt-2 text-sm font-medium">{selectedPost?.anon_name}</p>
                    {selectedPost?.user_name && <p className="text-xs text-muted-foreground">{selectedPost.user_name}</p>}
                    {selectedPost?.user_email && <p className="text-xs text-muted-foreground">{selectedPost.user_email}</p>}
                  </div>
                  <div className="rounded-xl border border-border bg-muted/20 p-4">
                    <p className="text-xs uppercase tracking-[0.2em] text-muted-foreground">Meta</p>
                    <p className="mt-2 text-sm">{selectedPost?.category}</p>
                    <p className="text-xs text-muted-foreground">Replies: {selectedPost?.reply_count ?? 0}</p>
                    <p className="text-xs text-muted-foreground">Posted: {selectedPost ? new Date(selectedPost.created_at).toLocaleString() : ""}</p>
                    {selectedPost?.is_flagged && <p className="text-xs text-destructive mt-1">Flagged</p>}
                  </div>
                </div>

                <div className="rounded-xl border border-border bg-muted/20 p-4">
                  <p className="text-xs uppercase tracking-[0.2em] text-muted-foreground">Content</p>
                  <p className="mt-2 text-sm whitespace-pre-wrap">{selectedPost?.content}</p>
                </div>

                {selectedPost?.image_urls?.length ? (
                  <div className="space-y-2">
                    <p className="text-xs uppercase tracking-[0.2em] text-muted-foreground">Images</p>
                    <div className="grid grid-cols-3 gap-2">
                      {selectedPost.image_urls.map((url, idx) => (
                        <a key={idx} href={url} target="_blank" rel="noreferrer" className="block rounded-lg overflow-hidden border border-border">
                          <img src={url} alt={`post image ${idx + 1}`} className="h-24 w-full object-cover" />
                        </a>
                      ))}
                    </div>
                  </div>
                ) : null}

                <div className="space-y-2">
                  <p className="text-xs uppercase tracking-[0.2em] text-muted-foreground">
                    Replies ({postDetail?.replies?.length ?? selectedPost?.reply_count ?? 0})
                  </p>
                  {detailLoading ? (
                    <div className="space-y-2">
                      <Skeleton className="h-16 w-full rounded-lg" />
                      <Skeleton className="h-16 w-full rounded-lg" />
                    </div>
                  ) : postDetail?.replies?.length ? (
                    <div className="space-y-2 max-h-[280px] overflow-y-auto pr-1">
                      {postDetail.replies.map((reply) => (
                        <div
                          key={reply.id}
                          className={cn(
                            "rounded-lg border border-border bg-muted/20 p-3",
                            reply.is_flagged && "border-destructive/40 bg-destructive/5"
                          )}
                        >
                          <div className="flex items-start justify-between gap-2">
                            <div className="flex items-center gap-2">
                              <span className="text-xs font-medium text-primary">{reply.anon_name}</span>
                              {reply.is_flagged && (
                                <span className="text-[10px] bg-destructive/20 text-destructive px-1.5 py-0.5 rounded-full">Flagged</span>
                              )}
                              <span className="text-[11px] text-muted-foreground">
                                {new Date(reply.created_at).toLocaleString()}
                              </span>
                            </div>
                            <Button
                              size="sm"
                              variant="ghost"
                              className="h-6 w-6 p-0 text-muted-foreground hover:text-destructive"
                              title="Delete reply"
                              disabled={deleteReply.isPending}
                              onClick={() => { if (confirm("Delete this reply?")) deleteReply.mutate(reply.id); }}
                            >
                              <Trash2 className="h-3 w-3" />
                            </Button>
                          </div>
                          <p className="mt-1.5 text-sm whitespace-pre-wrap">{reply.content}</p>
                          {reply.image_url && (
                            <a href={reply.image_url} target="_blank" rel="noreferrer" className="mt-2 inline-block rounded-lg overflow-hidden border border-border">
                              <img src={reply.image_url} alt="reply image" className="h-24 w-auto object-cover" />
                            </a>
                          )}
                        </div>
                      ))}
                    </div>
                  ) : (
                    <p className="text-xs text-muted-foreground">No replies yet.</p>
                  )}
                </div>

                <div className="space-y-2">
                  <p className="text-xs uppercase tracking-[0.2em] text-muted-foreground">Admin reply</p>
                  <textarea
                    className="w-full rounded-md border border-border bg-background px-3 py-2 text-sm min-h-[120px] resize-y"
                    placeholder="Write your admin reply..."
                    value={replyText}
                    onChange={(e) => setReplyText(e.target.value)}
                  />
                  {createReply.isError && (
                    <p className="text-xs text-destructive">Failed: {(createReply.error as Error).message}</p>
                  )}
                </div>
              </div>

              <DialogFooter>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => setSelectedPost(null)}
                >
                  Close
                </Button>
                <Button
                  size="sm"
                  disabled={!replyText.trim() || createReply.isPending || !selectedPost}
                  onClick={() => selectedPost && createReply.mutate({ postId: selectedPost.id, content: replyText })}
                >
                  {createReply.isPending ? "Sending..." : "Send reply"}
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>

          {/* Pagination */}
          {(data?.total ?? 0) > PAGE_SIZE && (
            <div className="flex items-center justify-between mt-4">
              <p className="text-xs text-muted-foreground">
                Page {page} of {Math.ceil((data?.total ?? 0) / PAGE_SIZE)}
              </p>
              <div className="flex gap-2">
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => setPage((p) => Math.max(1, p - 1))}
                  disabled={page === 1}
                >
                  <ChevronLeft className="h-4 w-4" />
                  Previous
                </Button>
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => setPage((p) => p + 1)}
                  disabled={page >= Math.ceil((data?.total ?? 0) / PAGE_SIZE)}
                >
                  Next
                  <ChevronRight className="h-4 w-4" />
                </Button>
              </div>
            </div>
          )}
        </>
      )}
    </div>
  );
}
