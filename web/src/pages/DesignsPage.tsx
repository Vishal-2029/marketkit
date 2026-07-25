import { Fragment, useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Trash2, ChevronLeft, ChevronRight, MessageSquareReply, MoreVertical } from "lucide-react";
import { PageHeader } from "@/components/PageHeader";
import { designsService, Design, DesignPurchase, MarketUser } from "@/services/designs";
import { designThreadsService } from "@/services/designThreads";
import { usersService } from "@/services/users";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { useToast } from "@/hooks/use-toast";
import { cn } from "@/lib/utils";

const formatPrice = (paise: number) => `₹${(paise / 100).toLocaleString("en-IN")}`;

export default function DesignsPage() {
  const qc = useQueryClient();
  const { toast } = useToast();
  const [tab, setTab] = useState<"designs" | "purchases" | "users">("designs");
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState("");
  const [purchasePage, setPurchasePage] = useState(1);
  const [threadPurchaseId, setThreadPurchaseId] = useState<string | null>(null);
  const [replyText, setReplyText] = useState("");
  const [userPage, setUserPage] = useState(1);
  const [userSearch, setUserSearch] = useState("");
  const [detailUserId, setDetailUserId] = useState<string | null>(null);
  const [detailTab, setDetailTab] = useState<"designs_sold" | "purchases">("designs_sold");

  const { data, isLoading, error } = useQuery({
    queryKey: ["market-designs", page, search],
    queryFn: () => designsService.listDesigns(page, search),
    enabled: tab === "designs",
  });

  const { data: purchaseData, isLoading: purchasesLoading } = useQuery({
    queryKey: ["market-purchases", purchasePage],
    queryFn: () => designsService.listPurchases(purchasePage),
    enabled: tab === "purchases",
  });

  const { data: threadMessages, isLoading: threadLoading } = useQuery({
    queryKey: ["market-purchase-thread", threadPurchaseId],
    queryFn: () => designThreadsService.listMessages(threadPurchaseId!),
    enabled: !!threadPurchaseId,
  });

  const { data: usersData, isLoading: usersLoading } = useQuery({
    queryKey: ["market-users", userPage, userSearch],
    queryFn: () => designsService.listMarketUsers(undefined, userPage, userSearch),
    enabled: tab === "users",
  });

  const { data: userDetail, isLoading: userDetailLoading } = useQuery({
    queryKey: ["market-user-designs", detailUserId],
    queryFn: () => designsService.getMarketUserDesigns(detailUserId!),
    enabled: !!detailUserId,
  });

  const replyThread = useMutation({
    mutationFn: (content: string) =>
      designThreadsService.reply(threadPurchaseId!, content),
    onSuccess: () => {
      setReplyText("");
      qc.invalidateQueries({ queryKey: ["market-purchase-thread", threadPurchaseId] });
    },
  });

  const deleteDesign = useMutation({
    mutationFn: (id: string) => designsService.deleteDesign(id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["market-designs"] }),
  });

  const deleteUser = useMutation({
    mutationFn: (id: string) => usersService.delete(id),
    onSuccess: () => {
      toast({ title: "User deleted" });
      qc.invalidateQueries({ queryKey: ["market-users"] });
    },
    onError: () => toast({ title: "Failed to delete user", variant: "destructive" }),
  });

  const designs: Design[] = data?.data ?? [];
  const purchases: DesignPurchase[] = purchaseData?.data ?? [];
  const marketUsers: MarketUser[] = usersData?.data ?? [];

  return (
    <div>
      <PageHeader
        title="Design Market"
        subtitle="Oversee designs listed for sale, purchases, and marketplace users"
      />

      <div className="flex gap-2 mt-6 mb-4">
        {(["designs", "purchases", "users"] as const).map((t) => (
          <Button
            key={t}
            size="sm"
            variant={tab === t ? "default" : "outline"}
            onClick={() => {
              setTab(t);
              setUserPage(1);
            }}
          >
            {t === "designs" ? "Designs" : t === "purchases" ? "Purchases" : "Buyer & Seller"}
          </Button>
        ))}
        {tab === "designs" && (
          <input
            className="ml-auto rounded-md border border-border bg-background px-3 py-1.5 text-sm w-64"
            placeholder="Search title or seller…"
            value={search}
            onChange={(e) => {
              setSearch(e.target.value);
              setPage(1);
            }}
          />
        )}
        {tab === "users" && (
          <input
            className="ml-auto rounded-md border border-border bg-background px-3 py-1.5 text-sm w-64"
            placeholder="Search name or email…"
            value={userSearch}
            onChange={(e) => {
              setUserSearch(e.target.value);
              setUserPage(1);
            }}
          />
        )}
      </div>

      {error && (
        <div className="rounded-lg border border-destructive/50 bg-destructive/10 text-destructive px-4 py-3 text-sm mb-4">
          Failed to load designs: {(error as Error).message}
        </div>
      )}

      {tab === "users" ? (
        usersLoading ? (
          <div className="space-y-3">
            {[...Array(5)].map((_, i) => (
              <Skeleton key={i} className="h-16 w-full rounded-xl" />
            ))}
          </div>
        ) : marketUsers.length === 0 ? (
          <div className="text-center py-16 text-muted-foreground">No buyers or sellers yet.</div>
        ) : (
          <>
            <p className="text-sm text-muted-foreground mb-4">
              {usersData?.meta.total ?? 0} total buyers &amp; sellers
            </p>
            <div className="rounded-xl border border-border overflow-hidden">
              <table className="w-full text-sm">
                <thead className="bg-muted/40">
                  <tr>
                    <th className="text-left px-4 py-3 font-medium text-muted-foreground">Name &amp; Email</th>
                    <th className="text-left px-4 py-3 font-medium text-muted-foreground">Mobile</th>
                    <th className="text-left px-4 py-3 font-medium text-muted-foreground">Buy</th>
                    <th className="text-left px-4 py-3 font-medium text-muted-foreground">Sell</th>
                    <th className="text-left px-4 py-3 font-medium text-muted-foreground">Total income</th>
                    <th className="text-left px-4 py-3 font-medium text-muted-foreground">Seller income</th>
                    <th className="text-left px-4 py-3 font-medium text-muted-foreground">Platform income</th>
                    <th className="text-left px-4 py-3 font-medium text-muted-foreground">Action</th>
                  </tr>
                </thead>
                <tbody>
                  {marketUsers.map((u) => (
                    <tr key={u.id} className="border-t border-border hover:bg-muted/20">
                      <td className="px-4 py-3">
                        <p className="font-medium">{u.name}</p>
                        <p className="text-[11px] text-muted-foreground">{u.email}</p>
                      </td>
                      <td className="px-4 py-3 text-muted-foreground">{u.phone || "—"}</td>
                      <td className="px-4 py-3">{u.purchase_count}</td>
                      <td className="px-4 py-3">{u.design_count}</td>
                      <td className="px-4 py-3 font-medium">{formatPrice(u.total_income_in_paise)}</td>
                      <td className="px-4 py-3">{formatPrice(u.seller_income_in_paise)}</td>
                      <td className="px-4 py-3 text-muted-foreground">{formatPrice(u.platform_income_in_paise)}</td>
                      <td className="px-4 py-3">
                        <DropdownMenu>
                          <DropdownMenuTrigger asChild>
                            <Button size="sm" variant="outline" className="h-7 px-2">
                              <MoreVertical className="h-3 w-3" />
                            </Button>
                          </DropdownMenuTrigger>
                          <DropdownMenuContent align="end">
                            <DropdownMenuItem
                              onClick={() => {
                                setDetailTab(u.design_count > 0 ? "designs_sold" : "purchases");
                                setDetailUserId(u.id);
                              }}
                            >
                              Account detail
                            </DropdownMenuItem>
                            <DropdownMenuItem
                              onClick={() => {
                                navigator.clipboard.writeText(u.email);
                                toast({ title: "Email copied" });
                              }}
                            >
                              Copy email
                            </DropdownMenuItem>
                            <DropdownMenuItem
                              className="text-destructive focus:text-destructive"
                              onClick={() => {
                                if (confirm(`Delete ${u.name}? This removes their account entirely.`)) {
                                  deleteUser.mutate(u.id);
                                }
                              }}
                            >
                              Delete
                            </DropdownMenuItem>
                          </DropdownMenuContent>
                        </DropdownMenu>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <Pagination page={userPage} pages={usersData?.meta.pages ?? 1} onChange={setUserPage} />
          </>
        )
      ) : tab === "designs" ? (
        isLoading ? (
          <div className="space-y-3">
            {[...Array(5)].map((_, i) => (
              <Skeleton key={i} className="h-16 w-full rounded-xl" />
            ))}
          </div>
        ) : designs.length === 0 ? (
          <div className="text-center py-16 text-muted-foreground">
            No designs listed yet.
          </div>
        ) : (
          <>
            <p className="text-sm text-muted-foreground mb-4">
              {data?.meta.total ?? 0} total designs
            </p>
            <div className="rounded-xl border border-border overflow-hidden">
              <table className="w-full text-sm">
                <thead className="bg-muted/40">
                  <tr>
                    <th className="text-left px-4 py-3 font-medium text-muted-foreground">Preview</th>
                    <th className="text-left px-4 py-3 font-medium text-muted-foreground">Title</th>
                    <th className="text-left px-4 py-3 font-medium text-muted-foreground">Seller</th>
                    <th className="text-left px-4 py-3 font-medium text-muted-foreground">Price</th>
                    <th className="text-left px-4 py-3 font-medium text-muted-foreground">Format</th>
                    <th className="text-left px-4 py-3 font-medium text-muted-foreground">Sales</th>
                    <th className="text-left px-4 py-3 font-medium text-muted-foreground">Status</th>
                    <th className="text-left px-4 py-3 font-medium text-muted-foreground">Listed</th>
                    <th className="text-left px-4 py-3 font-medium text-muted-foreground">Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {designs.map((d) => (
                    <tr key={d.id} className={cn("border-t border-border hover:bg-muted/20", !d.is_active && "opacity-60")}>
                      <td className="px-4 py-3">
                        {d.preview_urls?.length > 0 ? (
                          <a href={d.preview_urls[0]} target="_blank" rel="noreferrer">
                            <img src={d.preview_urls[0]} alt="" className="w-10 h-10 rounded object-cover border border-border hover:opacity-80 transition-opacity" />
                          </a>
                        ) : (
                          <span className="text-muted-foreground text-xs">—</span>
                        )}
                      </td>
                      <td className="px-4 py-3 max-w-[220px]">
                        <p className="font-medium truncate">{d.title}</p>
                        <p className="text-muted-foreground text-xs truncate mt-0.5">{d.description}</p>
                      </td>
                      <td className="px-4 py-3">
                        <p className="text-xs font-medium">{d.seller_name ?? "—"}</p>
                        {d.seller_email && <p className="text-[11px] text-muted-foreground">{d.seller_email}</p>}
                      </td>
                      <td className="px-4 py-3 font-medium">{formatPrice(d.price_in_paise)}</td>
                      <td className="px-4 py-3">
                        <span className="text-xs bg-primary/10 text-primary px-2 py-1 rounded-full uppercase">{d.file_format}</span>
                      </td>
                      <td className="px-4 py-3 text-muted-foreground">{d.sales_count}</td>
                      <td className="px-4 py-3">
                        <span className={cn("text-xs px-2 py-1 rounded-full", d.is_active ? "bg-emerald-500/10 text-emerald-600" : "bg-muted text-muted-foreground")}>
                          {d.is_active ? "Active" : "Unlisted"}
                        </span>
                      </td>
                      <td className="px-4 py-3 text-muted-foreground text-xs">{new Date(d.created_at).toLocaleDateString()}</td>
                      <td className="px-4 py-3">
                        <Button
                          size="sm"
                          variant="destructive"
                          className="h-7 px-2"
                          onClick={() => {
                            if (
                              confirm(
                                d.sales_count > 0
                                  ? "This design has sales — it will be unlisted but buyers keep their downloads. Continue?"
                                  : "Permanently delete this design?"
                              )
                            )
                              deleteDesign.mutate(d.id);
                          }}
                        >
                          <Trash2 className="h-3 w-3" />
                        </Button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <Pagination page={page} pages={data?.meta.pages ?? 1} onChange={setPage} />
          </>
        )
      ) : purchasesLoading ? (
        <div className="space-y-3">
          {[...Array(5)].map((_, i) => (
            <Skeleton key={i} className="h-16 w-full rounded-xl" />
          ))}
        </div>
      ) : purchases.length === 0 ? (
        <div className="text-center py-16 text-muted-foreground">
          No purchases yet.
        </div>
      ) : (
        <>
          <p className="text-sm text-muted-foreground mb-4">
            {purchaseData?.meta.total ?? 0} total purchases
          </p>
          <div className="rounded-xl border border-border overflow-hidden">
            <table className="w-full text-sm">
              <thead className="bg-muted/40">
                <tr>
                  <th className="text-left px-4 py-3 font-medium text-muted-foreground">Design</th>
                  <th className="text-left px-4 py-3 font-medium text-muted-foreground">Buyer</th>
                  <th className="text-left px-4 py-3 font-medium text-muted-foreground">Seller</th>
                  <th className="text-left px-4 py-3 font-medium text-muted-foreground">Amount</th>
                  <th className="text-left px-4 py-3 font-medium text-muted-foreground">Status</th>
                  <th className="text-left px-4 py-3 font-medium text-muted-foreground">Paid</th>
                  <th className="text-left px-4 py-3 font-medium text-muted-foreground">Support</th>
                </tr>
              </thead>
              <tbody>
                {purchases.map((p) => (
                  <Fragment key={p.id}>
                    <tr className="border-t border-border hover:bg-muted/20">
                      <td className="px-4 py-3 font-medium">{p.design_title ?? "—"}</td>
                      <td className="px-4 py-3">
                        <p className="text-xs font-medium">{p.buyer_name ?? "—"}</p>
                        {p.buyer_email && <p className="text-[11px] text-muted-foreground">{p.buyer_email}</p>}
                      </td>
                      <td className="px-4 py-3">
                        <p className="text-xs font-medium">{p.seller_name ?? "—"}</p>
                        {p.seller_email && <p className="text-[11px] text-muted-foreground">{p.seller_email}</p>}
                      </td>
                      <td className="px-4 py-3 font-medium">{formatPrice(p.amount_in_paise)}</td>
                      <td className="px-4 py-3">
                        <span
                          className={cn(
                            "text-xs px-2 py-1 rounded-full",
                            p.status === "SUCCESS"
                              ? "bg-emerald-500/10 text-emerald-600"
                              : p.status === "PENDING"
                                ? "bg-amber-500/10 text-amber-600"
                                : "bg-destructive/10 text-destructive"
                          )}
                        >
                          {p.status}
                        </span>
                      </td>
                      <td className="px-4 py-3 text-muted-foreground text-xs">
                        {p.paid_at ? new Date(p.paid_at).toLocaleString() : "—"}
                      </td>
                      <td className="px-4 py-3">
                        <Button
                          size="sm"
                          variant="outline"
                          className="h-7 px-2"
                          title="View / reply to buyer's question"
                          onClick={() => {
                            setReplyText("");
                            setThreadPurchaseId(threadPurchaseId === p.id ? null : p.id);
                          }}
                        >
                          <MessageSquareReply className="h-3 w-3" />
                        </Button>
                      </td>
                    </tr>
                    {threadPurchaseId === p.id && (
                      <tr className="border-t border-border bg-muted/10">
                        <td colSpan={7} className="px-4 py-4">
                          <div className="max-w-xl">
                            <p className="text-xs font-medium text-muted-foreground mb-2">
                              Private thread with {p.buyer_name ?? "buyer"} — the seller cannot see this.
                            </p>
                            {threadLoading ? (
                              <Skeleton className="h-16 w-full rounded-lg" />
                            ) : !threadMessages || threadMessages.length === 0 ? (
                              <p className="text-xs text-muted-foreground mb-3">No messages yet.</p>
                            ) : (
                              <div className="space-y-2 mb-3 max-h-64 overflow-y-auto">
                                {threadMessages.map((m) => (
                                  <div
                                    key={m.id}
                                    className={cn(
                                      "rounded-lg px-3 py-2 text-xs",
                                      m.is_admin ? "bg-primary/10" : "bg-background border border-border"
                                    )}
                                  >
                                    <p className="font-medium mb-0.5">
                                      {m.user_name}{m.is_admin ? " (Admin)" : ""}
                                      <span className="font-normal text-muted-foreground ml-2">
                                        {new Date(m.created_at).toLocaleString()}
                                      </span>
                                    </p>
                                    <p className="text-muted-foreground">{m.content}</p>
                                  </div>
                                ))}
                              </div>
                            )}
                            <div className="flex gap-2">
                              <input
                                className="flex-1 rounded-md border border-border bg-background px-3 py-1.5 text-xs"
                                placeholder="Write your reply as admin…"
                                value={replyText}
                                onChange={(e) => setReplyText(e.target.value)}
                                onKeyDown={(e) => {
                                  if (e.key === "Enter" && replyText.trim()) replyThread.mutate(replyText);
                                }}
                              />
                              <Button
                                size="sm"
                                disabled={!replyText.trim() || replyThread.isPending}
                                onClick={() => replyThread.mutate(replyText)}
                              >
                                {replyThread.isPending ? "…" : "Send"}
                              </Button>
                            </div>
                            {replyThread.isError && (
                              <p className="text-xs text-destructive mt-2">
                                Failed: {(replyThread.error as Error).message}
                              </p>
                            )}
                          </div>
                        </td>
                      </tr>
                    )}
                  </Fragment>
                ))}
              </tbody>
            </table>
          </div>
          <Pagination page={purchasePage} pages={purchaseData?.meta.pages ?? 1} onChange={setPurchasePage} />
        </>
      )}

      <Dialog open={!!detailUserId} onOpenChange={(open) => !open && setDetailUserId(null)}>
        <DialogContent className="max-w-3xl">
          <DialogHeader>
            <DialogTitle>{userDetail?.user.name ?? "User details"}</DialogTitle>
            <DialogDescription>
              {userDetail?.user.email}
              {userDetail?.user.phone ? ` • ${userDetail.user.phone}` : ""}
            </DialogDescription>
          </DialogHeader>
          {userDetailLoading ? (
            <div className="space-y-2">
              {[...Array(3)].map((_, i) => (
                <Skeleton key={i} className="h-14 w-full rounded-lg" />
              ))}
            </div>
          ) : !userDetail ? (
            <p className="text-sm text-muted-foreground py-6 text-center">Failed to load account details.</p>
          ) : (
            <Tabs value={detailTab} onValueChange={(v) => setDetailTab(v as "designs_sold" | "purchases")}>
              <TabsList>
                <TabsTrigger value="designs_sold">Designs listed ({userDetail.designs_sold.length})</TabsTrigger>
                <TabsTrigger value="purchases">Purchases ({userDetail.purchases.length})</TabsTrigger>
              </TabsList>

              <TabsContent value="designs_sold">
                {userDetail.designs_sold.length === 0 ? (
                  <p className="text-sm text-muted-foreground py-6 text-center">No designs listed.</p>
                ) : (
                  <div className="max-h-[60vh] overflow-y-auto rounded-lg border border-border">
                    <table className="w-full text-sm">
                      <thead className="bg-muted/40 sticky top-0">
                        <tr>
                          <th className="text-left px-3 py-2 font-medium text-muted-foreground">Photo</th>
                          <th className="text-left px-3 py-2 font-medium text-muted-foreground">Title</th>
                          <th className="text-left px-3 py-2 font-medium text-muted-foreground">Price</th>
                          <th className="text-left px-3 py-2 font-medium text-muted-foreground">Sold</th>
                          <th className="text-left px-3 py-2 font-medium text-muted-foreground">User profit</th>
                          <th className="text-left px-3 py-2 font-medium text-muted-foreground">Platform profit</th>
                        </tr>
                      </thead>
                      <tbody>
                        {userDetail.designs_sold.map((d) => (
                          <tr key={d.id} className="border-t border-border">
                            <td className="px-3 py-2">
                              {d.preview_urls[0] ? (
                                <img
                                  src={d.preview_urls[0]}
                                  alt=""
                                  className="w-10 h-10 rounded object-cover border border-border"
                                />
                              ) : (
                                <span className="text-muted-foreground text-xs">—</span>
                              )}
                            </td>
                            <td className="px-3 py-2 max-w-[180px] truncate">{d.title}</td>
                            <td className="px-3 py-2 font-medium">{formatPrice(d.price_in_paise)}</td>
                            <td className="px-3 py-2">{d.sell_count}</td>
                            <td className="px-3 py-2 text-emerald-600 font-medium">
                              {formatPrice(d.user_profit_in_paise)}
                            </td>
                            <td className="px-3 py-2 text-muted-foreground">
                              {formatPrice(d.pf_profit_in_paise)}
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}
              </TabsContent>

              <TabsContent value="purchases">
                {userDetail.purchases.length === 0 ? (
                  <p className="text-sm text-muted-foreground py-6 text-center">No purchases yet.</p>
                ) : (
                  <div className="max-h-[60vh] overflow-y-auto rounded-lg border border-border">
                    <table className="w-full text-sm">
                      <thead className="bg-muted/40 sticky top-0">
                        <tr>
                          <th className="text-left px-3 py-2 font-medium text-muted-foreground">Photo</th>
                          <th className="text-left px-3 py-2 font-medium text-muted-foreground">Design</th>
                          <th className="text-left px-3 py-2 font-medium text-muted-foreground">Seller</th>
                          <th className="text-left px-3 py-2 font-medium text-muted-foreground">Amount paid</th>
                          <th className="text-left px-3 py-2 font-medium text-muted-foreground">Platform fee</th>
                          <th className="text-left px-3 py-2 font-medium text-muted-foreground">Gateway</th>
                          <th className="text-left px-3 py-2 font-medium text-muted-foreground">Paid at</th>
                        </tr>
                      </thead>
                      <tbody>
                        {userDetail.purchases.map((p) => (
                          <tr key={p.id} className="border-t border-border">
                            <td className="px-3 py-2">
                              {p.preview_url ? (
                                <img
                                  src={p.preview_url}
                                  alt=""
                                  className="w-10 h-10 rounded object-cover border border-border"
                                />
                              ) : (
                                <span className="text-muted-foreground text-xs">—</span>
                              )}
                            </td>
                            <td className="px-3 py-2 max-w-[180px] truncate">{p.design_title}</td>
                            <td className="px-3 py-2 text-muted-foreground">{p.seller_name}</td>
                            <td className="px-3 py-2 font-medium">{formatPrice(p.amount_in_paise)}</td>
                            <td className="px-3 py-2 text-muted-foreground">{formatPrice(p.fee_in_paise)}</td>
                            <td className="px-3 py-2 text-muted-foreground">{p.gateway}</td>
                            <td className="px-3 py-2 text-muted-foreground text-xs">
                              {p.paid_at ? new Date(p.paid_at).toLocaleString() : "—"}
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}
              </TabsContent>
            </Tabs>
          )}
        </DialogContent>
      </Dialog>
    </div>
  );
}

function Pagination({ page, pages, onChange }: { page: number; pages: number; onChange: (p: number) => void }) {
  if (pages <= 1) return null;
  return (
    <div className="flex items-center justify-end gap-2 mt-4">
      <Button size="sm" variant="outline" disabled={page <= 1} onClick={() => onChange(page - 1)}>
        <ChevronLeft className="h-4 w-4" />
      </Button>
      <span className="text-sm text-muted-foreground">
        Page {page} of {pages}
      </span>
      <Button size="sm" variant="outline" disabled={page >= pages} onClick={() => onChange(page + 1)}>
        <ChevronRight className="h-4 w-4" />
      </Button>
    </div>
  );
}
