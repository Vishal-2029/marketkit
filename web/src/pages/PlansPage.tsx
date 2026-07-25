import { useEffect, useRef, useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import DOMPurify from "dompurify";
import { PageHeader } from "@/components/PageHeader";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Check, X, Plus, Trash2, Pencil, Bold, Italic, Underline, List, ListOrdered } from "lucide-react";
import { toast } from "sonner";
import { plansService, PlanPayload } from "@/services/plans";
import { Skeleton } from "@/components/ui/skeleton";

// Plan descriptions only ever need the rich-text editor's own output
// (bold/italic/underline/lists) — no attributes are needed for any of that,
// so stripping all of them closes off onerror/onload/style-based XSS vectors
// entirely rather than trying to allowlist "safe" attributes.
const sanitizeDescription = (html: string) =>
  DOMPurify.sanitize(html, {
    ALLOWED_TAGS: ["b", "strong", "i", "em", "u", "ul", "ol", "li", "br", "p", "div", "span"],
    ALLOWED_ATTR: [],
  });

interface Plan {
  id: string;
  name: string;
  description: string;
  price_in_paise: number;
  has_willcom: boolean;
  has_e4: boolean;
  has_mecad: boolean;
  duration_days: number;
  is_active: boolean;
  subscribers: number;
}

const emptyForm = (): PlanPayload => ({
  name: "",
  description: "",
  price_in_paise: 0,
  has_willcom: false,
  has_e4: false,
  has_mecad: false,
  duration_days: 365,
  is_active: true,
});

