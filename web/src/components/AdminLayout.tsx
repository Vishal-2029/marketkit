import { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { Link, useLocation, useNavigate } from "react-router-dom";
import {
  LayoutDashboard, Video, Image, Users, Monitor, CreditCard, DollarSign,
  FileText, BarChart3, ScrollText, LogOut, ChevronLeft, Menu, MessageSquare, Bell, X, UserCircle, RotateCcw, Wallet,
  ShoppingBag, BadgePercent, Receipt, FolderTree
} from "lucide-react";
import { useQuery } from "@tanstack/react-query";
import { cn } from "@/lib/utils";
import { useAuth } from "@/contexts/AuthContext";
import { useIsMobile } from "@/hooks/use-mobile";
import { notificationsService, NotificationLog } from "@/services/notifications";
import { Accordion, AccordionItem, AccordionTrigger, AccordionContent } from "@/components/ui/accordion";

const LS_KEY = "notif_last_seen";

function formatAudience(audience: string): string {
  if (audience === "all") return "All Users";
  if (audience.startsWith("plan:") && audience.includes("+status:")) {
    const [p, s] = audience.split("+status:");
    return `${p.replace("plan:", "")} · ${s}`;
  }
  if (audience.startsWith("plan:")) return `Plan: ${audience.replace("plan:", "")}`;
  if (audience.startsWith("status:")) return `Status: ${audience.replace("status:", "")}`;
  return audience;
}

function stripCta(body: string): string {
  return body.replace(/ · Tap to (view|watch)$/i, "");
}

function timeAgo(dateStr: string) {
  const diff = Date.now() - new Date(dateStr).getTime();
  const m = Math.floor(diff / 60000);
  if (m < 1) return "just now";
  if (m < 60) return `${m}m ago`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h}h ago`;
  return `${Math.floor(h / 24)}d ago`;
}

function BellDropdown() {
  const navigate = useNavigate();
  const [open, setOpen] = useState(false);
  const [modal, setModal] = useState<NotificationLog | null>(null);
  const [lastSeen, setLastSeen] = useState<number>(() => {
    const v = localStorage.getItem(LS_KEY);
    return v ? parseInt(v, 10) : 0;
  });
  const dropRef = useRef<HTMLDivElement>(null);

  const { data } = useQuery({
    queryKey: ["notifications", 1],
    queryFn: () => notificationsService.listNotifications(1),
    refetchInterval: 60_000,
  });

  const logs: NotificationLog[] = data?.logs ?? [];
  const unseen = logs.filter((l) => new Date(l.created_at).getTime() > lastSeen).length;

  const handleOpen = () => {
    const now = Date.now();
    localStorage.setItem(LS_KEY, String(now));
    setLastSeen(now);
    setOpen((v) => !v);
  };

  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (dropRef.current && !dropRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener("mousedown", handler);
    return () => document.removeEventListener("mousedown", handler);
  }, []);

  return (
    <>
      <div ref={dropRef} className="relative">
        <button
          onClick={handleOpen}
          className="relative p-2 rounded-lg text-muted-foreground hover:text-foreground hover:bg-accent transition-colors"
          aria-label="Notifications"
        >
          <Bell className="h-5 w-5" />
          {unseen > 0 && (
            <span className="absolute -top-0.5 -right-0.5 min-w-[18px] h-[18px] bg-destructive text-white text-[10px] font-bold rounded-full flex items-center justify-center px-1">
              {unseen > 9 ? "9+" : unseen}
            </span>
          )}
        </button>

        {open && (
          <div className="absolute right-0 top-full mt-2 w-80 bg-card border border-border rounded-xl shadow-lg z-50 overflow-hidden">
            <div className="flex items-center justify-between px-4 py-3 border-b border-border">
              <p className="text-sm font-semibold">Notifications</p>
              <button onClick={() => setOpen(false)} className="text-muted-foreground hover:text-foreground">
                <X className="h-4 w-4" />
              </button>
            </div>
            <div className="max-h-72 overflow-y-auto divide-y divide-border">
              {logs.length === 0 ? (
                <p className="text-sm text-muted-foreground text-center py-8">No notifications sent yet.</p>
              ) : (
                logs.slice(0, 10).map((log) => (
                  <button
                    key={log.id}
                    onClick={() => { setModal(log); setOpen(false); }}
                    className="w-full text-left px-4 py-3 hover:bg-muted/50 transition-colors"
                  >
                    <p className="text-sm font-medium truncate">{log.title}</p>
                    <p className="text-xs text-muted-foreground mt-0.5">{timeAgo(log.created_at)}</p>
                  </button>
                ))
              )}
            </div>
            <div className="border-t border-border px-4 py-2">
              <button
                onClick={() => { setOpen(false); navigate("/notifications"); }}
                className="text-xs text-primary hover:underline"
              >
                View all notifications →
              </button>
            </div>
          </div>
        )}
      </div>

      {/* Detail modal — rendered via portal to escape the sticky header's stacking context */}
      {modal && createPortal(
        <div
          className="fixed inset-0 bg-black/50 z-[60] flex items-center justify-center p-4"
          onClick={() => setModal(null)}
        >
          <div
            className="bg-card border border-border rounded-xl w-full max-w-md p-6 shadow-xl"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-start justify-between mb-4">
              <h3 className="font-semibold text-base">{modal.title}</h3>
              <button onClick={() => setModal(null)} className="text-muted-foreground hover:text-foreground ml-2 shrink-0">
                <X className="h-4 w-4" />
              </button>
            </div>
            <p className="text-sm text-foreground mb-4">{stripCta(modal.body)}</p>
            <div className="grid grid-cols-2 gap-3 text-xs">
              <div className="bg-muted/50 rounded-lg p-3">
                <p className="text-muted-foreground mb-1">Audience</p>
                <p className="font-medium">{formatAudience(modal.audience)}</p>
              </div>
              <div className="bg-muted/50 rounded-lg p-3">
                <p className="text-muted-foreground mb-1">Sent / Failed</p>
                <p className="font-medium">
                  <span className="text-green-600">{modal.sent_count}</span>
                  {" / "}
                  <span className="text-destructive">{modal.failed_count}</span>
                </p>
              </div>
              <div className="col-span-2 bg-muted/50 rounded-lg p-3">
                <p className="text-muted-foreground mb-1">Sent at</p>
                <p className="font-medium">{new Date(modal.created_at).toLocaleString()}</p>
              </div>
            </div>
          </div>
        </div>
      , document.body)}
    </>
  );
}

const navSections = [
  {
    label: "OVERVIEW",
    items: [
      { title: "Dashboard", url: "/", icon: LayoutDashboard },
    ],
  },
  {
    label: "CONTENT",
    items: [
      { title: "Videos", url: "/videos", icon: Video },
      { title: "Playlists", url: "/playlists", icon: ScrollText },
      { title: "Practice Photos", url: "/photos", icon: Image },
    ],
  },
  {
    label: "USERS",
    items: [
      { title: "Users", url: "/users", icon: Users },
      { title: "Admins", url: "/admins", icon: UserCircle },
      { title: "Sessions", url: "/sessions", icon: Monitor },
    ],
  },
  {
    label: "BUSINESS",
    items: [
      { title: "Plans", url: "/plans", icon: CreditCard },
      { title: "Payments", url: "/payments", icon: DollarSign },
      { title: "Refunds", url: "/refunds", icon: RotateCcw },
      { title: "Learning Revenue", url: "/revenue", icon: BarChart3 },
    ],
  },
  {
    label: "PRODUCT MARKET",
    items: [
      { title: "Sections", url: "/product-categories", icon: FolderTree },
      { title: "Products & Purchases", url: "/products", icon: ShoppingBag },
      { title: "Market Plans", url: "/market-plans", icon: BadgePercent },
      { title: "Plan Payments", url: "/market-plan-payments", icon: Receipt },
      { title: "Market Revenue", url: "/product-market-revenue", icon: BarChart3 },
      { title: "Withdrawal Settings", url: "/withdrawals", icon: Wallet },
    ],
  },
  {
    label: "COMMUNITY",
    items: [
      { title: "Community", url: "/community", icon: MessageSquare },
      { title: "Notifications", url: "/notifications", icon: Bell },
    ],
  },
  {
    label: "LOGS",
    items: [
      { title: "Audit Logs", url: "/audit-logs", icon: ScrollText },
      { title: "Playback Reports", url: "/playback", icon: FileText },
    ],
  },
  {
    label: "ACCOUNT",
    items: [
      { title: "Profile", url: "/profile", icon: UserCircle },
    ],
  },
];

const allNavItems = navSections.flatMap((s) => s.items);

interface AdminLayoutProps {
  children: React.ReactNode;
}

export function AdminLayout({ children }: AdminLayoutProps) {
  const [mobileOpen, setMobileOpen] = useState(false);
  const isMobile = useIsMobile();
  const location = useLocation();
  const navigate = useNavigate();
  const { logout, admin } = useAuth();
  const isSuperAdmin = admin?.is_super;

  const adminInitial = admin?.first_name ? admin.first_name[0].toUpperCase() : "A";

  // Mobile drawer's nav sections start collapsed except the one containing
  // the current page.
  const [openSections, setOpenSections] = useState<string[]>(() => {
    const active = navSections.find((s) => s.items.some((i) => i.url === location.pathname));
    return active ? [active.label] : [];
  });

  useEffect(() => {
    const active = navSections.find((s) => s.items.some((i) => i.url === location.pathname));
    if (!active) return;
    setOpenSections((prev) => (prev.includes(active.label) ? prev : [...prev, active.label]));
  }, [location.pathname]);

  useEffect(() => {
    if (isMobile) setMobileOpen(false);
  }, [location.pathname, isMobile]);

  useEffect(() => {
    if (isMobile && mobileOpen) {
      document.body.style.overflow = "hidden";
    } else {
      document.body.style.overflow = "";
    }
    return () => { document.body.style.overflow = ""; };
  }, [isMobile, mobileOpen]);

  const handleLogout = () => {
    logout();
    navigate("/login");
  };

  const renderNavItem = (item: (typeof navSections)[number]["items"][number]) => {
    const active = location.pathname === item.url;
    const itemIndex = allNavItems.findIndex((n) => n.url === item.url);
    return (
      <Link
        key={item.url}
        to={item.url}
        style={mobileOpen ? { animationDelay: `${itemIndex * 35}ms` } : undefined}
        className={cn(
          "flex items-center gap-3 h-10 rounded-lg px-3 text-sm font-medium transition-colors mb-0.5",
          active
            ? "bg-sidebar-accent text-sidebar-accent-foreground border-l-[3px] border-primary shadow-[inset_2px_0_8px_hsl(var(--primary)/0.08)]"
            : "text-sidebar-foreground hover:bg-muted",
          mobileOpen && "animate-fade-slide-in"
        )}
      >
        <item.icon className={cn("h-4 w-4 shrink-0", active && "text-primary")} />
        <span>{item.title}</span>
      </Link>
    );
  };

  // Mobile drawer keeps the full labeled nav (touch users can't hover for
  // tooltips, so labels + a logout affordance stay here). Branding lives in
  // the persistent top bar, which stays visible above this drawer.
  const mobileSidebarContent = (
    <>
      <div className="h-12 flex items-center px-4 border-b border-sidebar-border">
        <span className="text-sm font-medium text-muted-foreground">Menu</span>
        <button
          onClick={() => setMobileOpen(false)}
          className="ml-auto p-1.5 rounded-lg text-muted-foreground hover:text-foreground hover:bg-muted transition-colors"
        >
          <ChevronLeft className="h-4 w-4" />
        </button>
      </div>

      <nav className="flex-1 overflow-y-auto py-4 px-2">
        <Accordion type="multiple" value={openSections} onValueChange={setOpenSections}>
          {navSections.map((section) => (
            <AccordionItem key={section.label} value={section.label} className="border-b-0 mb-1">
              <AccordionTrigger className="py-1.5 px-3 text-[11px] font-semibold text-muted-foreground/60 uppercase tracking-wider hover:no-underline hover:text-muted-foreground [&>svg]:h-3.5 [&>svg]:w-3.5">
                {section.label}
              </AccordionTrigger>
              <AccordionContent className="pb-1 pt-0.5">
                {section.items
                  .filter((item) => item.url !== "/admins" || isSuperAdmin)
                  .map((item) => renderNavItem(item))}
              </AccordionContent>
            </AccordionItem>
          ))}
        </Accordion>
      </nav>

      {/* Bottom profile */}
      <div className="border-t border-sidebar-border p-3">
        <div className="flex items-center gap-3">
          <div className="w-8 h-8 rounded-full shrink-0 ring-2 ring-primary/10 overflow-hidden">
            {admin?.avatar_url ? (
              <img src={admin.avatar_url} alt="Profile" className="w-full h-full object-cover" />
            ) : (
              <div className="w-full h-full bg-primary/20 flex items-center justify-center">
                <span className="text-xs font-semibold text-primary">{adminInitial}</span>
              </div>
            )}
          </div>
          <div className="flex-1 min-w-0">
            <p className="text-sm font-medium text-foreground truncate">{admin?.first_name} {admin?.last_name}</p>
            <p className="text-caption truncate">{admin?.email}</p>
          </div>
          <button
            className="text-muted-foreground hover:text-foreground transition-colors"
            onClick={handleLogout}
            title="Logout"
          >
            <LogOut className="h-4 w-4" />
          </button>
        </div>
      </div>
    </>
  );

  return (
    <div className="min-h-screen flex flex-col w-full">

      {/* ── Full-width top bar — logo/name (left), bell/profile (right) ── */}
      <header className="h-14 bg-card/80 backdrop-blur-md border-b border-border flex items-center justify-between px-4 md:px-6 fixed top-0 left-0 right-0 z-40 shadow-soft">
        <div className="flex items-center gap-3">
          <button
            onClick={() => setMobileOpen(true)}
            className="md:hidden p-2 -ml-2 rounded-lg text-muted-foreground hover:text-foreground hover:bg-accent transition-colors"
            aria-label="Open menu"
          >
            <Menu className="h-5 w-5" />
          </button>
          <img src="/icon.svg" alt="MarketKit" className="w-7 h-7 rounded-lg shadow-sm object-contain" />
          <span className="font-semibold text-foreground text-sm">MarketKit</span>
        </div>
        <div className="flex items-center gap-2">
          <BellDropdown />
          <Link to="/profile" className="flex items-center gap-2 hover:opacity-80 transition-opacity">
            <span className="text-sm text-foreground hidden sm:block">{admin ? `${admin.first_name} ${admin.last_name}` : ""}</span>
            <div className="w-8 h-8 rounded-full shadow-brand ring-2 ring-background overflow-hidden shrink-0">
              {admin?.avatar_url ? (
                <img src={admin.avatar_url} alt="Profile" className="w-full h-full object-cover" />
              ) : (
                <div className="w-full h-full bg-gradient-primary flex items-center justify-center">
                  <span className="text-xs font-bold text-primary-foreground">{adminInitial}</span>
                </div>
              )}
            </div>
          </Link>
        </div>
      </header>

      <div className="flex flex-1 pt-14">

        {/* ── Mobile backdrop ── */}
        {isMobile && (
          <div
            onClick={() => setMobileOpen(false)}
            className={cn(
              "fixed inset-0 bg-black/50 z-40 md:hidden transition-opacity duration-300",
              mobileOpen ? "opacity-100 pointer-events-auto" : "opacity-0 pointer-events-none"
            )}
          />
        )}

        {/* ── Mobile sidebar ── */}
        {isMobile && (
          <aside
            className={cn(
              "fixed top-14 left-0 h-[calc(100%-3.5rem)] w-72 bg-sidebar border-r border-sidebar-border flex flex-col z-50",
              "transition-transform duration-300 ease-out",
              mobileOpen ? "translate-x-0" : "-translate-x-full"
            )}
          >
            {mobileSidebarContent}
          </aside>
        )}

        {/* ── Desktop sidebar — permanent labeled nav grouped by section ── */}
        {!isMobile && (
          <aside className="fixed top-14 left-0 h-[calc(100%-3.5rem)] w-60 bg-sidebar border-r border-sidebar-border flex flex-col z-30">
            <nav className="flex-1 overflow-y-auto py-4 px-2">
              {navSections.map((section) => (
                <div key={section.label} className="mb-3">
                  <p className="py-1.5 px-3 text-[11px] font-semibold text-muted-foreground/60 uppercase tracking-wider">
                    {section.label}
                  </p>
                  {section.items
                    .filter((item) => item.url !== "/admins" || isSuperAdmin)
                    .map((item) => renderNavItem(item))}
                </div>
              ))}
            </nav>
          </aside>
        )}

        {/* ── Main content ── */}
        <div className={cn("flex-1 flex flex-col", !isMobile && "ml-60")}>
          <main className="flex-1 p-4 md:p-8">
            <div className="max-w-[1280px] mx-auto">
              {children}
            </div>
          </main>
        </div>
      </div>
    </div>
  );
}
