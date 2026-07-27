import { useState, useRef } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { PageHeader } from "@/components/PageHeader";
import { StatusBadge } from "@/components/StatusBadge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import {
  FolderOpen, Folder, Plus, Trash2, Pencil, X,
  Video as VideoIcon, Check, ListVideo, Upload, Image, GripVertical,
} from "lucide-react";
import { toast } from "sonner";
import { playlistsService, AdminPlaylist } from "@/services/playlists";
import { videosService } from "@/services/videos";
import { api } from "@/lib/api";
import { cn } from "@/lib/utils";
import { CONTENT_CATEGORIES, categoryLabel, categoryBadgeVariant, DEFAULT_CATEGORY, CATEGORY_TEXT_COLORS } from "@/lib/featureCatalog";

interface PlaylistVideo {
  id: string;
  title: string;
  description?: string;
  category: string;
  thumbnail_url?: string;
  preview_url?: string;
  duration_seconds?: number;
  is_free: boolean;
  is_preview: boolean;
  status: string;
  uploaded_at: string;
}

interface PlaylistDetail extends AdminPlaylist {
  videos: PlaylistVideo[];
}

interface VideoOption {
  id: string;
  title: string;
  category: string;
  thumbnail_url?: string;
  status: string;
}

function fmtDuration(s?: number) {
  if (!s) return "—";
  const m = Math.floor(s / 60);
  const sec = s % 60;
  return `${m}m ${String(sec).padStart(2, "0")}s`;
}

const statusVariant = (s: string) => {
  if (s === "PUBLISHED") return "success" as const;
  if (s === "PROCESSING") return "warning" as const;
  if (s === "ERROR") return "danger" as const;
  return "neutral" as const;
};

const catVariant = (c: string) =>
  categoryBadgeVariant(c);

const catLabel = (c: string) =>
  categoryLabel(c);

const catColor = CATEGORY_TEXT_COLORS;

