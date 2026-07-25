import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Plus, Pencil, Trash2, Lock } from "lucide-react";
import { toast } from "sonner";
import { PageHeader } from "@/components/PageHeader";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { designCategoriesService, DesignCategory, DesignCategoryPayload } from "@/services/designCategories";
import { designsService } from "@/services/designs";

const formatPrice = (paise: number) => `₹${(paise / 100).toLocaleString("en-IN")}`;

const emptyForm = (): DesignCategoryPayload => ({ name: "", display_order: 0 });

function SectionRow({
  category,
  extraCount,
  selected,
  onSelect,
  onEdit,
  onDelete,
  deleteDisabledReason,
}: {
  category: DesignCategory;
  extraCount?: string;
  selected: boolean;
  onSelect: () => void;
  onEdit: () => void;
  onDelete: () => void;
  deleteDisabledReason: string | null;
}) {
  return (
    <div
      onClick={onSelect}
      className={`flex items-center justify-between gap-2 rounded-xl border px-3 py-2.5 cursor-pointer transition-colors ${
        selected ? "border-primary bg-primary/5" : "border-border hover:bg-muted/40"
      }`}
    >
      <div className="flex items-center gap-2.5 min-w-0">
        {category.photo_url ? (
          <img
            src={category.photo_url}
            alt=""
            className="h-9 w-9 rounded-lg object-cover border border-border shrink-0"
          />
        ) : (
          <div className="h-9 w-9 rounded-lg bg-muted shrink-0" />
        )}
        <div className="min-w-0">
          <p className="text-sm font-medium text-foreground truncate">{category.name}</p>
          <p className="text-xs text-muted-foreground mt-0.5">
            {category.design_count} design{category.design_count !== 1 ? "s" : ""}
            {extraCount ? ` · ${extraCount}` : ""}
          </p>
        </div>
      </div>
      <div className="flex items-center gap-1 shrink-0">
        <Button
          variant="ghost"
          size="icon"
          className="h-7 w-7"
          onClick={(e) => {
            e.stopPropagation();
            onEdit();
          }}
        >
          <Pencil className="h-3.5 w-3.5" />
        </Button>
        <Button
          variant="ghost"
          size="icon"
          className="h-7 w-7 text-danger hover:bg-danger/5"
          disabled={!!deleteDisabledReason}
          title={deleteDisabledReason ?? "Delete"}
          onClick={(e) => {
            e.stopPropagation();
            onDelete();
          }}
        >
          <Trash2 className="h-3.5 w-3.5" />
        </Button>
      </div>
    </div>
  );
}

