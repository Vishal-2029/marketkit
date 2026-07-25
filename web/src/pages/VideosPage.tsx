import { useState, useRef, useEffect } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { PageHeader } from "@/components/PageHeader";
import { StatusBadge } from "@/components/StatusBadge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { Search, Plus, MoreHorizontal, Upload, CloudUpload, Trash2, Eye, Lock, Play, CheckCircle, MessageSquare, ThumbsUp, ThumbsDown, Pencil, Star } from "lucide-react";
import { toast } from "sonner";
import { videosService, EngagementStats, VideoComment } from "@/services/videos";
import { playlistsService } from "@/services/playlists";

interface Video {
  id: string;
  title: string;
  description?: string;
  category: "WILLCOM" | "E4" | "MECAD";
  duration_seconds?: number;
  status: "PUBLISHED" | "PROCESSING" | "DRAFT" | "ERROR";
  has_hls?: boolean;
  transcode_error?: string;
  transcode_started_at?: string;
  transcode_finished_at?: string;
  is_preview: boolean;
  is_free: boolean;
  is_intro: boolean;
  uploaded_at: string;
  thumbnail_url?: string;
  preview_url?: string;
}

const statusVariant = (s: string) => {
  if (s === "PUBLISHED") return "success" as const;
  if (s === "PROCESSING") return "warning" as const;
  if (s === "ERROR") return "danger" as const;
  return "neutral" as const;
};

const catVariant = (c: string) =>
  c === "WILLCOM" ? "gold" as const : c === "E4" ? "info" as const : "purple" as const;

const catLabel = (c: string) =>
  c === "WILLCOM" ? "Wilcom 2006" : c === "E4" ? "E4" : "meCAD";

function fmtDuration(s?: number) {
  if (!s) return "—";
  const m = Math.floor(s / 60);
  const sec = s % 60;
  return `${m}m ${String(sec).padStart(2, "0")}s`;
}

function fmtElapsed(ms: number) {
  const secs = Math.max(0, Math.floor(ms / 1000));
  const m = Math.floor(secs / 60);
  const s = secs % 60;
  return m > 0 ? `${m}m ${String(s).padStart(2, "0")}s` : `${s}s`;
}

function timeAgo(dateStr: string) {
  const diff = Date.now() - new Date(dateStr).getTime();
  const secs = Math.floor(diff / 1000);
  if (secs < 60) return "just now";
  if (secs < 3600) return `${Math.floor(secs / 60)}m ago`;
  if (secs < 86400) return `${Math.floor(secs / 3600)}h ago`;
  return `${Math.floor(secs / 86400)}d ago`;
}

/// Transcode column: live state while processing, error message + retry
/// button on failure, and how long the transcode took once complete — so the
/// client can always see what's happening with a video.
function TranscodeCell({ video: v, onRetry }: { video: Video; onRetry: () => void }) {
  if (v.status === "PROCESSING") {
    const elapsed = v.transcode_started_at && !v.transcode_finished_at
      ? fmtElapsed(Date.now() - new Date(v.transcode_started_at).getTime())
      : null;
    return (
      <div className="flex items-center gap-1.5 text-xs text-amber-600">
        <span className="h-2 w-2 rounded-full bg-amber-500 animate-pulse shrink-0" />
        <span>Transcoding…{elapsed ? ` ${elapsed}` : ""}</span>
      </div>
    );
  }
  if (v.status === "ERROR") {
    return (
      <div className="text-xs max-w-[200px]">
        <div className="flex items-center gap-2">
          <span className="font-medium text-danger">Failed</span>
          <button
            className="px-2 py-0.5 rounded-md border border-danger/40 text-danger font-medium hover:bg-danger/10 transition-colors"
            onClick={onRetry}
          >
            Retry Transcode
          </button>
        </div>
        {v.transcode_error && (
          <p className="text-muted-foreground truncate mt-0.5 cursor-help" title={v.transcode_error}>
            {v.transcode_error}
          </p>
        )}
      </div>
    );
  }
  if (v.transcode_started_at && v.transcode_finished_at) {
    const took =
      new Date(v.transcode_finished_at).getTime() -
      new Date(v.transcode_started_at).getTime();
    return (
      <div className="flex items-center gap-1.5 text-xs text-emerald-600">
        <CheckCircle className="h-3.5 w-3.5 shrink-0" />
        <span>Completed <span className="text-muted-foreground">in {fmtElapsed(took)}</span></span>
      </div>
    );
  }
  if (v.has_hls) {
    return (
      <div className="flex items-center gap-1.5 text-xs text-emerald-600">
        <CheckCircle className="h-3.5 w-3.5 shrink-0" />
        <span>Completed</span>
      </div>
    );
  }
  // Legacy: published before the transcode pipeline — offer to transcode now.
  return (
    <div className="flex items-center gap-2 text-xs">
      <span className="text-muted-foreground">Not transcoded</span>
      <button
        className="px-2 py-0.5 rounded-md border border-border text-foreground font-medium hover:bg-muted transition-colors"
        onClick={onRetry}
      >
        Transcode
      </button>
    </div>
  );
}

function uploadToR2(url: string, file: File, onProgress: (ratio: number) => void): Promise<void> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.upload.onprogress = (e) => { if (e.lengthComputable) onProgress(e.loaded / e.total); };
    xhr.onload = () => (xhr.status >= 200 && xhr.status < 300) ? resolve() : reject(new Error(`R2 upload failed: ${xhr.status}`));
    xhr.onerror = () => reject(new Error("Upload network error"));
    xhr.open("PUT", url);
    xhr.setRequestHeader("Content-Type", file.type);
    xhr.send(file);
  });
}