export default function PlansPage() {
  const qc = useQueryClient();

  // modal state: null = closed, "create" = new plan, Plan = editing existing
  const [modal, setModal] = useState<null | "create" | Plan>(null);
  const [form, setForm] = useState<PlanPayload>(emptyForm());
  const [priceInput, setPriceInput] = useState("");
  const [deleteTarget, setDeleteTarget] = useState<Plan | null>(null);
  const [showInactive, setShowInactive] = useState(false);
  const [activeFormats, setActiveFormats] = useState({
    bold: false,
    italic: false,
    underline: false,
    unorderedList: false,
    orderedList: false,
  });
  const descriptionRef = useRef<HTMLDivElement>(null);
  const selectionRangeRef = useRef<Range | null>(null);

  const updateToolbarState = () => {
    setActiveFormats({
      bold: document.queryCommandState("bold"),
      italic: document.queryCommandState("italic"),
      underline: document.queryCommandState("underline"),
      unorderedList: document.queryCommandState("insertUnorderedList"),
      orderedList: document.queryCommandState("insertOrderedList"),
    });
  };

  const toolbarButtonClass = (active: boolean) =>
    `inline-flex items-center justify-center rounded-md px-2 py-1 transition-colors ${
      active
        ? "bg-muted text-foreground border border-border"
        : "text-foreground hover:bg-muted"
    }`;

  const applyDescriptionFormat = (command: string, value?: string) => {
    descriptionRef.current?.focus();

    const sel = window.getSelection?.();
    if (sel && selectionRangeRef.current) {
      sel.removeAllRanges();
      sel.addRange(selectionRangeRef.current);
    }

    document.execCommand(command, false, value ?? "");
    syncDescriptionFromRef();

    setTimeout(() => {
      updateToolbarState();
    }, 0);
  };

  const syncDescriptionFromRef = () => {
    setForm(current => ({ ...current, description: descriptionRef.current?.innerHTML ?? "" }));
  };

  useEffect(() => {
    if (modal !== null && descriptionRef.current) {
      descriptionRef.current.innerHTML = sanitizeDescription(form.description ?? "");
      updateToolbarState();
    }
  }, [modal]);

  useEffect(() => {
    const onSelectionChange = () => {
      const sel = window.getSelection && window.getSelection();
      if (!sel || sel.rangeCount === 0) return;

      const range = sel.getRangeAt(0);
      if (!descriptionRef.current) return;
      if (
        descriptionRef.current.contains(range.startContainer) ||
        descriptionRef.current === range.startContainer
      ) {
        selectionRangeRef.current = range.cloneRange();
        updateToolbarState();
      }
    };

    document.addEventListener("selectionchange", onSelectionChange);
    return () => document.removeEventListener("selectionchange", onSelectionChange);
  }, []);

  const { data: plans = [], isLoading } = useQuery({
    queryKey: ["plans"],
    queryFn: plansService.list,
  });

  const createMut = useMutation({
    mutationFn: (data: PlanPayload) => plansService.create(data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["plans"] });
      setModal(null);
      toast.success(`Plan "${form.name}" created.`);
    },
    onError: (e: any) => toast.error(e?.response?.data?.error ?? "Failed to create plan."),
  });

  const updateMut = useMutation({
    mutationFn: ({ id, data }: { id: string; data: PlanPayload }) =>
      plansService.update(id, data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["plans"] });
      setModal(null);
      toast.success("Plan updated.");
    },
    onError: () => toast.error("Failed to update plan."),
  });

  const deleteMut = useMutation({
    mutationFn: (id: string) => plansService.delete(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["plans"] });
      setDeleteTarget(null);
      toast.success("Plan deleted.");
    },
    onError: (e: any) =>
      toast.error(e?.response?.data?.error ?? "Failed to delete plan."),
  });

  const toggleActive = (plan: Plan) => {
    updateMut.mutate({ id: plan.id, data: { is_active: !plan.is_active } });
    toast.success(`"${plan.name}" ${plan.is_active ? "deactivated" : "activated"}.`);
  };

  const openCreate = () => {
    setForm(emptyForm());
    setPriceInput("");
    setModal("create");
  };

  const openEdit = (plan: Plan) => {
    setForm({
      name: plan.name,
      description: plan.description ?? "",
      price_in_paise: plan.price_in_paise,
      has_willcom: plan.has_willcom,
      has_e4: plan.has_e4,
      has_mecad: plan.has_mecad,
      duration_days: plan.duration_days,
      is_active: plan.is_active,
    });
    setPriceInput(String(plan.price_in_paise / 100));
    setModal(plan);
  };

  const handleSave = () => {
    const raw = priceInput.replace(/,/g, "");
    if (!/^\d+$/.test(raw) || Number(raw) <= 0) {
      toast.error("Please enter a valid price.");
      return;
    }
    if (!form.name?.trim()) {
      toast.error("Plan name is required.");
      return;
    }

    const payload: PlanPayload = {
      ...form,
      price_in_paise: Number(raw) * 100,
    };

    if (modal === "create") {
      createMut.mutate(payload);
    } else {
      updateMut.mutate({ id: (modal as Plan).id, data: payload });
    }
  };

  const isPending = createMut.isPending || updateMut.isPending;

  const visiblePlans = (plans as Plan[]).filter(p => showInactive || p.is_active);

  return (
    <div>
      <PageHeader title="Plans" subtitle="Manage subscription plan pricing and availability">
        <label className="flex items-center gap-2 cursor-pointer select-none">
          <button
            type="button"
            className={`w-10 h-6 rounded-full relative transition-colors duration-200 ${showInactive ? "bg-primary" : "bg-muted"}`}
            onClick={() => setShowInactive(v => !v)}
          >
            <span className={`absolute top-1 left-1 w-4 h-4 rounded-full bg-white shadow transition-all duration-200 ${showInactive ? "translate-x-4" : "translate-x-0"}`} />
          </button>
          <span className="text-sm text-muted-foreground">Show inactive</span>
        </label>
        <Button onClick={openCreate}><Plus className="h-4 w-4" /> New Plan</Button>
      </PageHeader>

      {isLoading ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5">
          {Array(3).fill(0).map((_, i) => <Skeleton key={i} className="h-64 rounded-2xl" />)}
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5">
          {visiblePlans.map(plan => {
            const revenueYearly = (plan.price_in_paise / 100) * plan.subscribers;
            return (
              <div key={plan.id} className="rounded-2xl border border-border bg-card p-6 flex flex-col">
                {/* Header */}
                <div className="flex items-start justify-between mb-1">
                  <div className="flex items-center gap-2">
                    <h2 className="text-lg font-semibold text-foreground">{plan.name}</h2>
                    {!plan.is_active && (
                      <span className="text-[10px] font-semibold px-1.5 py-0.5 rounded bg-muted text-muted-foreground">
                        Inactive
                      </span>
                    )}
                  </div>
                  <button
                    className={`w-10 h-6 rounded-full relative cursor-pointer transition-colors duration-200 shrink-0 ${plan.is_active ? "bg-primary" : "bg-muted"}`}
                    onClick={() => toggleActive(plan)}
                    title={plan.is_active ? "Deactivate" : "Activate"}
                  >
                    <span className={`absolute top-1 left-1 w-4 h-4 rounded-full bg-white shadow transition-all duration-200 ${plan.is_active ? "translate-x-4" : "translate-x-0"}`} />
                  </button>
                </div>

                {/* Duration */}
                <p className="text-caption mb-3">{plan.duration_days} days</p>

                {/* Price */}
                <p className="text-2xl font-bold text-primary mb-1">
                  ₹{(plan.price_in_paise / 100).toLocaleString("en-IN")}
                  <span className="text-sm font-normal text-muted-foreground"> / year</span>
                </p>

                {/* Revenue */}
                {plan.subscribers > 0 && (
                  <p className="text-xs text-success font-medium mb-2">
                    ₹{revenueYearly.toLocaleString("en-IN")} yearly revenue
                  </p>
                )}

                {/* Description */}
                {plan.description && (
                  <div className="text-sm text-muted-foreground mt-1 mb-3 leading-snug [&_ul]:list-disc [&_ul]:pl-6 [&_ol]:list-decimal [&_ol]:pl-6 [&_li]:my-1" dangerouslySetInnerHTML={{ __html: sanitizeDescription(plan.description) }} />
                )}

                {/* Features — only show included categories */}
                {[
                  { label: "Wilcom 2006 Videos", included: plan.has_willcom },
                  { label: "E4 Videos",      included: plan.has_e4 },
                  { label: "meCAD Videos",   included: plan.has_mecad },
                ].filter(f => f.included).length > 0 && (
                  <div className="space-y-1.5 mb-4 mt-2">
                    {[
                      { label: "Wilcom 2006 Videos", included: plan.has_willcom },
                      { label: "E4 Videos",      included: plan.has_e4 },
                      { label: "meCAD Videos",   included: plan.has_mecad },
                    ].filter(f => f.included).map(f => (
                      <div key={f.label} className="flex items-center gap-2">
                        <Check className="h-4 w-4 text-success shrink-0" />
                        <span className="text-sm text-foreground">{f.label}</span>
                      </div>
                    ))}
                  </div>
                )}

                {/* Subscribers */}
                <p className="text-caption mb-4">{plan.subscribers} active subscriber{plan.subscribers !== 1 ? "s" : ""}</p>

                {/* Actions */}
                <div className="flex gap-2 mt-auto">
                  <Button variant="outline" size="sm" className="flex-1" onClick={() => openEdit(plan)}>
                    <Pencil className="h-3.5 w-3.5 mr-1" /> Edit
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    className="text-danger border-danger/30 hover:bg-danger/5"
                    onClick={() => setDeleteTarget(plan)}
                    disabled={plan.subscribers > 0}
                    title={plan.subscribers > 0 ? "Cannot delete — has active subscribers" : "Delete plan"}
                  >
                    <Trash2 className="h-3.5 w-3.5" />
                  </Button>
                </div>
              </div>
            );
          })}

          {visiblePlans.length === 0 && (
            <div className="col-span-full py-16 text-center text-sm text-muted-foreground">
              {showInactive ? 'No plans yet. Click "New Plan" to create one.' : "No active plans. Enable \"Show inactive\" to see all plans."}
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
                {modal === "create" ? "New Plan" : "Edit Plan"}
              </h2>
              <Button variant="ghost" size="icon" onClick={() => setModal(null)}>
                <Plus className="h-4 w-4 rotate-45" />
              </Button>
            </div>

            <div className="space-y-4">
              {/* Name */}
              <div>
                <label className="text-sm font-medium text-foreground">Plan Name *</label>
                <Input
                  className="mt-1"
                  placeholder="e.g. Wilcom 2006 Basic"
                  value={form.name ?? ""}
                  onChange={e => setForm(f => ({ ...f, name: e.target.value }))}
                />
              </div>

              {/* Description */}
              <div>
                <label className="text-sm font-medium text-foreground">Description</label>
                <div className="mt-2 rounded-xl border border-border bg-background">
                  <div className="flex flex-wrap gap-1 border-b border-border px-2 py-2 bg-muted/50 rounded-t-xl">
                    <button
                      type="button"
                      className={toolbarButtonClass(activeFormats.bold)}
                      onMouseDown={(e) => { e.preventDefault(); applyDescriptionFormat("bold"); }}
                    >
                      <Bold className="h-4 w-4" />
                    </button>
                    <button
                      type="button"
                      className={toolbarButtonClass(activeFormats.italic)}
                      onMouseDown={(e) => { e.preventDefault(); applyDescriptionFormat("italic"); }}
                    >
                      <Italic className="h-4 w-4" />
                    </button>
                    <button
                      type="button"
                      className={toolbarButtonClass(activeFormats.underline)}
                      onMouseDown={(e) => { e.preventDefault(); applyDescriptionFormat("underline"); }}
                    >
                      <Underline className="h-4 w-4" />
                    </button>
                    <button
                      type="button"
                      className={toolbarButtonClass(activeFormats.unorderedList)}
                      onMouseDown={(e) => { e.preventDefault(); applyDescriptionFormat("insertUnorderedList"); }}
                    >
                      <List className="h-4 w-4" />
                    </button>
                    <button
                      type="button"
                      className={toolbarButtonClass(activeFormats.orderedList)}
                      onMouseDown={(e) => { e.preventDefault(); applyDescriptionFormat("insertOrderedList"); }}
                    >
                      <ListOrdered className="h-4 w-4" />
                    </button>
                  </div>
                  <div className="relative">
                    <div
                      ref={descriptionRef}
                      contentEditable
                      suppressContentEditableWarning
                      dir="ltr"
                      style={{ minHeight: "140px" }}
                      className="w-full rounded-b-xl bg-background px-3 py-3 text-sm text-foreground outline-none focus:ring-2 focus:ring-primary/50 [&_ul]:list-disc [&_ul]:pl-6 [&_ol]:list-decimal [&_ol]:pl-6 [&_li]:my-1"
                      onInput={syncDescriptionFromRef}
                    />
                    {!form.description && (
                      <div className="pointer-events-none absolute inset-0 px-3 py-3 text-sm text-muted-foreground">
                        Briefly describe this plan. Use bold, lists, and formatting options.
                      </div>
                    )}
                  </div>
                </div>
              </div>

              {/* Price */}
              <div>
                <label className="text-sm font-medium text-foreground">Yearly Price (₹) *</label>
                <Input
                  className="mt-1"
                  placeholder="e.g. 1499"
                  value={priceInput}
                  onChange={e => setPriceInput(e.target.value)}
                />
              </div>

              {/* Duration */}
              <div>
                <label className="text-sm font-medium text-foreground">Duration (days)</label>
                <Input
                  className="mt-1"
                  type="number"
                  min={1}
                  placeholder="365"
                  value={form.duration_days ?? 365}
                  onChange={e => setForm(f => ({ ...f, duration_days: Number(e.target.value) }))}
                />
              </div>

              {/* Features */}
              <div>
                <label className="text-sm font-medium text-foreground mb-2 block">Included Categories</label>
                <div className="space-y-2">
                  {([
                    { key: "has_willcom", label: "Wilcom 2006 Videos" },
                    { key: "has_e4",      label: "E4 Videos" },
                    { key: "has_mecad",   label: "meCAD Videos" },
                  ] as { key: keyof PlanPayload; label: string }[]).map(({ key, label }) => (
                    <label key={key} className="flex items-center gap-3 cursor-pointer">
                      <button
                        type="button"
                        className={`w-10 h-6 rounded-full relative transition-colors duration-200 ${form[key] ? "bg-primary" : "bg-muted"}`}
                        onClick={() => setForm(f => ({ ...f, [key]: !f[key] }))}
                      >
                        <span className={`absolute top-1 left-1 w-4 h-4 rounded-full bg-white shadow transition-all duration-200 ${form[key] ? "translate-x-4" : "translate-x-0"}`} />
                      </button>
                      <span className="text-sm text-foreground">{label}</span>
                    </label>
                  ))}
                </div>
              </div>

              {/* Active toggle (edit only) */}
              {modal !== "create" && (
                <div className="flex items-center justify-between py-1">
                  <p className="text-sm font-medium text-foreground">Active</p>
                  <button
                    type="button"
                    className={`w-10 h-6 rounded-full relative transition-colors duration-200 ${form.is_active ? "bg-primary" : "bg-muted"}`}
                    onClick={() => setForm(f => ({ ...f, is_active: !f.is_active }))}
                  >
                    <span className={`absolute top-1 left-1 w-4 h-4 rounded-full bg-white shadow transition-all duration-200 ${form.is_active ? "translate-x-4" : "translate-x-0"}`} />
                  </button>
                </div>
              )}
            </div>

            <div className="flex justify-end gap-2 mt-6">
              <Button variant="ghost" onClick={() => setModal(null)}>Cancel</Button>
              <Button onClick={handleSave} disabled={isPending}>
                {isPending ? "Saving..." : modal === "create" ? "Create Plan" : "Save Changes"}
              </Button>
            </div>
          </div>
        </div>
      )}

      {/* Delete Confirmation */}
      {deleteTarget && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-foreground/40">
          <div className="bg-card rounded-2xl shadow-xl w-full max-w-sm p-6">
            <h2 className="text-section-title mb-2">Delete Plan</h2>
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