export default function CategoriesPage() {
  const qc = useQueryClient();

  const [selectedParentId, setSelectedParentId] = useState<string | null>(null);
  const [selectedChildId, setSelectedChildId] = useState<string | null>(null);
  const [modal, setModal] = useState<null | "create-parent" | "create-child" | DesignCategory>(null);
  const [form, setForm] = useState<DesignCategoryPayload>(emptyForm());
  const [deleteTarget, setDeleteTarget] = useState<DesignCategory | null>(null);
  const [photoFile, setPhotoFile] = useState<File | null>(null);
  const [photoPreview, setPhotoPreview] = useState<string | null>(null);

  const { data: categories = [], isLoading } = useQuery({
    queryKey: ["design-categories"],
    queryFn: designCategoriesService.list,
  });

  const parents = categories.filter((c) => !c.parent_id && !c.is_other);
  const otherCategory = categories.find((c) => c.is_other);
  const children = categories.filter((c) => c.parent_id === selectedParentId);
  const selectedParent = parents.find((p) => p.id === selectedParentId) ?? null;
  const selectedChild = children.find((c) => c.id === selectedChildId) ?? null;

  const { data: designsData, isLoading: designsLoading } = useQuery({
    queryKey: ["market-designs-by-category", selectedChildId],
    queryFn: () => designsService.listDesigns(1, "", selectedChildId!),
    enabled: !!selectedChildId,
  });

  const invalidate = () => qc.invalidateQueries({ queryKey: ["design-categories"] });

  const createMut = useMutation({
    mutationFn: async (data: DesignCategoryPayload) => {
      const created = await designCategoriesService.create(data);
      return photoFile ? designCategoriesService.uploadPhoto(created.id, photoFile) : created;
    },
    onSuccess: () => {
      invalidate();
      setModal(null);
      toast.success(`Section "${form.name}" created.`);
    },
    onError: (e: any) => toast.error(e?.response?.data?.error ?? "Failed to create section."),
  });

  const updateMut = useMutation({
    mutationFn: async ({ id, data }: { id: string; data: DesignCategoryPayload }) => {
      const updated = await designCategoriesService.update(id, data);
      return photoFile ? designCategoriesService.uploadPhoto(id, photoFile) : updated;
    },
    onSuccess: () => {
      invalidate();
      setModal(null);
      toast.success("Section updated.");
    },
    onError: (e: any) => toast.error(e?.response?.data?.error ?? "Failed to update section."),
  });

  const deleteMut = useMutation({
    mutationFn: (id: string) => designCategoriesService.delete(id),
    onSuccess: (_data, id) => {
      invalidate();
      setDeleteTarget(null);
      if (selectedChildId === id) setSelectedChildId(null);
      if (selectedParentId === id) {
        setSelectedParentId(null);
        setSelectedChildId(null);
      }
      toast.success("Section deleted.");
    },
    onError: (e: any) => toast.error(e?.response?.data?.error ?? "Failed to delete section."),
  });

  const resetPhoto = () => {
    setPhotoFile(null);
    setPhotoPreview((prev) => {
      if (prev) URL.revokeObjectURL(prev);
      return null;
    });
  };

  const openCreateParent = () => {
    setForm(emptyForm());
    resetPhoto();
    setModal("create-parent");
  };

  const openCreateChild = () => {
    if (!selectedParentId) return;
    setForm(emptyForm());
    resetPhoto();
    setModal("create-child");
  };

  const openEdit = (cat: DesignCategory) => {
    setForm({ name: cat.name, display_order: cat.display_order });
    resetPhoto();
    setModal(cat);
  };

  const handlePhotoPick = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    e.target.value = "";
    if (!file) return;
    setPhotoFile(file);
    setPhotoPreview((prev) => {
      if (prev) URL.revokeObjectURL(prev);
      return URL.createObjectURL(file);
    });
  };

  const handleSave = () => {
    if (!form.name?.trim()) {
      toast.error("Section name is required.");
      return;
    }
    if (modal === "create-parent") {
      createMut.mutate({ name: form.name, display_order: form.display_order ?? 0, parent_id: null });
    } else if (modal === "create-child") {
      createMut.mutate({ name: form.name, display_order: form.display_order ?? 0, parent_id: selectedParentId });
    } else if (modal) {
      updateMut.mutate({ id: modal.id, data: { name: form.name, display_order: form.display_order ?? 0 } });
    }
  };

  const isPending = createMut.isPending || updateMut.isPending;

  const deleteReason = (cat: DesignCategory) => {
    if (cat.design_count > 0) return "Cannot delete — designs are assigned to this section";
    if (!cat.parent_id) {
      const childCount = categories.filter((c) => c.parent_id === cat.id).length;
      if (childCount > 0) return "Cannot delete — remove its sub-sections first";
    }
    return null;
  };

  const designs = designsData?.data ?? [];

  return (
    <div>
      <PageHeader title="Design Market Sections" subtitle="Manage parent and child sections, and browse designs by section" />

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
        {/* Parent sections */}
        <div className="rounded-2xl border border-border bg-card p-4">
          <div className="flex items-center justify-between mb-3">
            <h2 className="text-sm font-semibold text-foreground">Parent Sections</h2>
            <Button size="sm" onClick={openCreateParent}>
              <Plus className="h-3.5 w-3.5" /> New
            </Button>
          </div>

          {isLoading ? (
            <div className="space-y-2">
              {Array(4).fill(0).map((_, i) => <Skeleton key={i} className="h-14 rounded-xl" />)}
            </div>
          ) : (
            <div className="space-y-1.5">
              {parents.map((p) => {
                const childCount = categories.filter((c) => c.parent_id === p.id).length;
                return (
                  <SectionRow
                    key={p.id}
                    category={p}
                    extraCount={`${childCount} sub-section${childCount !== 1 ? "s" : ""}`}
                    selected={p.id === selectedParentId}
                    onSelect={() => {
                      setSelectedParentId(p.id);
                      setSelectedChildId(null);
                    }}
                    onEdit={() => openEdit(p)}
                    onDelete={() => setDeleteTarget(p)}
                    deleteDisabledReason={deleteReason(p)}
                  />
                );
              })}
              {parents.length === 0 && (
                <p className="text-sm text-muted-foreground py-8 text-center">
                  No parent sections yet. Click "New" to create one.
                </p>
              )}
            </div>
          )}

          {otherCategory && (
            <div className="mt-3 pt-3 border-t border-border flex items-center gap-2 text-xs text-muted-foreground">
              <Lock className="h-3 w-3 shrink-0" />
              <span>
                "Other" (system) — {otherCategory.design_count} design{otherCategory.design_count !== 1 ? "s" : ""}, protected
              </span>
            </div>
          )}
        </div>

        {/* Child sections */}
        <div className="rounded-2xl border border-border bg-card p-4">
          <div className="flex items-center justify-between mb-3">
            <h2 className="text-sm font-semibold text-foreground truncate">
              {selectedParent ? `Sub-sections of "${selectedParent.name}"` : "Sub-sections"}
            </h2>
            <Button size="sm" onClick={openCreateChild} disabled={!selectedParentId}>
              <Plus className="h-3.5 w-3.5" /> New
            </Button>
          </div>

          {!selectedParentId ? (
            <p className="text-sm text-muted-foreground py-8 text-center">
              Select a parent section to view its sub-sections.
            </p>
          ) : (
            <div className="space-y-1.5">
              {children.map((c) => (
                <SectionRow
                  key={c.id}
                  category={c}
                  selected={c.id === selectedChildId}
                  onSelect={() => setSelectedChildId(c.id)}
                  onEdit={() => openEdit(c)}
                  onDelete={() => setDeleteTarget(c)}
                  deleteDisabledReason={deleteReason(c)}
                />
              ))}
              {children.length === 0 && (
                <p className="text-sm text-muted-foreground py-8 text-center">
                  No sub-sections yet. Click "New" to create one.
                </p>
              )}
            </div>
          )}
        </div>
      </div>

      {/* Designs in selected child section */}
      {selectedChildId && (
        <div className="mt-8">
          <h2 className="text-section-title mb-4">
            Designs in "{selectedChild?.name ?? ""}"
          </h2>
          {designsLoading ? (
            <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-4">
              {Array(6).fill(0).map((_, i) => <Skeleton key={i} className="aspect-square rounded-xl" />)}
            </div>
          ) : designs.length === 0 ? (
            <p className="text-sm text-muted-foreground py-8 text-center border border-dashed border-border rounded-xl">
              No designs have been uploaded into this section yet.
            </p>
          ) : (
            <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-4">
              {designs.map((d) => (
                <a
                  key={d.id}
                  href={d.preview_urls?.[0] ?? undefined}
                  target="_blank"
                  rel="noreferrer"
                  className="rounded-xl border border-border bg-card overflow-hidden hover:opacity-90 transition-opacity"
                >
                  {d.preview_urls?.length > 0 ? (
                    <img src={d.preview_urls[0]} alt={d.title} className="w-full aspect-square object-cover" />
                  ) : (
                    <div className="w-full aspect-square bg-muted flex items-center justify-center text-xs text-muted-foreground">
                      No preview
                    </div>
                  )}
                  <div className="p-2">
                    <p className="text-xs font-medium text-foreground truncate">{d.title}</p>
                    <p className="text-xs text-muted-foreground">{formatPrice(d.price_in_paise)}</p>
                  </div>
                </a>
              ))}
            </div>
          )}
        </div>
      )}

      {/* Create / Edit Modal */}
      {modal !== null && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-foreground/40">
          <div className="bg-card rounded-2xl shadow-xl w-full max-w-md p-6">
            <div className="flex items-center justify-between mb-6">
              <h2 className="text-section-title">
                {modal === "create-parent"
                  ? "New Parent Section"
                  : modal === "create-child"
                    ? `New Sub-section${selectedParent ? ` of "${selectedParent.name}"` : ""}`
                    : "Edit Section"}
              </h2>
              <Button variant="ghost" size="icon" onClick={() => setModal(null)}>
                <Plus className="h-4 w-4 rotate-45" />
              </Button>
            </div>

            <div className="space-y-4">
              <div>
                <label className="text-sm font-medium text-foreground">Name *</label>
                <Input
                  className="mt-1"
                  placeholder="e.g. Bridal Collection"
                  value={form.name ?? ""}
                  onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
                />
              </div>
              <div>
                <label className="text-sm font-medium text-foreground">Display Order</label>
                <Input
                  className="mt-1"
                  type="number"
                  min={0}
                  value={form.display_order ?? 0}
                  onChange={(e) => setForm((f) => ({ ...f, display_order: Number(e.target.value) }))}
                />
                <p className="text-xs text-muted-foreground mt-1">Lower numbers appear first.</p>
              </div>
              <div>
                <label className="text-sm font-medium text-foreground">Section Photo</label>
                <p className="text-xs text-muted-foreground mt-0.5 mb-2">
                  Shown as this section's banner on the Design Market home page. If not set, the app falls back to the first design uploaded into it.
                </p>
                <div className="flex items-center gap-3">
                  {photoPreview || (typeof modal === "object" && modal?.photo_url) ? (
                    <img
                      src={photoPreview ?? (modal as DesignCategory).photo_url}
                      alt=""
                      className="h-14 w-14 rounded-lg object-cover border border-border"
                    />
                  ) : (
                    <div className="h-14 w-14 rounded-lg bg-muted" />
                  )}
                  <label className="cursor-pointer">
                    <span className="inline-flex items-center px-3 py-1.5 rounded-lg border border-border text-sm font-medium text-foreground hover:bg-muted/40 transition-colors">
                      {photoFile ? "Change photo" : "Upload photo"}
                    </span>
                    <input type="file" accept="image/*" className="hidden" onChange={handlePhotoPick} />
                  </label>
                </div>
              </div>
            </div>

            <div className="flex justify-end gap-2 mt-6">
              <Button variant="ghost" onClick={() => { resetPhoto(); setModal(null); }}>Cancel</Button>
              <Button onClick={handleSave} disabled={isPending}>
                {isPending ? "Saving..." : modal === "create-parent" || modal === "create-child" ? "Create" : "Save Changes"}
              </Button>
            </div>
          </div>
        </div>
      )}

      {/* Delete confirmation */}
      {deleteTarget && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-foreground/40">
          <div className="bg-card rounded-2xl shadow-xl w-full max-w-sm p-6">
            <h2 className="text-section-title mb-2">Delete Section</h2>
            <p className="text-sm text-muted-foreground mb-6">
              Are you sure you want to delete <span className="font-medium text-foreground">"{deleteTarget.name}"</span>? This cannot be undone.
            </p>
            <div className="flex justify-end gap-2">
              <Button variant="ghost" onClick={() => setDeleteTarget(null)}>Cancel</Button>
              <Button
                className="bg-danger text-white hover:bg-danger/90"
                onClick={() => deleteMut.mutate(deleteTarget.id)}
                disabled={deleteMut.isPending}
              >
                {deleteMut.isPending ? "Deleting..." : "Delete"}
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
