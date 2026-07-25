import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { PageHeader } from "@/components/PageHeader";
import { StatusBadge } from "@/components/StatusBadge";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Switch } from "@/components/ui/switch";
import { Search } from "lucide-react";
import { adminsService, AdminRecord } from "@/services/admins";
import { useAuth } from "@/contexts/AuthContext";

const PAGE_SIZE = 20;

export default function AdminsPage() {
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(1);
  const [showCreateDialog, setShowCreateDialog] = useState(false);
  const [firstName, setFirstName] = useState("");
  const [lastName, setLastName] = useState("");
  const [phone, setPhone] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [isSuper, setIsSuper] = useState(false);

  const { data, isLoading } = useQuery({
    queryKey: ["admins", page, search],
    queryFn: () => adminsService.list({ page, limit: PAGE_SIZE, search: search || undefined }),
  });

  const admins: AdminRecord[] = data?.data ?? [];
  const meta = data?.meta;
  const totalPages = meta ? Math.max(1, Math.ceil(meta.total / PAGE_SIZE)) : 1;
  const qc = useQueryClient();
  const { admin: currentAdmin } = useAuth();

  const deleteMut = useMutation({
    mutationFn: (id: string) => adminsService.delete(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["admins"] }),
  });

  const setSuperMut = useMutation({
    mutationFn: ({ id, isSuper }: { id: string; isSuper: boolean }) => adminsService.setSuper(id, isSuper),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["admins"] }),
  });

  const createMut = useMutation({
    mutationFn: () => adminsService.create({
      first_name: firstName,
      last_name: lastName,
      phone: phone || undefined,
      email,
      password,
      is_super: isSuper,
    }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["admins"] });
      setShowCreateDialog(false);
      setFirstName("");
      setLastName("");
      setPhone("");
      setEmail("");
      setPassword("");
      setIsSuper(false);
    },
  });

  const handleCreateAdmin = () => {
    createMut.mutate();
  };

  return (
    <div>
      <PageHeader title="Admins" subtitle="View and manage admin accounts" />

      <div className="flex flex-col gap-4 mb-6 md:flex-row md:items-center md:justify-between">
        <div className="relative max-w-md w-full">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <Input
            placeholder="Search admins by name or email..."
            className="pl-9 bg-muted border-0"
            value={search}
            onChange={(e) => { setSearch(e.target.value); setPage(1); }}
          />
        </div>
        <div className="flex flex-col items-start gap-2 sm:items-end">
          <div className="text-sm text-muted-foreground">
            {meta ? `${meta.total} admin${meta.total === 1 ? "" : "s"}` : "Loading admins..."}
          </div>
          {currentAdmin?.is_super && (
            <Button onClick={() => setShowCreateDialog(true)}>Create Admin</Button>
          )}
        </div>
      </div>

      <div className="overflow-hidden rounded-xl border border-border bg-card">
        <table className="w-full text-sm">
          <thead>
            <tr className="bg-table-header">
              <th className="text-table-header text-left px-4 py-3">Name</th>
              <th className="text-table-header text-left px-4 py-3 hidden sm:table-cell">Email</th>
              <th className="text-table-header text-left px-4 py-3 hidden md:table-cell">Phone</th>
              <th className="text-table-header text-left px-4 py-3">Status</th>
              <th className="text-table-header text-right px-4 py-3 hidden lg:table-cell">Created</th>
              <th className="text-table-header text-right px-4 py-3">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-border">
            {isLoading ? (
              Array.from({ length: 6 }).map((_, idx) => (
                <tr key={idx} className="animate-pulse">
                  <td className="px-4 py-4 h-12 bg-surface/50" colSpan={6} />
                </tr>
              ))
            ) : admins.length === 0 ? (
              <tr>
                <td className="px-4 py-8 text-center text-sm text-muted-foreground" colSpan={6}>
                  No admin accounts found.
                </td>
              </tr>
            ) : (
              admins.map((admin) => (
                <tr key={admin.id} className="border-b border-border last:border-0 hover:bg-table-hover transition-colors">
                  <td className="px-4 py-4 font-medium text-foreground">
                    <div className="flex items-center gap-2">
                      <span>{admin.first_name} {admin.last_name}</span>
                      {admin.is_super && (
                        <StatusBadge variant="gold">Super</StatusBadge>
                      )}
                    </div>
                  </td>
                  <td className="px-4 py-4 hidden sm:table-cell text-foreground truncate max-w-[220px]">
                    {admin.email}
                  </td>
                  <td className="px-4 py-4 hidden md:table-cell text-muted-foreground">
                    {admin.phone || "—"}
                  </td>
                  <td className="px-4 py-4">
                    <StatusBadge variant={admin.is_active ? "success" : "danger"}>
                      {admin.is_active ? "Active" : "Inactive"}
                    </StatusBadge>
                  </td>
                  <td className="px-4 py-4 hidden lg:table-cell text-right text-muted-foreground">
                    {new Date(admin.created_at).toLocaleDateString("en-IN", {
                      day: "2-digit", month: "short", year: "numeric",
                    })}
                  </td>
                  <td className="px-4 py-4 text-right">
                    {currentAdmin?.is_super ? (
                      <div className="flex flex-wrap items-center justify-end gap-2">
                        <Button
                          size="sm"
                          variant={admin.is_super ? "secondary" : "default"}
                          onClick={() => {
                            const confirmed = window.confirm(`Are you sure you want to ${admin.is_super ? 'revoke' : 'grant'} super-admin for ${admin.email}?`);
                            if (!confirmed) return;
                            setSuperMut.mutate({ id: admin.id, isSuper: !admin.is_super });
                          }}
                        >
                          {admin.is_super ? "Super" : "Make Super"}
                        </Button>
                        <Button
                          size="sm"
                          variant="destructive"
                          onClick={() => {
                            if (!window.confirm(`Permanently delete admin ${admin.email}? This cannot be undone.`)) return;
                            deleteMut.mutate(admin.id);
                          }}
                        >
                          Delete
                        </Button>
                      </div>
                    ) : (
                      <div className="text-right text-muted-foreground">&nbsp;</div>
                    )}
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      <div className="mt-4 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <p className="text-sm text-muted-foreground">
          Page {page} of {totalPages}
        </p>
        <div className="flex items-center gap-2">
          <button
            className="rounded-lg border border-border bg-card px-3 py-2 text-sm text-foreground transition hover:bg-muted disabled:cursor-not-allowed disabled:opacity-50"
            disabled={page <= 1}
            onClick={() => setPage((current) => Math.max(1, current - 1))}
          >
            Previous
          </button>
          <button
            className="rounded-lg border border-border bg-card px-3 py-2 text-sm text-foreground transition hover:bg-muted disabled:cursor-not-allowed disabled:opacity-50"
            disabled={!meta || page >= totalPages}
            onClick={() => setPage((current) => Math.min(totalPages, current + 1))}
          >
            Next
          </button>
        </div>
      </div>

      <Dialog open={showCreateDialog} onOpenChange={setShowCreateDialog}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Create Admin</DialogTitle>
            <DialogDescription>
              Super-admins can create new admin accounts here. New admins can be granted super-admin access if needed.
            </DialogDescription>
          </DialogHeader>
          <div className="grid gap-4 py-2">
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div>
                <label className="text-sm font-medium text-foreground">First Name</label>
                <Input value={firstName} onChange={(e) => setFirstName(e.target.value)} className="mt-1" />
              </div>
              <div>
                <label className="text-sm font-medium text-foreground">Last Name</label>
                <Input value={lastName} onChange={(e) => setLastName(e.target.value)} className="mt-1" />
              </div>
            </div>
            <div>
              <label className="text-sm font-medium text-foreground">Email</label>
              <Input type="email" value={email} onChange={(e) => setEmail(e.target.value)} className="mt-1" />
            </div>
            <div>
              <label className="text-sm font-medium text-foreground">Password</label>
              <Input type="password" value={password} onChange={(e) => setPassword(e.target.value)} className="mt-1" />
            </div>
            <div className="flex items-center justify-between gap-4">
              <div>
                <p className="text-sm font-medium text-foreground">Make Super Admin</p>
                <p className="text-xs text-muted-foreground">Grant full super-admin access for this account.</p>
              </div>
              <Switch checked={isSuper} onCheckedChange={(checked) => setIsSuper(Boolean(checked))} />
            </div>
          </div>
          <DialogFooter>
            <Button variant="secondary" onClick={() => setShowCreateDialog(false)} type="button">
              Cancel
            </Button>
            <Button
              type="button"
              disabled={createMut.status === "pending"}
              onClick={handleCreateAdmin}
            >
              {createMut.status === "pending" ? "Creating..." : "Create Admin"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