export default function VideosPage() {
  const qc = useQueryClient();
  const [search, setSearch] = useState("");
  const [catFilter, setCatFilter] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [isFreeFilter, setIsFreeFilter] = useState("");
  const [page, setPage] = useState(1);
  const [showUpload, setShowUpload] = useState(false);
  const [actionMenu, setActionMenu] = useState<string | null>(null);
  const [playVideo, setPlayVideo] = useState<Video | null>(null);
  const [videoError, setVideoError] = useState(false);
  const [streamSrc, setStreamSrc] = useState<string | null>(null);

  const [uploadTitle, setUploadTitle] = useState("");
  const [uploadDesc, setUploadDesc] = useState("");
  const [uploadCat, setUploadCat] = useState("WILLCOM");
  const [uploadIsFree, setUploadIsFree] = useState(true);
  const [uploadPlaylistId, setUploadPlaylistId] = useState("");
  const [newPlaylistName, setNewPlaylistName] = useState("");
  const [uploadFile, setUploadFile] = useState<File | null>(null);
  const [uploadThumb, setUploadThumb] = useState<File | null>(null);
  const [uploadThumbPreview, setUploadThumbPreview] = useState<string | null>(null);
  const [uploadPhotos, setUploadPhotos] = useState<File[]>([]);
  const [uploadPhotosPreviews, setUploadPhotosPreviews] = useState<string[]>([]);
  const [uploadProgress, setUploadProgress] = useState(0);
  const [isUploading, setIsUploading] = useState(false);
  const [isAttachingPhotos, setIsAttachingPhotos] = useState(false);
  const fileRef = useRef<HTMLInputElement>(null);
  const thumbRef = useRef<HTMLInputElement>(null);
  const photosRef = useRef<HTMLInputElement>(null);

  const [deleteTarget, setDeleteTarget] = useState<Video | null>(null);

  // Edit modal state
  const [editVideo, setEditVideo] = useState<Video | null>(null);
  const [editTitle, setEditTitle] = useState("");
  const [editDesc, setEditDesc] = useState("");
  const [editCategory, setEditCategory] = useState("WILLCOM");
  const [editThumb, setEditThumb] = useState<File | null>(null);
  const [editThumbPreview, setEditThumbPreview] = useState<string | null>(null);
  const [editPlaylistId, setEditPlaylistId] = useState("");
  const editThumbRef = useRef<HTMLInputElement>(null);

  // Engagement modal state
  const [engVideo, setEngVideo] = useState<Video | null>(null);
  const [engagement, setEngagement] = useState<EngagementStats | null>(null);
  const [engComments, setEngComments] = useState<VideoComment[]>([]);
  const [engPage, setEngPage] = useState(1);
  const [engLoading, setEngLoading] = useState(false);
  const [engHasMore, setEngHasMore] = useState(false);
  const [replyTargetId, setReplyTargetId] = useState<string | null>(null);
  const [replyText, setReplyText] = useState("");
  const [replySending, setReplySending] = useState(false);

  const prevStatusRef = useRef<Record<string, string>>({});

  const { data: playlistsData = [] } = useQuery({
    queryKey: ["admin-playlists"],
    queryFn: () => playlistsService.list(),
  });

  const createPlaylistMut = useMutation({
    // A quick-created playlist inherits the upload's category — playlists
    // only accept videos of their own category.
    mutationFn: (name: string) => playlistsService.create({ name, description: "", thumbnail_url: "", category: uploadCat }),
    onSuccess: (newPl) => {
      qc.invalidateQueries({ queryKey: ["admin-playlists"] });
      setUploadPlaylistId(newPl.id);
      setNewPlaylistName("");
      toast.success(`Playlist "${newPl.name}" created.`);
    },
    onError: () => toast.error("Failed to create playlist."),
  });

  const handleAddPlaylist = () => {
    const name = newPlaylistName.trim();
    if (!name) return;
    createPlaylistMut.mutate(name);
  };

  const { data, isLoading } = useQuery({
    queryKey: ["videos", page, search, catFilter, statusFilter, isFreeFilter],
    queryFn: () => videosService.list({
      page, limit: 10,
      search: search || undefined,
      category: catFilter || undefined,
      status: statusFilter || undefined,
      is_free: isFreeFilter !== "" ? isFreeFilter === "true" : undefined,
    }),
    refetchInterval: (query) => {
      const vids: Video[] = (query.state.data as { data?: Video[] } | undefined)?.data ?? [];
      return vids.some(v => v.status === "PROCESSING") ? 10_000 : false;
    },
  });

  const videos: Video[] = data?.data ?? [];
  const meta = data?.meta;

  useEffect(() => {
    const prev = prevStatusRef.current;
    videos.forEach(v => {
      if (prev[v.id] === "PROCESSING") {
        if (v.status === "PUBLISHED") toast.success(`"${v.title}" finished processing and is now published.`);
        else if (v.status === "ERROR") toast.error(`"${v.title}" failed to process.`);
      }
    });
    prevStatusRef.current = Object.fromEntries(videos.map(v => [v.id, v.status]));
  }, [videos]);

  useEffect(() => {
    if (!playVideo) { setStreamSrc(null); return; }
    setStreamSrc(null);
    setVideoError(false);
    let cancelled = false;
    videosService.fetchStreamUrl(playVideo.id)
      .then(url => { if (!cancelled) setStreamSrc(url); })
      .catch(() => { if (!cancelled) setVideoError(true); });
    return () => { cancelled = true; };
  }, [playVideo?.id]);

  const deleteMut = useMutation({
    mutationFn: (id: string) => videosService.delete(id),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ["videos"] }); toast.success("Video deleted."); },
    onError: () => toast.error("Failed to delete video."),
  });

  const retryMut = useMutation({
    mutationFn: (id: string) => videosService.retry(id),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ["videos"] }); toast.success("Video requeued for transcoding."); },
    onError: (err: { response?: { data?: { message?: string } } }) =>
      toast.error(err?.response?.data?.message ?? "Failed to retry."),
  });

  const togglePreviewMut = useMutation({
    mutationFn: ({ id, is_preview }: { id: string; is_preview: boolean }) =>
      videosService.update(id, { is_preview }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ["videos"] }); },
    onError: () => toast.error("Failed to update preview."),
  });

  const toggleFreeMut = useMutation({
    mutationFn: ({ id, is_free }: { id: string; is_free: boolean }) =>
      videosService.update(id, { is_free }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ["videos"] }); },
    onError: () => toast.error("Failed to update access."),
  });

  const toggleIntroMut = useMutation({
    mutationFn: ({ id, is_intro }: { id: string; is_intro: boolean }) =>
      videosService.update(id, { is_intro }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ["videos"] }); },
    onError: () => toast.error("Failed to update intro status."),
  });

  const togglePublishMut = useMutation({
    mutationFn: ({ id, status }: { id: string; status: string }) =>
      videosService.update(id, { status }),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ["videos"] }); },
    // Surface the API's reason (e.g. "video is still transcoding — it can
    // be published once transcoding completes") instead of a generic error.
    onError: (err: { response?: { data?: { message?: string } } }) =>
      toast.error(err?.response?.data?.message ?? "Failed to update status."),
  });

  const createMut = useMutation({
    mutationFn: (data: object) => videosService.create(data),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ["videos"] }); },
    onError: () => {},
  });

  const editMut = useMutation({
    mutationFn: ({ id, fd }: { id: string; fd: FormData }) => videosService.edit(id, fd),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["videos"] });
      setEditVideo(null);
      toast.success("Video updated.");
    },
    onError: () => toast.error("Failed to update video."),
  });

  const handleOpenEdit = (v: Video) => {
    setEditVideo(v);
    setEditTitle(v.title);
    setEditDesc(v.description ?? "");
    setEditCategory(v.category);
    setEditThumb(null);
    setEditThumbPreview(v.thumbnail_url ?? null);
    setEditPlaylistId("");
    setActionMenu(null);
  };

  const handleEditSave = () => {
    if (!editVideo || !editTitle.trim()) { toast.error("Title is required."); return; }
    const fd = new FormData();
    fd.append("title", editTitle.trim());
    fd.append("description", editDesc);
    fd.append("category", editCategory);
    if (editThumb) fd.append("thumbnail", editThumb);
    if (editPlaylistId) fd.append("playlist_id", editPlaylistId);
    else fd.append("clear_playlist", "1");
    editMut.mutate({ id: editVideo.id, fd });
  };

  const handleEditThumbChange = (file: File | null) => {
    setEditThumb(file);
    setEditThumbPreview(file ? URL.createObjectURL(file) : (editVideo?.thumbnail_url ?? null));
  };

  const handleUpload = async () => {
    if (!uploadTitle.trim()) { toast.error("Please enter a video title."); return; }
    if (!uploadFile) { toast.error("Please select a video file."); return; }

    setIsUploading(true);
    setUploadProgress(0);
    let video: Video | null = null;
    try {
      const presign = await videosService.presignUpload(uploadFile.name, uploadFile.type);

      if (presign.direct && presign.upload_url && presign.file_key) {
        // Direct upload: browser → R2 (0-90%), then metadata → server (90-100%)
        await uploadToR2(presign.upload_url, uploadFile, (ratio) =>
          setUploadProgress(Math.round(ratio * 90))
        );
        setUploadProgress(90);
        const fd = new FormData();
        fd.append("file_key", presign.file_key);
        fd.append("title", uploadTitle);
        fd.append("description", uploadDesc);
        fd.append("category", uploadCat);
        fd.append("is_free", String(uploadIsFree));
        if (uploadPlaylistId) fd.append("playlist_id", uploadPlaylistId);
        if (uploadThumb) fd.append("thumbnail", uploadThumb);
        video = await videosService.finalize(fd, (pct) => setUploadProgress(90 + Math.round(pct * 0.1)));
      } else {
        // Fallback: local storage — regular multipart through server
        const fd = new FormData();
        fd.append("title", uploadTitle);
        fd.append("description", uploadDesc);
        fd.append("category", uploadCat);
        fd.append("is_free", String(uploadIsFree));
        if (uploadPlaylistId) fd.append("playlist_id", uploadPlaylistId);
        fd.append("file", uploadFile);
        if (uploadThumb) fd.append("thumbnail", uploadThumb);
        video = await videosService.create(fd, setUploadProgress);
      }

      qc.invalidateQueries({ queryKey: ["videos"] });
      toast.success("Video uploaded and is processing.");

      if (video && uploadPhotos.length > 0) {
        setIsAttachingPhotos(true);
        try {
          const photosFd = new FormData();
          photosFd.append("title", uploadTitle);
          uploadPhotos.forEach(f => photosFd.append("photos", f));
          await videosService.uploadPhotos(video.id, photosFd);
          qc.invalidateQueries({ queryKey: ["photos"] });
        } catch {
          toast.error("Video uploaded, but attaching photos failed — add them from the Photos page.");
        } finally {
          setIsAttachingPhotos(false);
        }
      }

      uploadPhotosPreviews.forEach(url => URL.revokeObjectURL(url));
      setShowUpload(false);
      setUploadTitle(""); setUploadDesc(""); setUploadCat("WILLCOM");
      setUploadIsFree(true); setUploadFile(null);
      setUploadThumb(null); setUploadThumbPreview(null);
      setUploadPhotos([]); setUploadPhotosPreviews([]);
    } catch {
      toast.error("Upload failed. Please try again.");
    } finally {
      setIsUploading(false);
      setUploadProgress(0);
    }
  };

  const handlePlay = (v: Video) => {
    setPlayVideo(v);
    setVideoError(false);
    setActionMenu(null);
  };

  const handleTogglePublish = (v: Video) => {
    const next = v.status === "PUBLISHED" ? "DRAFT" : "PUBLISHED";
    togglePublishMut.mutate({ id: v.id, status: next });
    toast.success(`"${v.title}" ${next === "PUBLISHED" ? "published" : "unpublished"}.`);
    setActionMenu(null);
  };

  const handleTogglePreview = (v: Video) => {
    togglePreviewMut.mutate({ id: v.id, is_preview: !v.is_preview });
    toast.success(`Preview ${v.is_preview ? "disabled" : "enabled"} for "${v.title}".`);
    setActionMenu(null);
  };

  const handleToggleFree = (v: Video) => {
    toggleFreeMut.mutate({ id: v.id, is_free: !v.is_free });
    toast.success(`"${v.title}" changed to ${v.is_free ? "Paid" : "Free"}.`);
    setActionMenu(null);
  };

  const handleToggleIntro = (v: Video) => {
    if (v.is_intro) {
      toggleIntroMut.mutate({ id: v.id, is_intro: false });
      toast.success(`Intro unset for "${v.title}".`);
    } else {
      toggleIntroMut.mutate({ id: v.id, is_intro: true });
      toast.success(`"${v.title}" set as intro video.`);
    }
    setActionMenu(null);
  };

  const handleDelete = (v: Video) => {
    setDeleteTarget(v);
    setActionMenu(null);
  };

  const closeUpload = () => {
    if (isUploading) return;
    setShowUpload(false);
    setUploadTitle(""); setUploadDesc(""); setUploadCat("WILLCOM");
    setUploadIsFree(true); setUploadPlaylistId(""); setNewPlaylistName(""); setUploadFile(null);
    setUploadThumb(null); setUploadThumbPreview(null);
    uploadPhotosPreviews.forEach(url => URL.revokeObjectURL(url));
    setUploadPhotos([]); setUploadPhotosPreviews([]);
  };

  const handleThumbChange = (file: File | null) => {
    setUploadThumb(file);
    if (file) {
      const url = URL.createObjectURL(file);
      setUploadThumbPreview(url);
    } else {
      setUploadThumbPreview(null);
    }
  };

  const handleAddPhotos = (files: FileList | null) => {
    if (!files || files.length === 0) return;
    const newFiles = Array.from(files);
    setUploadPhotos(prev => [...prev, ...newFiles]);
    setUploadPhotosPreviews(prev => [...prev, ...newFiles.map(f => URL.createObjectURL(f))]);
  };

  const removeUploadPhoto = (index: number) => {
    setUploadPhotosPreviews(prev => {
      URL.revokeObjectURL(prev[index]);
      return prev.filter((_, i) => i !== index);
    });
    setUploadPhotos(prev => prev.filter((_, i) => i !== index));
  };

  const openEngagement = async (v: Video) => {
    setEngVideo(v);
    setEngagement(null);
    setEngComments([]);
    setEngPage(1);
    setEngHasMore(false);
    setReplyTargetId(null);
    setReplyText("");
    setEngLoading(true);
    try {
      const [stats, comments] = await Promise.all([
        videosService.getEngagement(v.id),
        videosService.getComments(v.id, 1),
      ]);
      setEngagement(stats);
      setEngComments(comments);
      setEngHasMore(comments.length === 20);
    } catch {
      toast.error("Failed to load engagement data.");
    } finally {
      setEngLoading(false);
    }
  };

  const loadMoreEngComments = async () => {
    if (!engVideo || engLoading) return;
    const nextPage = engPage + 1;
    setEngLoading(true);
    try {
      const more = await videosService.getComments(engVideo.id, nextPage);
      setEngComments(prev => [...prev, ...more]);
      setEngPage(nextPage);
      setEngHasMore(more.length === 20);
    } catch {
      toast.error("Failed to load more comments.");
    } finally {
      setEngLoading(false);
    }
  };

  const handleDeleteEngComment = async (commentId: string) => {
    if (!engVideo) return;
    try {
      await videosService.deleteComment(engVideo.id, commentId);
      setEngComments(prev => prev.filter(c => c.id !== commentId));
      if (engagement) setEngagement({ ...engagement, comment_count: engagement.comment_count - 1 });
    } catch {
      toast.error("Failed to delete comment.");
    }
  };

  const handleSendReply = async (c: VideoComment) => {
    if (!engVideo || !replyText.trim() || replySending) return;
    setReplySending(true);
    try {
      const reply = await videosService.replyToComment(engVideo.id, c.thread_user_id, replyText.trim());
      setEngComments(prev => [reply, ...prev]);
      if (engagement) setEngagement({ ...engagement, comment_count: engagement.comment_count + 1 });
      setReplyText("");
      setReplyTargetId(null);
    } catch {
      toast.error("Failed to send reply.");
    } finally {
      setReplySending(false);
    }
  };

  return (
    <div onClick={() => setActionMenu(null)}>
      <PageHeader title="Videos" subtitle="Upload and manage training videos">
        <Button onClick={() => setShowUpload(true)}><Plus className="h-4 w-4" /> Upload Video</Button>
      </PageHeader>

      <div className="flex items-center gap-3 mb-6 flex-wrap">
        <div className="relative flex-1 max-w-sm">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input
            placeholder="Search videos..."
            value={search}
            onChange={e => { setSearch(e.target.value); setPage(1); }}
            className="pl-9 bg-muted border-0"
          />
        </div>
        <select
          className="h-10 rounded-lg border border-border bg-card px-3 text-sm"
          value={catFilter}
          onChange={e => { setCatFilter(e.target.value); setPage(1); }}
        >
          <option value="">All Categories</option>
          <option value="WILLCOM">Wilcom 2006</option>
          <option value="E4">E4</option>
          <option value="MECAD">meCAD</option>
        </select>
        <select
          className="h-10 rounded-lg border border-border bg-card px-3 text-sm"
          value={statusFilter}
          onChange={e => { setStatusFilter(e.target.value); setPage(1); }}
        >
          <option value="">All Status</option>
          <option value="PUBLISHED">Published</option>
          <option value="DRAFT">Draft</option>
          <option value="PROCESSING">Processing</option>
          <option value="ERROR">Error</option>
        </select>
        <select
          className="h-10 rounded-lg border border-border bg-card px-3 text-sm"
          value={isFreeFilter}
          onChange={e => { setIsFreeFilter(e.target.value); setPage(1); }}
        >
          <option value="">All Access</option>
          <option value="true">Free</option>
          <option value="false">Paid</option>
        </select>
      </div>

      <div className="rounded-xl border border-border bg-card">
        <table className="w-full">
          <thead>
            <tr className="bg-table-header">
              <th className="text-table-header text-left px-4 py-3">Video</th>
              <th className="text-table-header text-left px-4 py-3 hidden md:table-cell">Category</th>
              <th className="text-table-header text-left px-4 py-3 hidden md:table-cell">Duration</th>
              <th className="text-table-header text-left px-4 py-3">Status</th>
              <th className="text-table-header text-left px-4 py-3 hidden md:table-cell">Transcode</th>
              <th className="text-table-header text-left px-4 py-3 hidden md:table-cell">Preview</th>
              <th className="text-table-header text-left px-4 py-3 hidden md:table-cell">Access</th>
              <th className="text-table-header text-left px-4 py-3 hidden lg:table-cell">Uploaded</th>
              <th className="text-table-header text-left px-4 py-3">Actions</th>
            </tr>
          </thead>
          <tbody>
            {isLoading
              ? Array(5).fill(0).map((_, i) => (
                <tr key={i}><td colSpan={9} className="px-4 py-3"><Skeleton className="h-8" /></td></tr>
              ))
              : videos.map(v => (
                <tr key={v.id} className="border-b border-border last:border-0 hover:bg-table-hover transition-colors">
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-3">
                      <div className="w-16 h-10 rounded-lg bg-muted flex items-center justify-center shrink-0 overflow-hidden">
                        {v.thumbnail_url
                          ? <img src={v.thumbnail_url} alt={v.title} className="w-full h-full object-cover" />
                          : v.preview_url
                          ? <video src={v.preview_url} className="w-full h-full object-cover" muted playsInline preload="metadata" />
                          : <Upload className="h-4 w-4 text-muted-foreground" />
                        }
                      </div>
                      <div>
                        <p className="text-sm font-medium text-foreground">{v.title}</p>
                        <p className="text-caption truncate max-w-[200px]">{v.description ?? ""}</p>
                      </div>
                    </div>
                  </td>
                  <td className="px-4 py-3 hidden md:table-cell"><StatusBadge variant={catVariant(v.category)}>{catLabel(v.category)}</StatusBadge></td>
                  <td className="px-4 py-3 text-sm text-muted-foreground font-mono hidden md:table-cell">{fmtDuration(v.duration_seconds)}</td>
                  <td className="px-4 py-3">
                    {/* Retry + error details live in the Transcode column. */}
                    <StatusBadge variant={statusVariant(v.status)}>{v.status.charAt(0) + v.status.slice(1).toLowerCase()}</StatusBadge>
                  </td>
                  <td className="px-4 py-3 hidden md:table-cell">
                    <TranscodeCell video={v} onRetry={() => retryMut.mutate(v.id)} />
                  </td>
                  <td className="px-4 py-3 hidden md:table-cell">
                    <button
                      className={`w-9 h-5 rounded-full flex items-center px-0.5 transition-colors ${v.is_preview ? "bg-primary" : "bg-muted"}`}
                      onClick={() => handleTogglePreview(v)}
                      title={v.is_preview ? "Disable preview" : "Enable preview"}
                    >
                      <div className={`w-4 h-4 rounded-full bg-card transition-transform ${v.is_preview ? "translate-x-4" : ""}`} />
                    </button>
                  </td>
                  <td className="px-4 py-3 hidden md:table-cell">
                    <StatusBadge variant={v.is_free ? "success" : "info"}>
                      {v.is_free ? "Free" : "Paid"}
                    </StatusBadge>
                  </td>
                  <td className="px-4 py-3 text-sm text-muted-foreground hidden lg:table-cell">
                    {new Date(v.uploaded_at).toLocaleDateString("en-IN", { day: "2-digit", month: "short", year: "numeric" })}
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-0.5">
                      <Button
                        variant="ghost"
                        size="icon"
                        title="View engagement"
                        onClick={e => { e.stopPropagation(); openEngagement(v); setActionMenu(null); }}
                      >
                        <MessageSquare className="h-4 w-4" />
                      </Button>
                    <div className="relative" onClick={e => e.stopPropagation()}>
                      <Button
                        variant="ghost"
                        size="icon"
                        onClick={() => setActionMenu(actionMenu === v.id ? null : v.id)}
                      >
                        <MoreHorizontal className="h-4 w-4" />
                      </Button>
                      {actionMenu === v.id && (
                        <div className="absolute right-0 top-full mt-1 w-48 bg-card border border-border rounded-lg shadow-lg z-50 py-1">
                          <button
                            className="flex items-center gap-2 w-full px-3 py-2 text-sm text-foreground hover:bg-muted"
                            onClick={() => handleOpenEdit(v)}
                          >
                            <Pencil className="h-3.5 w-3.5" /> Edit Video
                          </button>
                          <button
                            className="flex items-center gap-2 w-full px-3 py-2 text-sm text-foreground hover:bg-muted"
                            onClick={() => handlePlay(v)}
                          >
                            <Play className="h-3.5 w-3.5" /> Play Video
                          </button>
                          <button
                            className="flex items-center gap-2 w-full px-3 py-2 text-sm text-foreground hover:bg-muted"
                            onClick={() => handleTogglePreview(v)}
                          >
                            <Eye className="h-3.5 w-3.5" /> Toggle Preview
                          </button>
                          <button
                            className="flex items-center gap-2 w-full px-3 py-2 text-sm text-foreground hover:bg-muted"
                            onClick={() => handleToggleFree(v)}
                          >
                            <Lock className="h-3.5 w-3.5" />
                            {v.is_free ? "Mark as Paid" : "Mark as Free"}
                          </button>
                          <button
                            className={`flex items-center gap-2 w-full px-3 py-2 text-sm hover:bg-muted ${v.is_intro ? "text-amber-400" : "text-foreground"}`}
                            onClick={() => handleToggleIntro(v)}
                          >
                            <Star className={`h-3.5 w-3.5 ${v.is_intro ? "fill-amber-400" : ""}`} />
                            {v.is_intro ? "Unset Intro Video" : "Set as Intro Video"}
                          </button>
                          <button
                            className="flex items-center gap-2 w-full px-3 py-2 text-sm text-foreground hover:bg-muted"
                            onClick={() => handleTogglePublish(v)}
                          >
                            <CheckCircle className="h-3.5 w-3.5" />
                            {v.status === "PUBLISHED" ? "Unpublish" : "Publish"}
                          </button>
                          <button
                            className="flex items-center gap-2 w-full px-3 py-2 text-sm text-danger hover:bg-muted"
                            onClick={() => handleDelete(v)}
                          >
                            <Trash2 className="h-3.5 w-3.5" /> Delete
                          </button>
                        </div>
                      )}
                    </div>
                    </div>
                  </td>
                </tr>
              ))
            }
            {!isLoading && videos.length === 0 && (
              <tr><td colSpan={9} className="px-4 py-8 text-center text-sm text-muted-foreground">No videos found.</td></tr>
            )}
          </tbody>
        </table>
        {meta && meta.pages > 1 && (
          <div className="bg-table-header px-4 py-3 flex items-center justify-between">
            <span className="text-caption">Page {page} of {meta.pages} · {meta.total} total</span>
            <div className="flex gap-1">
              <Button variant="outline" size="sm" disabled={page === 1} onClick={() => setPage(p => p - 1)}>Previous</Button>
              <Button variant="outline" size="sm" disabled={page === meta.pages} onClick={() => setPage(p => p + 1)}>Next</Button>
            </div>
          </div>
        )}
      </div>

      {/* Video Player Modal */}
      {playVideo && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-foreground/60"
          onClick={() => setPlayVideo(null)}
        >
          <div
            className="bg-card rounded-2xl shadow-xl w-full max-w-3xl p-6"
            onClick={e => e.stopPropagation()}
          >
            <div className="flex items-center justify-between mb-4">
              <div>
                <h2 className="text-section-title">{playVideo.title}</h2>
                {playVideo.description && (
                  <p className="text-caption mt-0.5">{playVideo.description}</p>
                )}
              </div>
              <Button variant="ghost" size="icon" onClick={() => setPlayVideo(null)}>
                <Plus className="h-4 w-4 rotate-45" />
              </Button>
            </div>
            {videoError ? (
              <div className="w-full rounded-xl bg-muted flex flex-col items-center justify-center py-16 text-center">
                <p className="text-sm font-medium text-foreground">Unable to play this video</p>
                <p className="text-caption mt-1">The video file may not be available on the server yet.</p>
              </div>
            ) : !streamSrc ? (
              <div className="w-full rounded-xl bg-muted flex items-center justify-center py-16">
                <p className="text-sm text-muted-foreground">Loading video…</p>
              </div>
            ) : (
              <video
                key={playVideo.id}
                src={streamSrc}
                controls
                autoPlay
                className="w-full rounded-xl bg-black max-h-[60vh]"
                onError={() => setVideoError(true)}
              >
                Your browser does not support the video tag.
              </video>
            )}
            <div className="flex items-center gap-3 mt-4">
              <StatusBadge variant={catVariant(playVideo.category)}>{catLabel(playVideo.category)}</StatusBadge>
              <StatusBadge variant={statusVariant(playVideo.status)}>
                {playVideo.status.charAt(0) + playVideo.status.slice(1).toLowerCase()}
              </StatusBadge>
              <StatusBadge variant={playVideo.is_free ? "success" : "info"}>
                {playVideo.is_free ? "Free" : "Paid"}
              </StatusBadge>
              {playVideo.duration_seconds != null && playVideo.duration_seconds > 0 && (
                <span className="text-sm text-muted-foreground font-mono ml-auto">
                  {fmtDuration(playVideo.duration_seconds)}
                </span>
              )}
            </div>
          </div>
        </div>
      )}

      {/* Engagement Modal */}
      {engVideo && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-foreground/60"
          onClick={() => setEngVideo(null)}
        >
          <div
            className="bg-card rounded-2xl shadow-xl w-full max-w-lg mx-4 flex flex-col max-h-[85vh]"
            onClick={e => e.stopPropagation()}
          >
            {/* Header */}
            <div className="flex items-center justify-between px-6 py-4 border-b border-border shrink-0">
              <div>
                <h2 className="text-section-title">Engagement</h2>
                <p className="text-caption mt-0.5 truncate max-w-[320px]">{engVideo.title}</p>
              </div>
              <Button variant="ghost" size="icon" onClick={() => setEngVideo(null)}>
                <Plus className="h-4 w-4 rotate-45" />
              </Button>
            </div>

            {engLoading && !engagement ? (
              <div className="flex items-center justify-center py-16">
                <div className="h-6 w-6 rounded-full border-2 border-primary border-t-transparent animate-spin" />
              </div>
            ) : (
              <>
                {/* Reaction counts */}
                {engagement && (
                  <div className="grid grid-cols-3 gap-3 px-6 py-4 border-b border-border shrink-0">
                    <div className="flex flex-col items-center gap-1 bg-muted rounded-xl py-3">
                      <ThumbsUp className="h-5 w-5 text-primary" />
                      <span className="text-xl font-bold text-foreground">{engagement.like_count}</span>
                      <span className="text-xs text-muted-foreground">Likes</span>
                    </div>
                    <div className="flex flex-col items-center gap-1 bg-muted rounded-xl py-3">
                      <ThumbsDown className="h-5 w-5 text-muted-foreground" />
                      <span className="text-xl font-bold text-foreground">{engagement.dislike_count}</span>
                      <span className="text-xs text-muted-foreground">Dislikes</span>
                    </div>
                    <div className="flex flex-col items-center gap-1 bg-muted rounded-xl py-3">
                      <MessageSquare className="h-5 w-5 text-muted-foreground" />
                      <span className="text-xl font-bold text-foreground">{engagement.comment_count}</span>
                      <span className="text-xs text-muted-foreground">Comments</span>
                    </div>
                  </div>
                )}

                {/* Comments list */}
                <div className="overflow-y-auto flex-1 px-6 py-4">
                  <p className="text-sm font-semibold text-foreground mb-3">
                    Private messages ({engagement?.comment_count ?? 0})
                  </p>
                  <p className="text-xs text-muted-foreground -mt-2 mb-3">
                    Each user only sees their own messages and your replies to them.
                  </p>
                  {engComments.length === 0 ? (
                    <p className="text-sm text-muted-foreground text-center py-8">No messages yet.</p>
                  ) : (
                    <div className="space-y-3">
                      {engComments.map(c => (
                        <div key={c.id} className={c.is_admin ? "rounded-lg bg-primary/5 -mx-2 px-2 py-2" : ""}>
                          <div className="flex items-start gap-3">
                            {c.is_admin ? (
                              <div className="w-8 h-8 rounded-full bg-primary flex items-center justify-center shrink-0">
                                <MessageSquare className="h-4 w-4 text-primary-foreground" />
                              </div>
                            ) : (
                              <div className="w-8 h-8 rounded-full bg-primary/15 flex items-center justify-center shrink-0 text-sm font-bold text-primary">
                                {c.user_name.charAt(0).toUpperCase()}
                              </div>
                            )}
                            <div className="flex-1 min-w-0">
                              <div className="flex items-center gap-2">
                                <span className="text-sm font-medium text-foreground">
                                  {c.is_admin ? `${c.user_name} (Admin)` : c.user_name}
                                </span>
                                <span className="text-xs text-muted-foreground">{timeAgo(c.created_at)}</span>
                              </div>
                              <p className="text-sm text-muted-foreground mt-0.5 break-words">{c.content}</p>
                              {!c.is_admin && (
                                <button
                                  className="text-xs text-primary hover:underline mt-1"
                                  onClick={() => {
                                    setReplyTargetId(replyTargetId === c.id ? null : c.id);
                                    setReplyText("");
                                  }}
                                >
                                  Reply
                                </button>
                              )}
                              {replyTargetId === c.id && (
                                <div className="mt-2 flex items-center gap-2">
                                  <input
                                    className="flex-1 h-8 rounded-lg border border-border bg-muted px-2 text-sm"
                                    placeholder={`Reply to ${c.user_name}…`}
                                    value={replyText}
                                    onChange={e => setReplyText(e.target.value)}
                                    onKeyDown={e => { if (e.key === "Enter") { e.preventDefault(); handleSendReply(c); } }}
                                    autoFocus
                                  />
                                  <Button
                                    size="sm"
                                    disabled={replySending || !replyText.trim()}
                                    onClick={() => handleSendReply(c)}
                                  >
                                    {replySending ? "Sending…" : "Send"}
                                  </Button>
                                </div>
                              )}
                            </div>
                            <button
                              className="shrink-0 text-muted-foreground hover:text-danger transition-colors"
                              title="Delete comment"
                              onClick={() => handleDeleteEngComment(c.id)}
                            >
                              <Trash2 className="h-4 w-4" />
                            </button>
                          </div>
                        </div>
                      ))}
                    </div>
                  )}

                  {engHasMore && (
                    <button
                      className="mt-4 w-full text-sm text-primary hover:underline disabled:opacity-50"
                      disabled={engLoading}
                      onClick={loadMoreEngComments}
                    >
                      {engLoading ? "Loading…" : "Load more"}
                    </button>
                  )}
                </div>
              </>
            )}
          </div>
        </div>
      )}

      {/* Edit Video Modal */}
      {editVideo && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-foreground/40"
          onClick={() => setEditVideo(null)}
        >
          <div
            className="bg-card rounded-2xl shadow-xl w-full max-w-lg p-6"
            onClick={e => e.stopPropagation()}
          >
            <div className="flex items-center justify-between mb-6">
              <h2 className="text-section-title">Edit Video</h2>
              <Button variant="ghost" size="icon" onClick={() => setEditVideo(null)}>
                <Plus className="h-4 w-4 rotate-45" />
              </Button>
            </div>

            {/* Thumbnail picker */}
            <div className="mb-5">
              <label className="text-sm font-medium text-foreground">Thumbnail</label>
              <div
                className="mt-1 border border-dashed border-border rounded-xl p-4 flex items-center gap-4 cursor-pointer hover:border-primary transition-colors"
                onClick={() => editThumbRef.current?.click()}
              >
                {editThumbPreview
                  ? <img src={editThumbPreview} alt="Thumbnail" className="w-24 h-14 object-cover rounded-lg shrink-0" />
                  : <div className="w-24 h-14 rounded-lg bg-muted flex items-center justify-center shrink-0">
                      <Upload className="h-4 w-4 text-muted-foreground" />
                    </div>
                }
                <div className="min-w-0">
                  <p className="text-sm text-foreground font-medium truncate">
                    {editThumb ? editThumb.name : "Click to replace thumbnail"}
                  </p>
                  <p className="text-xs text-muted-foreground mt-0.5">JPG, PNG or WebP · 16:9 recommended</p>
                </div>
                {editThumb && (
                  <button
                    className="ml-auto text-xs text-danger hover:underline shrink-0"
                    onClick={e => { e.stopPropagation(); handleEditThumbChange(null); }}
                  >
                    Remove
                  </button>
                )}
              </div>
              <input
                ref={editThumbRef}
                type="file"
                accept="image/jpeg,image/png,image/webp"
                className="hidden"
                onChange={e => handleEditThumbChange(e.target.files?.[0] ?? null)}
              />
            </div>

            <div className="space-y-4">
              <div>
                <label className="text-sm font-medium text-foreground">Title *</label>
                <input
                  className="mt-1 w-full rounded-lg border border-border bg-card px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
                  placeholder="Enter video title"
                  value={editTitle}
                  onChange={e => setEditTitle(e.target.value)}
                />
              </div>
              <div>
                <label className="text-sm font-medium text-foreground">Description</label>
                <textarea
                  className="mt-1 w-full rounded-lg border border-border bg-card px-3 py-2 text-sm min-h-[80px] focus:outline-none focus:ring-2 focus:ring-ring"
                  placeholder="Video description..."
                  value={editDesc}
                  onChange={e => setEditDesc(e.target.value)}
                />
              </div>
              <div>
                <label className="text-sm font-medium text-foreground">Category</label>
                <select
                  className="mt-1 h-10 w-full rounded-lg border border-border bg-card px-3 text-sm"
                  value={editCategory}
                  onChange={e => {
                    setEditCategory(e.target.value);
                    // A playlist of another category is no longer valid.
                    setEditPlaylistId("");
                  }}
                >
                  <option value="WILLCOM">Wilcom 2006</option>
                  <option value="E4">E4</option>
                  <option value="MECAD">meCAD</option>
                </select>
              </div>
              <div>
                <label className="text-sm font-medium text-foreground">Playlist</label>
                <select
                  className="mt-1 h-10 w-full rounded-lg border border-border bg-card px-3 text-sm"
                  value={editPlaylistId}
                  onChange={e => setEditPlaylistId(e.target.value)}
                >
                  <option value="">— No playlist —</option>
                  {playlistsData.filter(p => p.category === editCategory).map(p => (
                    <option key={p.id} value={p.id}>{p.name}</option>
                  ))}
                </select>
                <p className="text-[11px] text-muted-foreground mt-1">Only playlists of the selected category are shown.</p>
                <div className="mt-2 rounded-lg border border-border bg-muted/30 p-2.5">
                  <p className="text-xs font-medium text-muted-foreground mb-2">Create new playlist</p>
                  <div className="flex gap-2">
                    <Input
                      className="h-8 text-sm"
                      placeholder="Playlist name..."
                      value={newPlaylistName}
                      onChange={e => setNewPlaylistName(e.target.value)}
                      onKeyDown={e => { if (e.key === "Enter") { e.preventDefault(); handleAddPlaylist(); } }}
                    />
                    <Button
                      type="button"
                      variant="secondary"
                      className="h-8 shrink-0"
                      disabled={!newPlaylistName.trim() || createPlaylistMut.isPending}
                      onClick={() => {
                        const name = newPlaylistName.trim();
                        if (!name) return;
                        playlistsService.create({ name, description: "", thumbnail_url: "", category: editCategory }).then(newPl => {
                          qc.invalidateQueries({ queryKey: ["admin-playlists"] });
                          setEditPlaylistId(newPl.id);
                          setNewPlaylistName("");
                          toast.success(`Playlist "${newPl.name}" created.`);
                        }).catch(() => toast.error("Failed to create playlist."));
                      }}
                    >
                      Add
                    </Button>
                  </div>
                </div>
              </div>
            </div>

            <div className="flex justify-end gap-2 mt-6">
              <Button variant="ghost" onClick={() => setEditVideo(null)}>Cancel</Button>
              <Button onClick={handleEditSave} disabled={editMut.isPending}>
                {editMut.isPending ? "Saving…" : "Save Changes"}
              </Button>
            </div>
          </div>
        </div>
      )}

      {/* Delete Confirmation Modal */}
      {deleteTarget && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-foreground/60"
          onClick={() => setDeleteTarget(null)}
        >
          <div
            className="bg-card rounded-2xl shadow-xl w-full max-w-sm p-6"
            onClick={e => e.stopPropagation()}
          >
            <div className="flex items-center justify-center w-12 h-12 rounded-full bg-red-50 mx-auto mb-4">
              <Trash2 className="h-5 w-5 text-danger" />
            </div>
            <h2 className="text-section-title text-center">Delete Video?</h2>
            <p className="text-sm text-muted-foreground text-center mt-1 mb-1">
              <span className="font-medium text-foreground">"{deleteTarget.title}"</span>
            </p>
            <p className="text-sm text-muted-foreground text-center mb-6">
              Are you sure you want to delete this video? This cannot be undone.
            </p>
            <div className="flex gap-3">
              <Button
                variant="outline"
                className="flex-1"
                onClick={() => setDeleteTarget(null)}
              >
                Cancel
              </Button>
              <Button
                className="flex-1 bg-danger hover:bg-danger/90 text-white"
                disabled={deleteMut.isPending}
                onClick={() => {
                  deleteMut.mutate(deleteTarget.id, {
                    onSettled: () => setDeleteTarget(null),
                  });
                }}
              >
                {deleteMut.isPending ? "Deleting…" : "Delete"}
              </Button>
            </div>
          </div>
        </div>
      )}

      {/* Upload Modal */}
      {showUpload && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4">
          <div className="bg-card rounded-2xl shadow-2xl w-full max-w-3xl flex flex-col max-h-[90vh]">

            {/* Header */}
            <div className="flex items-center justify-between px-6 py-4 border-b border-border shrink-0">
              <div>
                <h2 className="text-base font-semibold text-foreground">Upload Video</h2>
                <p className="text-xs text-muted-foreground mt-0.5">Fill in the details and choose a file to upload</p>
              </div>
              <Button variant="ghost" size="icon" onClick={closeUpload} disabled={isUploading}>
                <Plus className="h-4 w-4 rotate-45" />
              </Button>
            </div>

            {/* Body — two columns */}
            <div className="flex-1 overflow-y-auto">
              <div className="grid grid-cols-1 md:grid-cols-[280px_1fr] divide-y md:divide-y-0 md:divide-x divide-border">

                {/* Left — file + thumbnail */}
                <div className="p-5 space-y-4">
                  {/* Video drop zone */}
                  <div>
                    <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wide mb-2">Video File *</p>
                    <div
                      className={`relative flex flex-col items-center justify-center gap-2 rounded-xl border-2 border-dashed cursor-pointer transition-colors h-44 text-center px-4
                        ${uploadFile ? "border-primary bg-primary/5" : "border-border hover:border-primary hover:bg-muted/40"}`}
                      onClick={() => !isUploading && fileRef.current?.click()}
                    >
                      {uploadFile ? (
                        <>
                          <div className="h-12 w-12 rounded-xl bg-primary/10 flex items-center justify-center">
                            <CloudUpload className="h-6 w-6 text-primary" />
                          </div>
                          <p className="text-sm font-medium text-foreground leading-snug break-all line-clamp-2">{uploadFile.name}</p>
                          <p className="text-xs text-muted-foreground">{(uploadFile.size / 1024 / 1024).toFixed(1)} MB</p>
                          {!isUploading && (
                            <button
                              className="text-xs text-danger hover:underline mt-0.5"
                              onClick={e => { e.stopPropagation(); setUploadFile(null); }}
                            >
                              Remove
                            </button>
                          )}
                        </>
                      ) : (
                        <>
                          <div className="h-12 w-12 rounded-xl bg-muted flex items-center justify-center">
                            <CloudUpload className="h-6 w-6 text-muted-foreground" />
                          </div>
                          <div>
                            <p className="text-sm font-medium text-foreground">Drop video here</p>
                            <p className="text-xs text-muted-foreground mt-0.5">or click to browse</p>
                          </div>
                          <p className="text-xs text-muted-foreground/70">MP4 · MOV · AVI · Max 4 GB</p>
                        </>
                      )}
                      <input
                        ref={fileRef}
                        type="file"
                        accept="video/*"
                        className="hidden"
                        onChange={e => setUploadFile(e.target.files?.[0] ?? null)}
                      />
                    </div>
                  </div>

                  {/* Thumbnail */}
                  <div>
                    <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wide mb-2">
                      Thumbnail <span className="normal-case font-normal">(optional)</span>
                    </p>
                    <div
                      className="relative rounded-xl border border-dashed border-border overflow-hidden cursor-pointer hover:border-primary transition-colors group"
                      style={{ aspectRatio: "16/9" }}
                      onClick={() => !isUploading && thumbRef.current?.click()}
                    >
                      {uploadThumbPreview ? (
                        <>
                          <img src={uploadThumbPreview} alt="Thumbnail" className="w-full h-full object-cover" />
                          <div className="absolute inset-0 bg-black/40 opacity-0 group-hover:opacity-100 transition-opacity flex items-center justify-center">
                            <p className="text-xs text-white font-medium">Click to change</p>
                          </div>
                          {!isUploading && (
                            <button
                              className="absolute top-1.5 right-1.5 h-6 w-6 rounded-full bg-black/60 flex items-center justify-center text-white hover:bg-danger"
                              onClick={e => { e.stopPropagation(); handleThumbChange(null); }}
                            >
                              <Plus className="h-3 w-3 rotate-45" />
                            </button>
                          )}
                        </>
                      ) : (
                        <div className="w-full h-full flex flex-col items-center justify-center gap-1.5 text-muted-foreground">
                          <Upload className="h-5 w-5" />
                          <p className="text-xs font-medium">Upload thumbnail</p>
                          <p className="text-[11px] opacity-60">JPG · PNG · WebP · 16:9</p>
                        </div>
                      )}
                    </div>
                    <input
                      ref={thumbRef}
                      type="file"
                      accept="image/jpeg,image/png,image/webp"
                      className="hidden"
                      onChange={e => handleThumbChange(e.target.files?.[0] ?? null)}
                    />
                  </div>
                </div>

                {/* Right — metadata */}
                <div className="p-5 space-y-4">
                  <div>
                    <label className="text-xs font-semibold text-muted-foreground uppercase tracking-wide block mb-1.5">Title *</label>
                    <Input
                      placeholder="Enter video title"
                      value={uploadTitle}
                      onChange={e => setUploadTitle(e.target.value)}
                    />
                  </div>

                  <div>
                    <label className="text-xs font-semibold text-muted-foreground uppercase tracking-wide block mb-1.5">Description</label>
                    <textarea
                      className="w-full rounded-lg border border-border bg-card px-3 py-2 text-sm min-h-[90px] resize-none focus:outline-none focus:ring-2 focus:ring-ring"
                      placeholder="Video description…"
                      value={uploadDesc}
                      onChange={e => setUploadDesc(e.target.value)}
                    />
                  </div>

                  <div className="grid grid-cols-2 gap-3">
                    <div>
                      <label className="text-xs font-semibold text-muted-foreground uppercase tracking-wide block mb-1.5">Category *</label>
                      <select
                        className="h-10 w-full rounded-lg border border-border bg-card px-3 text-sm"
                        value={uploadCat}
                        onChange={e => {
                          const cat = e.target.value;
                          setUploadCat(cat);
                          setUploadIsFree(cat === "WILLCOM");
                          // A playlist of another category is no longer valid.
                          setUploadPlaylistId("");
                        }}
                      >
                        <option value="WILLCOM">Wilcom 2006</option>
                        <option value="E4">E4</option>
                        <option value="MECAD">meCAD</option>
                      </select>
                    </div>

                    <div>
                      <label className="text-xs font-semibold text-muted-foreground uppercase tracking-wide block mb-1.5">Playlist</label>
                      <select
                        className="h-10 w-full rounded-lg border border-border bg-card px-3 text-sm"
                        value={uploadPlaylistId}
                        onChange={e => setUploadPlaylistId(e.target.value)}
                      >
                        <option value="">— None —</option>
                        {playlistsData.filter(p => p.category === uploadCat).map(p => (
                          <option key={p.id} value={p.id}>{p.name}</option>
                        ))}
                      </select>
                      <p className="text-[11px] text-muted-foreground mt-1">Only playlists matching the category are shown.</p>
                    </div>
                  </div>

                  {/* Inline create playlist */}
                  <div className="rounded-lg bg-muted/40 border border-border px-3 py-2.5 flex items-center gap-2">
                    <Input
                      className="h-8 text-sm bg-card"
                      placeholder="New playlist name…"
                      value={newPlaylistName}
                      onChange={e => setNewPlaylistName(e.target.value)}
                      onKeyDown={e => { if (e.key === "Enter") { e.preventDefault(); handleAddPlaylist(); } }}
                    />
                    <Button
                      type="button"
                      size="sm"
                      variant="secondary"
                      className="h-8 shrink-0"
                      disabled={!newPlaylistName.trim() || createPlaylistMut.isPending}
                      onClick={handleAddPlaylist}
                    >
                      + Create
                    </Button>
                  </div>

                  {/* Free access toggle */}
                  <div className="flex items-center justify-between rounded-lg border border-border bg-muted/20 px-4 py-3">
                    <div>
                      <p className="text-sm font-medium text-foreground">Free Access</p>
                      <p className="text-xs text-muted-foreground mt-0.5">
                        {uploadCat === "WILLCOM" ? "Wilcom 2006 defaults to free." : "E4 / meCAD defaults to paid."}
                      </p>
                    </div>
                    <button
                      type="button"
                      className={`w-11 h-6 rounded-full flex items-center px-0.5 transition-colors ${uploadIsFree ? "bg-primary" : "bg-muted"}`}
                      onClick={() => setUploadIsFree(f => !f)}
                    >
                      <div className={`w-5 h-5 rounded-full bg-white shadow transition-transform ${uploadIsFree ? "translate-x-5" : ""}`} />
                    </button>
                  </div>
                </div>
              </div>

              {/* Attach photos (optional) */}
              <div className="px-5 pb-5">
                <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wide mb-2">
                  Attach Photos <span className="normal-case font-normal">(optional)</span>
                </p>
                <div className="flex flex-wrap gap-3">
                  {uploadPhotosPreviews.map((preview, i) => (
                    <div key={preview} className="relative w-20 h-20 rounded-lg overflow-hidden border border-border group shrink-0">
                      <img src={preview} alt={`Photo ${i + 1}`} className="w-full h-full object-cover" />
                      {!isUploading && (
                        <button
                          className="absolute top-1 right-1 h-5 w-5 rounded-full bg-black/60 flex items-center justify-center text-white hover:bg-danger"
                          onClick={() => removeUploadPhoto(i)}
                        >
                          <Plus className="h-3 w-3 rotate-45" />
                        </button>
                      )}
                    </div>
                  ))}
                  <div
                    className="w-20 h-20 rounded-lg border-2 border-dashed border-border hover:border-primary hover:bg-muted/40 cursor-pointer flex flex-col items-center justify-center gap-1 text-muted-foreground shrink-0 transition-colors"
                    onClick={() => !isUploading && photosRef.current?.click()}
                  >
                    <Upload className="h-4 w-4" />
                    <p className="text-[10px] font-medium">Add</p>
                  </div>
                  <input
                    ref={photosRef}
                    type="file"
                    accept="image/*"
                    multiple
                    className="hidden"
                    onChange={e => { handleAddPhotos(e.target.files); e.target.value = ""; }}
                  />
                </div>
                <p className="text-xs text-muted-foreground/70 mt-2">
                  Shown alongside this video's description and filed in the Photos gallery under the same category.
                </p>
              </div>
            </div>

            {/* Footer — progress + actions */}
            <div className="border-t border-border px-6 py-4 shrink-0 space-y-3">
              {isUploading && (
                <div>
                  <div className="flex justify-between text-xs text-muted-foreground mb-1.5">
                    <span>{isAttachingPhotos ? "Attaching photos… do not close this window" : "Uploading… do not close this window"}</span>
                    <span className="font-medium text-foreground">{isAttachingPhotos ? "" : `${uploadProgress}%`}</span>
                  </div>
                  <div className="w-full bg-muted rounded-full h-2">
                    <div
                      className="bg-primary h-2 rounded-full transition-all duration-300"
                      style={{ width: isAttachingPhotos ? "100%" : `${uploadProgress}%` }}
                    />
                  </div>
                </div>
              )}
              <div className="flex justify-end gap-2">
                <Button variant="outline" onClick={closeUpload} disabled={isUploading}>Cancel</Button>
                <Button onClick={handleUpload} disabled={isUploading || !uploadFile}>
                  {isAttachingPhotos ? "Attaching photos…" : isUploading ? `Uploading ${uploadProgress}%…` : "Upload & Publish"}
                </Button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