export default function PlaylistsPage() {
  const qc = useQueryClient();

  const [openId, setOpenId] = useState<string | null>(null);
  const [showCreate, setShowCreate] = useState(false);
  const [editTarget, setEditTarget] = useState<AdminPlaylist | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<AdminPlaylist | null>(null);
  const [showVideoPicker, setShowVideoPicker] = useState(false);
  const [pickerSearch, setPickerSearch] = useState("");
  const [pickerSelected, setPickerSelected] = useState<string[]>([]);
  const [dragIndex, setDragIndex] = useState<number | null>(null);
  const [dragOverIndex, setDragOverIndex] = useState<number | null>(null);

  const [formName, setFormName] = useState("");
  const [formDesc, setFormDesc] = useState("");
  const [formCategory, setFormCategory] = useState<string>(DEFAULT_CATEGORY);

  // Thumbnail state for create modal
  const [createThumb, setCreateThumb] = useState<File | null>(null);
  const [createThumbPreview, setCreateThumbPreview] = useState<string | null>(null);
  const createThumbRef = useRef<HTMLInputElement>(null);

  // Thumbnail state for edit modal
  const [editThumb, setEditThumb] = useState<File | null>(null);
  const [editThumbPreview, setEditThumbPreview] = useState<string | null>(null);
  const editThumbRef = useRef<HTMLInputElement>(null);

  // ── Queries ──
  const { data: playlists = [], isLoading } = useQuery({
    queryKey: ["admin-playlists"],
    queryFn: () => playlistsService.list(),
  });

  const { data: openDetail, isLoading: detailLoading } = useQuery<PlaylistDetail>({
    queryKey: ["admin-playlist-detail", openId],
    queryFn: () => playlistsService.getVideos(openId!),
    enabled: !!openId,
  });

  const { data: allVideosData } = useQuery({
    queryKey: ["videos-all-published"],
    queryFn: () => videosService.list({ page: 1, limit: 500, status: "PUBLISHED" }),
    enabled: showVideoPicker,
  });
  const allVideos: VideoOption[] = allVideosData?.data ?? [];

  // ── Mutations ──
  const createMut = useMutation({
    mutationFn: () => {
      if (createThumb) {
        const fd = new FormData();
        fd.append("name", formName.trim());
        fd.append("description", formDesc.trim());
        fd.append("category", formCategory);
        fd.append("thumbnail", createThumb);
        return api.post("/admin/playlists", fd).then(r => r.data.data as AdminPlaylist);
      }
      return playlistsService.create({ name: formName.trim(), description: formDesc.trim(), thumbnail_url: "", category: formCategory });
    },
    onSuccess: (pl) => {
      qc.invalidateQueries({ queryKey: ["admin-playlists"] });
      toast.success(`Playlist "${pl.name}" created.`);
      setShowCreate(false);
      setFormName(""); setFormDesc("");
      setCreateThumb(null); setCreateThumbPreview(null);
      setOpenId(pl.id);
    },
    onError: () => toast.error("Failed to create playlist."),
  });

  const updateMut = useMutation({
    mutationFn: () => {
      if (editThumb) {
        const fd = new FormData();
        fd.append("name", formName.trim());
        fd.append("description", formDesc.trim());
        fd.append("category", formCategory);
        fd.append("thumbnail", editThumb);
        return api.patch(`/admin/playlists/${editTarget!.id}`, fd).then(r => r.data.data);
      }
      return playlistsService.update(editTarget!.id, { name: formName.trim(), description: formDesc.trim(), category: formCategory });
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["admin-playlists"] });
      toast.success("Playlist updated.");
      setEditTarget(null);
      setFormName(""); setFormDesc("");
      setEditThumb(null); setEditThumbPreview(null);
    },
    onError: () => toast.error("Failed to update playlist."),
  });

  const deleteMut = useMutation({
    mutationFn: (id: string) => playlistsService.delete(id),
    onSuccess: (_, id) => {
      qc.invalidateQueries({ queryKey: ["admin-playlists"] });
      toast.success("Playlist deleted.");
      setDeleteTarget(null);
      if (openId === id) setOpenId(null);
    },
    onError: () => toast.error("Failed to delete playlist."),
  });

  const setVideosMut = useMutation({
    mutationFn: (ids: string[]) => playlistsService.setVideos(openId!, ids),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["admin-playlist-detail", openId] });
      qc.invalidateQueries({ queryKey: ["admin-playlists"] });
      toast.success("Videos updated.");
      setShowVideoPicker(false);
    },
    onError: () => toast.error("Failed to update videos."),
  });

  // ── Helpers ──
  function openEdit(pl: AdminPlaylist, e: React.MouseEvent) {
    e.stopPropagation();
    setEditTarget(pl);
    setFormName(pl.name);
    setFormDesc(pl.description);
    setFormCategory(pl.category || DEFAULT_CATEGORY);
    setEditThumb(null);
    setEditThumbPreview(pl.thumbnail_url || null);
  }

  function openDelete(pl: AdminPlaylist, e: React.MouseEvent) {
    e.stopPropagation();
    setDeleteTarget(pl);
  }

  function openVideoPicker() {
    setPickerSelected((openDetail?.videos ?? []).map(v => v.id));
    setPickerSearch("");
    setShowVideoPicker(true);
  }

  function removeVideo(videoId: string) {
    const newIds = (openDetail?.videos ?? []).map(v => v.id).filter(id => id !== videoId);
    setVideosMut.mutate(newIds);
  }

  function handleRowDrop(targetIndex: number) {
    const videos = openDetail?.videos ?? [];
    if (dragIndex === null || dragIndex === targetIndex || !videos.length) {
      setDragIndex(null);
      setDragOverIndex(null);
      return;
    }
    const reordered = [...videos];
    const [moved] = reordered.splice(dragIndex, 1);
    reordered.splice(targetIndex, 0, moved);
    setVideosMut.mutate(reordered.map(v => v.id));
    setDragIndex(null);
    setDragOverIndex(null);
  }

  function togglePicker(id: string) {
    setPickerSelected(prev =>
      prev.includes(id) ? prev.filter(v => v !== id) : [...prev, id]
    );
  }

  function handleCreateThumbChange(file: File | null) {
    setCreateThumb(file);
    setCreateThumbPreview(file ? URL.createObjectURL(file) : null);
  }

  function handleEditThumbChange(file: File | null) {
    setEditThumb(file);
    setEditThumbPreview(file ? URL.createObjectURL(file) : (editTarget?.thumbnail_url || null));
  }

  const openPlaylist = playlists.find(p => p.id === openId);

  // The picker only offers videos matching the playlist's category — a
  // playlist never holds videos of another category (the API enforces this
  // too). Legacy playlists without a category show everything.
  const filteredVideos = allVideos
    .filter(v => !openPlaylist?.category || v.category === openPlaylist.category)
    .filter(v =>
      v.title.toLowerCase().includes(pickerSearch.toLowerCase()) ||
      v.category.toLowerCase().includes(pickerSearch.toLowerCase())
    );

  return (
    <div className="p-4 md:p-6 space-y-6">
      <PageHeader
        title="Playlists"
        description="Organise videos into playlists"
        action={
          <Button onClick={() => { setShowCreate(true); setFormName(""); setFormDesc(""); setFormCategory(DEFAULT_CATEGORY); setCreateThumb(null); setCreateThumbPreview(null); }}>
            <Plus className="h-4 w-4 mr-1.5" /> New Playlist
          </Button>
        }
      />

      {/* ── Folder grid ── */}
      {isLoading ? (
        <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 gap-4">
          {Array.from({ length: 8 }).map((_, i) => <Skeleton key={i} className="h-40 rounded-xl" />)}
        </div>
      ) : playlists.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-20 text-muted-foreground gap-3">
          <Folder className="h-16 w-16 opacity-30" />
          <p className="text-sm">No playlists yet. Create one to get started.</p>
        </div>
      ) : (
        <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 gap-4">
          {playlists.map(pl => {
            const isOpen = openId === pl.id;
            return (
              <div
                key={pl.id}
                onClick={() => setOpenId(isOpen ? null : pl.id)}
                className={cn(
                  "relative group cursor-pointer rounded-xl overflow-hidden border-2 transition-all select-none",
                  isOpen ? "border-primary shadow-lg" : "border-transparent hover:border-primary/40 hover:shadow-md"
                )}
              >
                {/* Cover image / gradient */}
                <div className="h-32 w-full relative">
                  {pl.thumbnail_url ? (
                    <img
                      src={pl.thumbnail_url}
                      alt={pl.name}
                      className="w-full h-full object-cover"
                    />
                  ) : (
                    <div className="w-full h-full bg-gradient-to-br from-amber-100 via-amber-200 to-orange-200 flex items-center justify-center">
                      {isOpen
                        ? <FolderOpen className="h-12 w-12 text-amber-500/70" />
                        : <Folder className="h-12 w-12 text-amber-400/70" />
                      }
                    </div>
                  )}

                  {/* Selected overlay tint */}
                  {isOpen && (
                    <div className="absolute inset-0 bg-primary/10" />
                  )}

                  {/* Hover edit/delete buttons */}
                  <div className="absolute top-2 right-2 hidden group-hover:flex items-center gap-1">
                    <button
                      className="h-7 w-7 flex items-center justify-center rounded-lg bg-black/60 text-white hover:bg-black/80 backdrop-blur-sm"
                      onClick={e => openEdit(pl, e)}
                    >
                      <Pencil className="h-3.5 w-3.5" />
                    </button>
                    <button
                      className="h-7 w-7 flex items-center justify-center rounded-lg bg-black/60 text-white hover:bg-red-600/80 backdrop-blur-sm"
                      onClick={e => openDelete(pl, e)}
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                    </button>
                  </div>
                </div>

                {/* Info bar */}
                <div className="bg-card px-3 py-2.5 flex items-center justify-between gap-2">
                  <div className="min-w-0">
                    <p className="text-sm font-semibold text-foreground truncate leading-tight">{pl.name}</p>
                    <p className="text-xs text-muted-foreground mt-0.5">
                      {pl.video_count} {pl.video_count === 1 ? "video" : "videos"}
                      {pl.category && (
                        <span className={cn("ml-1.5 font-medium", catColor[pl.category] ?? "")}>
                          · {catLabel(pl.category)}
                        </span>
                      )}
                    </p>
                  </div>
                  {isOpen
                    ? <FolderOpen className="h-4 w-4 text-primary shrink-0" />
                    : <Folder className="h-4 w-4 text-muted-foreground shrink-0 group-hover:text-primary transition-colors" />
                  }
                </div>
              </div>
            );
          })}
        </div>
      )}

      {/* ── Open folder detail ── */}
      {openId && (
        <div className="rounded-xl border border-border bg-card overflow-hidden">
          <div className="flex items-center gap-3 px-5 py-4 border-b border-border bg-muted/20">
            <FolderOpen className="h-5 w-5 text-amber-400 shrink-0" />
            <div className="flex-1 min-w-0">
              <p className="font-semibold text-foreground truncate">{openPlaylist?.name}</p>
              {openPlaylist?.description && (
                <p className="text-xs text-muted-foreground truncate mt-0.5">{openPlaylist.description}</p>
              )}
            </div>
            <Button size="sm" variant="outline" className="shrink-0" onClick={openVideoPicker}>
              <ListVideo className="h-4 w-4 mr-1.5" /> Add / Remove Videos
            </Button>
            <button className="text-muted-foreground hover:text-foreground" onClick={() => setOpenId(null)}>
              <X className="h-4 w-4" />
            </button>
          </div>

          {detailLoading ? (
            <div className="p-4 space-y-3">
              {Array.from({ length: 4 }).map((_, i) => <Skeleton key={i} className="h-14" />)}
            </div>
          ) : !openDetail?.videos?.length ? (
            <div className="flex flex-col items-center justify-center py-14 text-muted-foreground gap-2">
              <VideoIcon className="h-10 w-10 opacity-30" />
              <p className="text-sm font-medium">No videos in this playlist yet.</p>
              <Button size="sm" variant="outline" className="mt-2" onClick={openVideoPicker}>
                <Plus className="h-4 w-4 mr-1.5" /> Add Videos
              </Button>
            </div>
          ) : (
            <table className="w-full">
              <thead>
                <tr className="bg-table-header">
                  <th className="text-table-header text-left px-4 py-3 w-8">#</th>
                  <th className="text-table-header text-left px-4 py-3">Video</th>
                  <th className="text-table-header text-left px-4 py-3 hidden md:table-cell">Category</th>
                  <th className="text-table-header text-left px-4 py-3 hidden md:table-cell">Duration</th>
                  <th className="text-table-header text-left px-4 py-3">Status</th>
                  <th className="text-table-header text-left px-4 py-3 hidden md:table-cell">Preview</th>
                  <th className="text-table-header text-left px-4 py-3 hidden md:table-cell">Access</th>
                  <th className="text-table-header text-left px-4 py-3 hidden lg:table-cell">Uploaded</th>
                  <th className="text-table-header px-4 py-3" />
                </tr>
              </thead>
              <tbody>
                {openDetail.videos.map((v, i) => (
                  <tr
                    key={v.id}
                    draggable={!setVideosMut.isPending}
                    onDragStart={() => setDragIndex(i)}
                    onDragOver={e => { e.preventDefault(); if (dragOverIndex !== i) setDragOverIndex(i); }}
                    onDrop={() => handleRowDrop(i)}
                    onDragEnd={() => { setDragIndex(null); setDragOverIndex(null); }}
                    className={cn(
                      "border-b border-border last:border-0 hover:bg-table-hover transition-colors group/row",
                      dragIndex === i && "opacity-40",
                      dragOverIndex === i && dragIndex !== null && dragIndex !== i && "border-t-2 border-t-primary",
                    )}
                  >
                    <td className="px-4 py-3 text-xs text-muted-foreground">
                      <div className="flex items-center gap-1.5 justify-end cursor-grab active:cursor-grabbing">
                        <GripVertical className="h-3.5 w-3.5 opacity-40 group-hover/row:opacity-100 transition-opacity" />
                        {i + 1}
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-3">
                        <div className="w-16 h-10 rounded-lg bg-muted flex items-center justify-center shrink-0 overflow-hidden">
                          {v.thumbnail_url ? (
                            <img src={v.thumbnail_url} alt={v.title} className="w-full h-full object-cover" />
                          ) : v.preview_url ? (
                            <video src={v.preview_url} className="w-full h-full object-cover" muted playsInline preload="metadata" />
                          ) : (
                            <Upload className="h-4 w-4 text-muted-foreground" />
                          )}
                        </div>
                        <div>
                          <p className="text-sm font-medium text-foreground">{v.title}</p>
                          {v.description && (
                            <p className="text-xs text-muted-foreground truncate max-w-[200px]">{v.description}</p>
                          )}
                        </div>
                      </div>
                    </td>
                    <td className="px-4 py-3 hidden md:table-cell">
                      <StatusBadge variant={catVariant(v.category)}>{catLabel(v.category)}</StatusBadge>
                    </td>
                    <td className="px-4 py-3 text-sm text-muted-foreground font-mono hidden md:table-cell">
                      {fmtDuration(v.duration_seconds)}
                    </td>
                    <td className="px-4 py-3">
                      <StatusBadge variant={statusVariant(v.status)}>
                        {v.status.charAt(0) + v.status.slice(1).toLowerCase()}
                      </StatusBadge>
                    </td>
                    <td className="px-4 py-3 hidden md:table-cell">
                      <div className={cn("w-9 h-5 rounded-full flex items-center px-0.5", v.is_preview ? "bg-primary" : "bg-muted")}>
                        <div className={cn("w-4 h-4 rounded-full bg-card transition-transform", v.is_preview ? "translate-x-4" : "")} />
                      </div>
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
                      <button
                        className="h-7 w-7 flex items-center justify-center rounded hover:bg-red-50 text-muted-foreground hover:text-danger opacity-0 group-hover/row:opacity-100 transition-opacity"
                        title="Remove from playlist"
                        onClick={() => removeVideo(v.id)}
                        disabled={setVideosMut.isPending}
                      >
                        <X className="h-4 w-4" />
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}

      {/* ── Video picker modal ── */}
      {showVideoPicker && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
          <div className="bg-card border border-border rounded-2xl w-full max-w-lg mx-4 flex flex-col max-h-[85vh]">
            <div className="flex items-center gap-3 px-6 py-4 border-b border-border shrink-0">
              <ListVideo className="h-5 w-5 text-primary" />
              <div className="flex-1">
                <h2 className="text-base font-semibold">Add / Remove Videos</h2>
                <p className="text-xs text-muted-foreground mt-0.5">
                  {openPlaylist?.name} · {pickerSelected.length} selected
                </p>
              </div>
              <button className="text-muted-foreground hover:text-foreground" onClick={() => setShowVideoPicker(false)}>
                <X className="h-5 w-5" />
              </button>
            </div>
            <div className="px-4 py-3 border-b border-border shrink-0">
              <Input autoFocus placeholder="Search videos…" value={pickerSearch} onChange={e => setPickerSearch(e.target.value)} />
            </div>
            <div className="overflow-y-auto flex-1 px-4 py-2 space-y-1">
              {allVideos.length === 0 && <p className="text-sm text-muted-foreground text-center py-8">Loading videos…</p>}
              {filteredVideos.length === 0 && allVideos.length > 0 && (
                <p className="text-sm text-muted-foreground text-center py-8">No videos match "{pickerSearch}".</p>
              )}
              {filteredVideos.map(v => {
                const selected = pickerSelected.includes(v.id);
                return (
                  <button
                    key={v.id}
                    onClick={() => togglePicker(v.id)}
                    className={cn(
                      "flex items-center gap-3 w-full px-3 py-2 rounded-lg text-left transition-colors",
                      selected ? "bg-primary/10 border border-primary/30" : "hover:bg-muted border border-transparent"
                    )}
                  >
                    {v.thumbnail_url ? (
                      <img src={v.thumbnail_url} alt="" className="w-12 h-8 rounded object-cover shrink-0" />
                    ) : (
                      <div className="w-12 h-8 rounded bg-muted shrink-0 flex items-center justify-center">
                        <VideoIcon className="h-4 w-4 text-muted-foreground" />
                      </div>
                    )}
                    <div className="flex-1 min-w-0">
                      <p className="text-sm text-foreground truncate">{v.title}</p>
                      <p className={cn("text-xs font-medium", catColor[v.category] ?? "text-muted-foreground")}>
                        {catLabel(v.category)}
                      </p>
                    </div>
                    {selected && (
                      <div className="h-5 w-5 rounded-full bg-primary flex items-center justify-center shrink-0">
                        <Check className="h-3 w-3 text-primary-foreground" />
                      </div>
                    )}
                  </button>
                );
              })}
            </div>
            <div className="flex items-center justify-between px-6 py-4 border-t border-border shrink-0">
              <p className="text-xs text-muted-foreground">{pickerSelected.length} video{pickerSelected.length !== 1 ? "s" : ""} selected</p>
              <div className="flex gap-2">
                <Button variant="outline" onClick={() => setShowVideoPicker(false)}>Cancel</Button>
                <Button disabled={setVideosMut.isPending} onClick={() => setVideosMut.mutate(pickerSelected)}>
                  {setVideosMut.isPending ? "Saving…" : "Save"}
                </Button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* ── Create modal ── */}
      {showCreate && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
          <div className="bg-card border border-border rounded-2xl w-full max-w-sm mx-4 overflow-hidden">
            <div className="flex items-center gap-3 px-5 py-4 border-b border-border">
              <Folder className="h-5 w-5 text-amber-400" />
              <h2 className="text-base font-semibold flex-1">New Playlist</h2>
              <button className="text-muted-foreground hover:text-foreground" onClick={() => setShowCreate(false)}>
                <X className="h-5 w-5" />
              </button>
            </div>

            <div className="p-5 space-y-4">
              {/* Thumbnail picker */}
              <div>
                <label className="text-xs font-semibold text-muted-foreground uppercase tracking-wide block mb-1.5">
                  Cover Image <span className="normal-case font-normal">(optional)</span>
                </label>
                <div
                  className="relative rounded-xl border border-dashed border-border overflow-hidden cursor-pointer hover:border-primary transition-colors group"
                  style={{ aspectRatio: "16/9" }}
                  onClick={() => createThumbRef.current?.click()}
                >
                  {createThumbPreview ? (
                    <>
                      <img src={createThumbPreview} alt="Cover" className="w-full h-full object-cover" />
                      <div className="absolute inset-0 bg-black/40 opacity-0 group-hover:opacity-100 transition-opacity flex items-center justify-center">
                        <p className="text-xs text-white font-medium">Click to change</p>
                      </div>
                      <button
                        className="absolute top-1.5 right-1.5 h-6 w-6 rounded-full bg-black/60 flex items-center justify-center text-white hover:bg-danger"
                        onClick={e => { e.stopPropagation(); handleCreateThumbChange(null); }}
                      >
                        <X className="h-3 w-3" />
                      </button>
                    </>
                  ) : (
                    <div className="w-full h-full flex flex-col items-center justify-center gap-1.5 text-muted-foreground">
                      <Image className="h-6 w-6" />
                      <p className="text-xs font-medium">Upload cover image</p>
                      <p className="text-[11px] opacity-60">JPG · PNG · WebP · 16:9</p>
                    </div>
                  )}
                </div>
                <input
                  ref={createThumbRef}
                  type="file"
                  accept="image/jpeg,image/png,image/webp"
                  className="hidden"
                  onChange={e => handleCreateThumbChange(e.target.files?.[0] ?? null)}
                />
              </div>

              <div>
                <label className="text-xs font-semibold text-muted-foreground uppercase tracking-wide block mb-1.5">Name *</label>
                <Input
                  autoFocus
                  value={formName}
                  onChange={e => setFormName(e.target.value)}
                  onKeyDown={e => { if (e.key === "Enter" && formName.trim()) createMut.mutate(); }}
                  placeholder="e.g. Getting Started"
                />
              </div>
              <div>
                <label className="text-xs font-semibold text-muted-foreground uppercase tracking-wide block mb-1.5">Description</label>
                <Input value={formDesc} onChange={e => setFormDesc(e.target.value)} placeholder="Optional" />
              </div>
              <div>
                <label className="text-xs font-semibold text-muted-foreground uppercase tracking-wide block mb-1.5">Category *</label>
                <select
                  className="w-full h-10 rounded-lg border border-border bg-background px-3 text-sm"
                  value={formCategory}
                  onChange={e => setFormCategory(e.target.value)}
                >
                  {CONTENT_CATEGORIES.map((c) => (
                    <option key={c} value={c}>{categoryLabel(c)}</option>
                  ))}
                </select>
                <p className="text-[11px] text-muted-foreground mt-1">Only videos of this category can be added to the playlist.</p>
              </div>
            </div>

            <div className="flex justify-end gap-2 px-5 pb-5">
              <Button variant="outline" onClick={() => setShowCreate(false)}>Cancel</Button>
              <Button disabled={!formName.trim() || createMut.isPending} onClick={() => createMut.mutate()}>Create</Button>
            </div>
          </div>
        </div>
      )}

      {/* ── Edit modal ── */}
      {editTarget && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
          <div className="bg-card border border-border rounded-2xl w-full max-w-sm mx-4 overflow-hidden">
            <div className="flex items-center gap-3 px-5 py-4 border-b border-border">
              <FolderOpen className="h-5 w-5 text-amber-400" />
              <h2 className="text-base font-semibold flex-1">Edit Playlist</h2>
              <button className="text-muted-foreground hover:text-foreground" onClick={() => setEditTarget(null)}>
                <X className="h-5 w-5" />
              </button>
            </div>

            <div className="p-5 space-y-4">
              {/* Thumbnail picker */}
              <div>
                <label className="text-xs font-semibold text-muted-foreground uppercase tracking-wide block mb-1.5">
                  Cover Image
                </label>
                <div
                  className="relative rounded-xl border border-dashed border-border overflow-hidden cursor-pointer hover:border-primary transition-colors group"
                  style={{ aspectRatio: "16/9" }}
                  onClick={() => editThumbRef.current?.click()}
                >
                  {editThumbPreview ? (
                    <>
                      <img src={editThumbPreview} alt="Cover" className="w-full h-full object-cover" />
                      <div className="absolute inset-0 bg-black/40 opacity-0 group-hover:opacity-100 transition-opacity flex items-center justify-center">
                        <p className="text-xs text-white font-medium">Click to change</p>
                      </div>
                      <button
                        className="absolute top-1.5 right-1.5 h-6 w-6 rounded-full bg-black/60 flex items-center justify-center text-white hover:bg-danger"
                        onClick={e => { e.stopPropagation(); handleEditThumbChange(null); }}
                      >
                        <X className="h-3 w-3" />
                      </button>
                    </>
                  ) : (
                    <div className="w-full h-full flex flex-col items-center justify-center gap-1.5 text-muted-foreground">
                      <Image className="h-6 w-6" />
                      <p className="text-xs font-medium">Upload cover image</p>
                      <p className="text-[11px] opacity-60">JPG · PNG · WebP · 16:9</p>
                    </div>
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

              <div>
                <label className="text-xs font-semibold text-muted-foreground uppercase tracking-wide block mb-1.5">Name *</label>
                <Input
                  autoFocus
                  value={formName}
                  onChange={e => setFormName(e.target.value)}
                  onKeyDown={e => { if (e.key === "Enter" && formName.trim()) updateMut.mutate(); }}
                />
              </div>
              <div>
                <label className="text-xs font-semibold text-muted-foreground uppercase tracking-wide block mb-1.5">Description</label>
                <Input value={formDesc} onChange={e => setFormDesc(e.target.value)} />
              </div>
              <div>
                <label className="text-xs font-semibold text-muted-foreground uppercase tracking-wide block mb-1.5">Category *</label>
                <select
                  className="w-full h-10 rounded-lg border border-border bg-background px-3 text-sm"
                  value={formCategory}
                  onChange={e => setFormCategory(e.target.value)}
                >
                  {CONTENT_CATEGORIES.map((c) => (
                    <option key={c} value={c}>{categoryLabel(c)}</option>
                  ))}
                </select>
                <p className="text-[11px] text-muted-foreground mt-1">Only videos of this category can be added to the playlist.</p>
              </div>
            </div>

            <div className="flex justify-end gap-2 px-5 pb-5">
              <Button variant="outline" onClick={() => setEditTarget(null)}>Cancel</Button>
              <Button disabled={!formName.trim() || updateMut.isPending} onClick={() => updateMut.mutate()}>Save</Button>
            </div>
          </div>
        </div>
      )}

      {/* ── Delete confirm ── */}
      {deleteTarget && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
          <div className="bg-card border border-border rounded-2xl w-full max-w-sm mx-4 p-6 space-y-4">
            <div className="flex items-center justify-center w-12 h-12 rounded-full bg-red-50 mx-auto">
              <Trash2 className="h-5 w-5 text-danger" />
            </div>
            <h2 className="text-lg font-semibold text-center">Delete Playlist?</h2>
            <p className="text-sm text-muted-foreground text-center">
              Delete <strong>"{deleteTarget.name}"</strong>? Videos won't be deleted, only removed from this playlist.
            </p>
            <div className="flex justify-end gap-2">
              <Button variant="outline" onClick={() => setDeleteTarget(null)}>Cancel</Button>
              <Button variant="destructive" disabled={deleteMut.isPending} onClick={() => deleteMut.mutate(deleteTarget.id)}>
                Delete
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
