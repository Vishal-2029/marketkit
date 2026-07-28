import { currencySymbol, formatMoney } from "@/lib/currency";
import { useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Camera, Eye, EyeOff, Loader2, LogOut, Mail, Phone, Trash2, User, Wallet, Download, GraduationCap, ShoppingBag, Percent, ArrowDownToLine, FileText, FileSpreadsheet } from "lucide-react";
import { toast } from "sonner";
import { PageHeader } from "@/components/PageHeader";
import { AvatarCropDialog } from "@/components/AvatarCropDialog";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { StatCard } from "@/components/StatCard";
import { useAuth } from "@/contexts/AuthContext";
import { authService } from "@/services/auth";
import { platformWalletService, type PlatformLedgerRow } from "@/services/platformWallet";

function fmtMinor(minor: number) {
  return formatMoney(minor);
}

const SOURCE_LABEL: Record<string, string> = {
  LEARNING_PLAN: "Learning plan",
  MARKET_PLAN: "Market plan",
  PLATFORM_FEE: "Product sale fee",
  WITHDRAWAL: "Withdrawal",
};

function PlatformWalletSection() {
  const qc = useQueryClient();
  const [page, setPage] = useState(1);
  const [withdrawOpen, setWithdrawOpen] = useState(false);
  const [amount, setAmount] = useState("");
  const [note, setNote] = useState("");
  const [exporting, setExporting] = useState(false);

  const summary = useQuery({
    queryKey: ["platform-wallet"],
    queryFn: platformWalletService.get,
  });
  const txs = useQuery({
    queryKey: ["platform-wallet-transactions", page],
    queryFn: () => platformWalletService.transactions({ page, limit: 20 }),
  });

  const withdrawMut = useMutation({
    mutationFn: () =>
      platformWalletService.withdraw({
        amount_minor: Math.round(Number(amount) * 100),
        note: note.trim() || undefined,
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["platform-wallet"] });
      qc.invalidateQueries({ queryKey: ["platform-wallet-transactions"] });
      setWithdrawOpen(false);
      setAmount("");
      setNote("");
      toast.success("Withdrawal recorded.");
    },
    onError: (err: unknown) => {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error;
      toast.error(msg ?? "Withdrawal failed.");
    },
  });

  const handleExport = async (format: "csv" | "pdf") => {
    setExporting(true);
    try {
      const blob = format === "csv"
        ? await platformWalletService.exportCsv()
        : await platformWalletService.exportPdf();
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      const today = new Date().toISOString().slice(0, 10).replace(/-/g, "");
      a.href = url;
      a.download = `platform-wallet-transactions-${today}.${format}`;
      a.click();
      URL.revokeObjectURL(url);
    } catch {
      toast.error(`Failed to export ${format.toUpperCase()}.`);
    } finally {
      setExporting(false);
    }
  };

  const s = summary.data;
  const rows: PlatformLedgerRow[] = txs.data?.data ?? [];
  const meta = txs.data?.meta;

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0">
        <div>
          <CardTitle className="text-lg flex items-center gap-2">
            <Wallet className="h-4.5 w-4.5 text-primary" />
            Platform Wallet
          </CardTitle>
          <CardDescription>
            Organization-wide platform income — visible only to super-admins.
          </CardDescription>
        </div>
        <Button size="sm" onClick={() => setWithdrawOpen(v => !v)}>
          <ArrowDownToLine className="h-4 w-4 mr-1.5" />
          Withdraw
        </Button>
      </CardHeader>
      <CardContent className="space-y-6">
        {summary.isLoading ? (
          <Skeleton className="h-24 rounded-xl" />
        ) : (
          <>
            <div className="rounded-xl border border-border bg-muted/30 p-5">
              <p className="text-caption mb-1">Total balance</p>
              <p className="text-[32px] font-bold text-primary">{fmtMinor(s?.balance_minor ?? 0)}</p>
            </div>
            <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
              <StatCard label="Learning plans" value={fmtMinor(s?.breakdown.learning_plan_minor ?? 0)} />
              <StatCard label="Market plans" value={fmtMinor(s?.breakdown.market_plan_minor ?? 0)} />
              <StatCard label="Product sale fees" value={fmtMinor(s?.breakdown.platform_fee_minor ?? 0)} />
              <StatCard label="Withdrawn" value={fmtMinor(Math.abs(s?.breakdown.withdrawal_minor ?? 0))} />
            </div>
          </>
        )}

        {withdrawOpen && (
          <div className="rounded-xl border border-border p-4 space-y-3">
            <p className="text-sm font-semibold text-foreground">Record a withdrawal</p>
            <div className="grid gap-3 sm:grid-cols-2">
              <div>
                <Label htmlFor="pw_amount">Amount ({currencySymbol()})</Label>
                <Input id="pw_amount" type="number" min="1" value={amount}
                  onChange={e => setAmount(e.target.value)} className="mt-1" />
              </div>
              <div>
                <Label htmlFor="pw_note">Note (optional)</Label>
                <Input id="pw_note" value={note} onChange={e => setNote(e.target.value)}
                  placeholder="e.g. transferred to company account" className="mt-1" />
              </div>
            </div>
            <div className="flex justify-end gap-2">
              <Button variant="ghost" size="sm" onClick={() => setWithdrawOpen(false)}>Cancel</Button>
              <Button
                size="sm"
                disabled={withdrawMut.isPending || !amount || Number(amount) <= 0}
                onClick={() => withdrawMut.mutate()}
              >
                {withdrawMut.isPending ? "Saving…" : "Confirm withdrawal"}
              </Button>
            </div>
          </div>
        )}

        <div className="rounded-xl border border-border overflow-hidden">
          <div className="p-4 border-b border-border flex items-center justify-between">
            <h3 className="text-sm font-semibold text-foreground">Transaction history</h3>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="outline" size="sm" disabled={exporting}>
                  <Download className="h-3.5 w-3.5 mr-1.5" />
                  {exporting ? "Exporting…" : "Export"}
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuItem onClick={() => handleExport("csv")}>
                  <FileSpreadsheet className="h-3.5 w-3.5 mr-2" />
                  Export as CSV
                </DropdownMenuItem>
                <DropdownMenuItem onClick={() => handleExport("pdf")}>
                  <FileText className="h-3.5 w-3.5 mr-2" />
                  Export as PDF
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
          <table className="w-full">
            <thead>
              <tr className="bg-table-header">
                <th className="text-table-header text-left px-4 py-3">Date</th>
                <th className="text-table-header text-left px-4 py-3">Source</th>
                <th className="text-table-header text-left px-4 py-3">Reference</th>
                <th className="text-table-header text-right px-4 py-3">Amount</th>
                <th className="text-table-header text-right px-4 py-3">Balance</th>
              </tr>
            </thead>
            <tbody>
              {txs.isLoading
                ? Array(4).fill(0).map((_, i) => (
                  <tr key={i}><td colSpan={5} className="px-4 py-3"><Skeleton className="h-6" /></td></tr>
                ))
                : rows.map(r => (
                  <tr key={r.id} className="border-b border-border last:border-0 hover:bg-table-hover transition-colors">
                    <td className="px-4 py-3 text-sm text-muted-foreground">
                      {new Date(r.created_at).toLocaleDateString("en-IN", { day: "2-digit", month: "short", year: "numeric" })}
                    </td>
                    <td className="px-4 py-3">
                      <span className="inline-flex items-center gap-1.5 text-sm text-foreground">
                        {r.source === "LEARNING_PLAN" && <GraduationCap className="h-3.5 w-3.5 text-primary" />}
                        {r.source === "MARKET_PLAN" && <ShoppingBag className="h-3.5 w-3.5 text-primary" />}
                        {r.source === "PLATFORM_FEE" && <Percent className="h-3.5 w-3.5 text-primary" />}
                        {r.source === "WITHDRAWAL" && <ArrowDownToLine className="h-3.5 w-3.5 text-danger" />}
                        {SOURCE_LABEL[r.source] ?? r.source}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-xs font-mono text-muted-foreground">
                      {r.reference_id ? r.reference_id.slice(0, 8) : "—"}
                    </td>
                    <td className={`px-4 py-3 text-sm text-right font-semibold ${r.amount_minor < 0 ? "text-danger" : "text-success"}`}>
                      {r.amount_minor < 0 ? "-" : "+"}{fmtMinor(Math.abs(r.amount_minor))}
                    </td>
                    <td className="px-4 py-3 text-sm text-right text-foreground">
                      {fmtMinor(r.balance_after_minor)}
                    </td>
                  </tr>
                ))
              }
              {!txs.isLoading && rows.length === 0 && (
                <tr>
                  <td colSpan={5} className="px-4 py-10 text-center text-sm text-muted-foreground">
                    No platform wallet transactions yet.
                  </td>
                </tr>
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
      </CardContent>
    </Card>
  );
}

