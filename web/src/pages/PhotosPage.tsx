import { useState, useRef } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { PageHeader } from "@/components/PageHeader";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { Plus, MoreHorizontal, CloudUpload, Trash2, X } from "lucide-react";
import { toast } from "sonner";
import { photosService, photoUrl, photoCategoriesService, PhotoCategoryItem } from "@/services/photos";
import { api } from "@/lib/api";

interface Photo {
  id: string;
  title: string;
  category: string;
  file_key: string;
  url?: string;
  mime_type: string;
  size_bytes: number;
  is_published: boolean;
  uploaded_at: string;
}

const CATEGORY_LABELS: Record<string, string> = {
  GENERAL: "General", WILLCOM: "Wilcom 2006", E4: "E4", MECAD: "meCAD",
};

export default function PhotosPage() {
  const qc = useQueryClient();
  const [showUpload, setShowUpload] = useState(false);
  const [menuId, setMenuId] = useState<string | null>(null);
  const [uploadTitle, setUploadTitle] = useState("");
  const [uploadCategory, setUploadCategory] = useState("GENERAL");
  const [uploadFile, setUploadFile] = useState<File | null>(null);
  const [newCategory, setNewCategory] = useState("");
  const fileRef = useRef<HTMLInputElement>(null);

  const { data, isLoading } = useQuery({
    queryKey: ["photos"],
    queryFn: () => photosService.list(),
  });

  const { data: categories = [] } = useQuery({
    queryKey: ["photo-categories"],
    queryFn: () => photoCategoriesService.list(),
  });

  const photos: Photo[] = data?.data ?? [];

  const deleteMut = useMutation({
    mutationFn: (id: string) => photosService.delete(id),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ["photos"] }); toast.success("Photo deleted."); },
    onError: () => toast.error("Failed to delete photo."),
  });

  const togglePublishMut = useMutation({
    mutationFn: ({ id, is_published }: { id: string; is_published: boolean }) =>
      photosService.update(id, { is_published }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["photos"] }),
    onError: () => toast.error("Failed to update photo."),
  });

  const createMut = useMutation({
    mutationFn: async (fd: FormData) => {
      const r = await api.post("/photos", fd, { headers: { "Content-Type": "multipart/form-data" } });
      return r.data.data;
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["photos"] });
      setShowUpload(false);
      setUploadTitle(""); setUploadFile(null);
      toast.success("Photo uploaded.");
    },
    onError: () => toast.error("Failed to upload photo."),
  });

  const createCategoryMut = useMutation({
    mutationFn: (name: string) => photoCategoriesService.create(name),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["photo-categories"] });
      setNewCategory("");
      toast.success("Category added.");
    },
    onError: () => toast.error("Failed to add category."),
  });

  const deleteCategoryMut = useMutation({
    mutationFn: (id: string) => photoCategoriesService.delete(id),
    onSuccess: (_data, id) => {
      qc.invalidateQueries({ queryKey: ["photo-categories"] });
      // If the removed category was selected, fall back to the first remaining one.
      const removed = categories.find(c => c.id === id);
      if (removed && removed.name === uploadCategory) {
        const next = categories.find(c => c.id !== id);
        setUploadCategory(next?.name ?? "");
      }
      toast.success("Category deleted.");
    },
    onError: () => toast.error("Failed to delete category."),
  });

  const handleAddCategory = () => {
    const name = newCategory.trim();
    if (!name) { toast.error("Please enter a category name."); return; }
    if (categories.some(c => c.name.toLowerCase() === name.toLowerCase())) {
      toast.error("Category already exists.");
      return;
    }
    createCategoryMut.mutate(name);
  };

  const handleUpload = () => {
    if (!uploadTitle.trim()) { toast.error("Please enter a title."); return; }
    if (!uploadFile) { toast.error("Please select a photo."); return; }
    const fd = new FormData();
    fd.append("title", uploadTitle);
    fd.append("category", uploadCategory);
    fd.append("file", uploadFile);
    createMut.mutate(fd);
  };

  return (
    <div onClick={() => setMenuId(null)}>
      <PageHeader title="Practice Photos" subtitle="Accessible to all plan holders">
        <Button onClick={() => {
          setUploadCategory(categories[0]?.name ?? "GENERAL");
          setShowUpload(true);
        }}><Plus className="h-4 w-4" /> Upload Photos</Button>
      </PageHeader>

      {isLoading ? (
        <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-4">
          {Array(8).fill(0).map((_, i) => <Skeleton key={i} className="aspect-square rounded-xl" />)}
        </div>
      ) : (
        <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-4">
          {photos.map(p => (
            <div key={p.id} className="rounded-xl border border-border bg-card group">
              <div className="aspect-square bg-muted flex items-center justify-center overflow-hidden">
                {p.file_key
                  ? <img src={p.url || photoUrl(p.file_key)} alt={p.title} className="w-full h-full object-cover" />
                  : <span className="text-4xl text-muted-foreground/30">🧵</span>
                }
              </div>
              <div className="p-3 flex items-center justify-between">
                <div>
                  <p className="text-sm font-medium text-foreground">{p.title}</p>
                  {p.category && p.category !== "GENERAL" && (
                    <span className="inline-block text-[10px] font-semibold px-1.5 py-0.5 rounded bg-primary/10 text-primary mt-0.5">
                      {CATEGORY_LABELS[p.category] ?? p.category}
                    </span>
                  )}
                  <div className="flex items-center gap-2 mt-1">
                    <button
                      className={`w-8 h-4 rounded-full flex items-center px-0.5 cursor-pointer transition-colors ${p.is_published ? "bg-primary" : "bg-muted"}`}
                      onClick={() => togglePublishMut.mutate({ id: p.id, is_published: !p.is_published })}
                    >
                      <div className={`w-3 h-3 rounded-full bg-card transition-transform ${p.is_published ? "translate-x-4" : ""}`} />
                    </button>
                    <span className="text-caption">{p.is_published ? "Published" : "Draft"}</span>
                  </div>
                </div>
                <div className="relative" onClick={e => e.stopPropagation()}>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="opacity-0 group-hover:opacity-100"
                    onClick={() => setMenuId(menuId === p.id ? null : p.id)}
                  >
                    <MoreHorizontal className="h-4 w-4" />
                  </Button>
                  {menuId === p.id && (
                    <div className="absolute right-0 top-full mt-1 w-36 bg-card border border-border rounded-lg shadow-lg z-50 py-1">
                      <button
                        className="flex items-center gap-2 w-full px-3 py-2 text-sm text-danger hover:bg-muted"
                        onClick={() => { deleteMut.mutate(p.id); setMenuId(null); }}
                      >
                        <Trash2 className="h-3.5 w-3.5" /> Delete
                      </button>
                    </div>
                  )}
                </div>
              </div>
            </div>
          ))}
          {photos.length === 0 && (
            <div className="col-span-full py-16 text-center text-sm text-muted-foreground">No photos uploaded yet.</div>
          )}
        </div>
      )}

      {/* Upload Modal */}
      {showUpload && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-foreground/40">
          <div className="bg-card rounded-2xl shadow-xl w-full max-w-lg p-6">
            <div className="flex items-center justify-between mb-6">
              <h2 className="text-section-title">Upload Photo</h2>
              <Button variant="ghost" size="icon" onClick={() => setShowUpload(false)}><Plus className="h-4 w-4 rotate-45" /></Button>
            </div>
            <div
              className="border-2 border-dashed border-border rounded-xl p-10 text-center mb-4 cursor-pointer hover:border-primary transition-colors"
              onClick={() => fileRef.current?.click()}
            >
              <CloudUpload className="h-10 w-10 text-muted-foreground mx-auto mb-3" />
              {uploadFile
                ? <p className="text-sm text-foreground font-medium">{uploadFile.name}</p>
                : <>
                  <p className="text-sm text-foreground font-medium">Drag and drop photos here</p>
                  <p className="text-caption mt-1">JPG, PNG, WebP · Max 10MB per photo</p>
                </>
              }
              <input
                ref={fileRef}
                type="file"
                accept="image/*"
                className="hidden"
                onChange={e => setUploadFile(e.target.files?.[0] ?? null)}
              />
            </div>
            <div className="mb-4">
              <label className="text-sm font-medium text-foreground">Title *</label>
              <Input className="mt-1" placeholder="e.g. Rose Pattern Product" value={uploadTitle} onChange={e => setUploadTitle(e.target.value)} />
            </div>
            <div className="mb-4">
              <label className="text-sm font-medium text-foreground">Category</label>
              <select
                className="mt-1 w-full rounded-md border border-border bg-background px-3 py-2 text-sm text-foreground"
                value={uploadCategory}
                onChange={e => setUploadCategory(e.target.value)}
              >
                {categories.length === 0 && <option value="">No categories</option>}
                {categories.map(c => (
                  <option key={c.id} value={c.name}>{CATEGORY_LABELS[c.name] ?? c.name}</option>
                ))}
              </select>

              {/* Inline category management */}
              <div className="mt-3 rounded-lg border border-border bg-muted/30 p-3">
                <p className="text-xs font-medium text-muted-foreground mb-2">Manage categories</p>
                <div className="flex flex-wrap gap-1.5 mb-2">
                  {categories.map((c: PhotoCategoryItem) => (
                    <span key={c.id} className="inline-flex items-center gap-1 text-xs bg-card border border-border rounded px-2 py-1">
                      {CATEGORY_LABELS[c.name] ?? c.name}
                      <button
                        type="button"
                        className="text-muted-foreground hover:text-danger disabled:opacity-50"
                        disabled={deleteCategoryMut.isPending}
                        onClick={() => deleteCategoryMut.mutate(c.id)}
                        aria-label={`Delete ${c.name}`}
                      >
                        <X className="h-3 w-3" />
                      </button>
                    </span>
                  ))}
                  {categories.length === 0 && (
                    <span className="text-xs text-muted-foreground">No categories yet.</span>
                  )}
                </div>
                <div className="flex gap-2">
                  <Input
                    className="h-8 text-sm"
                    placeholder="New category name"
                    value={newCategory}
                    onChange={e => setNewCategory(e.target.value)}
                    onKeyDown={e => { if (e.key === "Enter") { e.preventDefault(); handleAddCategory(); } }}
                  />
                  <Button
                    type="button"
                    variant="secondary"
                    className="h-8"
                    disabled={createCategoryMut.isPending}
                    onClick={handleAddCategory}
                  >
                    Add
                  </Button>
                </div>
              </div>
            </div>
            <div className="flex justify-end gap-2 mt-4">
              <Button variant="ghost" onClick={() => setShowUpload(false)}>Cancel</Button>
              <Button onClick={handleUpload} disabled={createMut.isPending}>
                {createMut.isPending ? "Uploading..." : "Upload Photo"}
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