export default function ProfilePage() {
  const navigate = useNavigate();
  const { admin, logout, updateProfile, uploadAvatar, changePassword } = useAuth();

  const profile = useQuery({
    queryKey: ["admin-profile"],
    queryFn: authService.getMe,
    initialData: admin ?? undefined,
  });

  const [firstName, setFirstName] = useState("");
  const [lastName, setLastName] = useState("");
  const [phone, setPhone] = useState("");
  const [savingProfile, setSavingProfile] = useState(false);

  const fileInputRef = useRef<HTMLInputElement>(null);
  const [cropSrc, setCropSrc] = useState<string | null>(null);
  const [uploadingAvatar, setUploadingAvatar] = useState(false);
  const [removingAvatar, setRemovingAvatar] = useState(false);

  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [showCurrent, setShowCurrent] = useState(false);
  const [showNew, setShowNew] = useState(false);
  const [savingPassword, setSavingPassword] = useState(false);

  useEffect(() => {
    if (profile.data) {
      setFirstName(profile.data.first_name ?? "");
      setLastName(profile.data.last_name ?? "");
      setPhone(profile.data.phone ?? "");
    }
  }, [profile.data]);

  const data = profile.data;
  const initial = data?.first_name?.[0]?.toUpperCase() ?? "A";

  const handleSaveProfile = async () => {
    if (!firstName.trim()) {
      toast.error("First name is required.");
      return;
    }

    setSavingProfile(true);
    try {
      await updateProfile({
        first_name: firstName.trim(),
        last_name: lastName.trim(),
        phone: phone.trim(),
      });
      toast.success("Profile updated.");
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error;
      toast.error(msg ?? "Failed to update profile.");
    } finally {
      setSavingProfile(false);
    }
  };

  const handleChangePassword = async () => {
    if (!currentPassword || !newPassword) {
      toast.error("Please fill in all password fields.");
      return;
    }
    if (newPassword.length < 6) {
      toast.error("New password must be at least 6 characters.");
      return;
    }
    if (newPassword !== confirmPassword) {
      toast.error("New passwords do not match.");
      return;
    }

    setSavingPassword(true);
    try {
      await changePassword(currentPassword, newPassword);
      toast.success("Password updated.");
      setCurrentPassword("");
      setNewPassword("");
      setConfirmPassword("");
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error;
      toast.error(msg ?? "Failed to change password.");
    } finally {
      setSavingPassword(false);
    }
  };

  const closeCrop = () => {
    setCropSrc((prev) => {
      if (prev) URL.revokeObjectURL(prev);
      return null;
    });
  };

  const handleFilePick = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    e.target.value = ""; // allow re-selecting the same file
    if (!file) return;
    setCropSrc(URL.createObjectURL(file));
  };

  const handleRemoveAvatar = async () => {
    setRemovingAvatar(true);
    try {
      await authService.removeAvatar();
      await profile.refetch();
      toast.success("Profile photo removed.");
    } catch {
      toast.error("Failed to remove photo.");
    } finally {
      setRemovingAvatar(false);
    }
  };

  const handleCropped = async (blob: Blob) => {
    setUploadingAvatar(true);
    try {
      await uploadAvatar(blob);
      await profile.refetch();
      toast.success("Profile photo updated.");
      closeCrop();
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error;
      toast.error(msg ?? "Failed to upload photo.");
    } finally {
      setUploadingAvatar(false);
    }
  };

  const handleLogout = () => {
    logout();
    navigate("/login");
  };

  if (profile.isLoading && !data) {
    return (
      <div>
        <PageHeader title="Profile" subtitle="Your admin account" />
        <div className="grid gap-6 lg:grid-cols-[280px_1fr]">
          <Skeleton className="h-64 rounded-xl" />
          <div className="space-y-6">
            <Skeleton className="h-72 rounded-xl" />
            <Skeleton className="h-64 rounded-xl" />
          </div>
        </div>
      </div>
    );
  }

  return (
    <div>
      <PageHeader title="Profile" subtitle="Manage your admin account settings">
        <Button variant="outline" onClick={handleLogout}>
          <LogOut className="h-4 w-4 mr-2" />
          Log out
        </Button>
      </PageHeader>

      <div className="grid gap-6 lg:grid-cols-[280px_1fr]">
        {/* Summary card */}
        <Card>
          <CardContent className="pt-6 flex flex-col items-center text-center">
            <div className="relative w-20 h-20 mb-4">
              {data?.avatar_url ? (
                <img
                  src={data.avatar_url}
                  alt={`${data.first_name ?? "Admin"} avatar`}
                  className="w-20 h-20 rounded-full object-cover ring-4 ring-primary/10"
                />
              ) : (
                <div className="w-20 h-20 rounded-full bg-primary/20 flex items-center justify-center ring-4 ring-primary/10">
                  <span className="text-2xl font-semibold text-primary">{initial}</span>
                </div>
              )}
              <button
                type="button"
                onClick={() => fileInputRef.current?.click()}
                disabled={uploadingAvatar}
                aria-label="Change photo"
                className="absolute bottom-0 right-0 flex h-7 w-7 items-center justify-center rounded-full bg-primary text-primary-foreground shadow ring-2 ring-card transition-colors hover:bg-primary/90 disabled:opacity-60"
              >
                {uploadingAvatar ? (
                  <Loader2 className="h-3.5 w-3.5 animate-spin" />
                ) : (
                  <Camera className="h-3.5 w-3.5" />
                )}
              </button>
              <input
                ref={fileInputRef}
                type="file"
                accept="image/*"
                className="hidden"
                onChange={handleFilePick}
              />
            </div>
            {data?.avatar_url && (
              <button
                type="button"
                onClick={handleRemoveAvatar}
                disabled={removingAvatar}
                className="mt-1 mb-1 flex items-center gap-1 text-xs text-muted-foreground hover:text-danger transition-colors disabled:opacity-50"
              >
                {removingAvatar ? (
                  <Loader2 className="h-3 w-3 animate-spin" />
                ) : (
                  <Trash2 className="h-3 w-3" />
                )}
                Remove photo
              </button>
            )}
            <h2 className="text-lg font-semibold">
              {data?.first_name} {data?.last_name}
            </h2>
            <p className="text-sm text-muted-foreground mt-1">{data?.email}</p>
            {data?.created_at && (
              <p className="text-xs text-muted-foreground mt-4">
                Member since {new Date(data.created_at).toLocaleDateString(undefined, {
                  year: "numeric",
                  month: "long",
                  day: "numeric",
                })}
              </p>
            )}
          </CardContent>
        </Card>

        <div className="space-y-6">
          {/* Edit profile */}
          <Card>
            <CardHeader>
              <CardTitle className="text-lg">Personal information</CardTitle>
              <CardDescription>Update your name and contact details.</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="grid gap-4 sm:grid-cols-2">
                <div className="space-y-2">
                  <Label htmlFor="first_name">First name</Label>
                  <div className="relative">
                    <User className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                    <Input
                      id="first_name"
                      value={firstName}
                      onChange={(e) => setFirstName(e.target.value)}
                      className="pl-9"
                    />
                  </div>
                </div>
                <div className="space-y-2">
                  <Label htmlFor="last_name">Last name</Label>
                  <div className="relative">
                    <User className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                    <Input
                      id="last_name"
                      value={lastName}
                      onChange={(e) => setLastName(e.target.value)}
                      className="pl-9"
                    />
                  </div>
                </div>
              </div>

              <div className="space-y-2">
                <Label htmlFor="email">Email</Label>
                <div className="relative">
                  <Mail className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                  <Input id="email" value={data?.email ?? ""} disabled className="pl-9 bg-muted/50" />
                </div>
                <p className="text-xs text-muted-foreground">Email cannot be changed here.</p>
              </div>

              <div className="space-y-2">
                <Label htmlFor="phone">Phone</Label>
                <div className="relative">
                  <Phone className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
                  <Input
                    id="phone"
                    value={phone}
                    onChange={(e) => setPhone(e.target.value)}
                    placeholder="+91 98765 43210"
                    className="pl-9"
                  />
                </div>
              </div>

              <div className="flex justify-end pt-2">
                <Button onClick={handleSaveProfile} disabled={savingProfile}>
                  {savingProfile ? "Saving…" : "Save changes"}
                </Button>
              </div>
            </CardContent>
          </Card>

          {/* Change password */}
          <Card>
            <CardHeader>
              <CardTitle className="text-lg">Change password</CardTitle>
              <CardDescription>Update your login password.</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="space-y-2">
                <Label htmlFor="current_password">Current password</Label>
                <div className="relative">
                  <Input
                    id="current_password"
                    type={showCurrent ? "text" : "password"}
                    value={currentPassword}
                    onChange={(e) => setCurrentPassword(e.target.value)}
                    autoComplete="current-password"
                  />
                  <button
                    type="button"
                    onClick={() => setShowCurrent((v) => !v)}
                    className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                  >
                    {showCurrent ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                  </button>
                </div>
              </div>

              <div className="grid gap-4 sm:grid-cols-2">
                <div className="space-y-2">
                  <Label htmlFor="new_password">New password</Label>
                  <div className="relative">
                    <Input
                      id="new_password"
                      type={showNew ? "text" : "password"}
                      value={newPassword}
                      onChange={(e) => setNewPassword(e.target.value)}
                      autoComplete="new-password"
                    />
                    <button
                      type="button"
                      onClick={() => setShowNew((v) => !v)}
                      className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                    >
                      {showNew ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                    </button>
                  </div>
                </div>
                <div className="space-y-2">
                  <Label htmlFor="confirm_password">Confirm new password</Label>
                  <Input
                    id="confirm_password"
                    type="password"
                    value={confirmPassword}
                    onChange={(e) => setConfirmPassword(e.target.value)}
                    autoComplete="new-password"
                  />
                </div>
              </div>

              <div className="flex justify-end pt-2">
                <Button variant="secondary" onClick={handleChangePassword} disabled={savingPassword}>
                  {savingPassword ? "Updating…" : "Update password"}
                </Button>
              </div>
            </CardContent>
          </Card>

          {admin?.is_super && <PlatformWalletSection />}
        </div>
      </div>

      <AvatarCropDialog
        open={!!cropSrc}
        imageSrc={cropSrc}
        onClose={closeCrop}
        onCropped={handleCropped}
      />
    </div>
  );
}
