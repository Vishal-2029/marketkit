import type { ReactNode } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { BrowserRouter, Route, Routes, Navigate } from "react-router-dom";
import { Toaster as Sonner } from "@/components/ui/sonner";
import { Toaster } from "@/components/ui/toaster";
import { TooltipProvider } from "@/components/ui/tooltip";
import { AdminLayout } from "@/components/AdminLayout";
import { AuthProvider, useAuth } from "@/contexts/AuthContext";
import LoginPage from "./pages/LoginPage";
import DashboardPage from "./pages/DashboardPage";
import UsersPage from "./pages/UsersPage";
import VideosPage from "./pages/VideosPage";
import PhotosPage from "./pages/PhotosPage";
import PlansPage from "./pages/PlansPage";
import PaymentsPage from "./pages/PaymentsPage";
import SessionsPage from "./pages/SessionsPage";
import AuditLogsPage from "./pages/AuditLogsPage";
import PlaybackPage from "./pages/PlaybackPage";
import RevenuePage from "./pages/RevenuePage";
import CommunityPage from "./pages/CommunityPage";
import ProductsPage from "./pages/ProductsPage";
import CategoriesPage from "./pages/CategoriesPage";
import MarketPlansPage from "./pages/MarketPlansPage";
import MarketPlanPaymentsPage from "./pages/MarketPlanPaymentsPage";
import ProductMarketRevenuePage from "./pages/ProductMarketRevenuePage";
import NotificationsPage from "./pages/NotificationsPage";
import AdminsPage from "./pages/AdminsPage";
import RefundsPage from "./pages/RefundsPage";
import WithdrawalsPage from "./pages/WithdrawalsPage";
import ProfilePage from "./pages/ProfilePage";
import PlaylistsPage from "./pages/PlaylistsPage";
import NotFound from "./pages/NotFound";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: 1,
      staleTime: 30_000,
      refetchOnWindowFocus: false,
    },
  },
});

function GuestOnlyRoute({ children }: { children: ReactNode }) {
  const { isLoggedIn, isLoading } = useAuth();
  if (isLoading) return null;
  if (isLoggedIn) return <Navigate to="/" replace />;
  return <>{children}</>;
}

function ProtectedLayout() {
  const { isLoggedIn, isLoading } = useAuth();
  // Wait for the refresh token check to complete before redirecting.
  // Without this, the app flashes /login on every page refresh even for
  // valid sessions.
  if (isLoading) return null;
  if (!isLoggedIn) return <Navigate to="/login" replace />;
  return (
    <AdminLayout>
      <Routes>
        <Route path="/" element={<DashboardPage />} />
        <Route path="/users" element={<UsersPage />} />
        <Route path="/videos" element={<VideosPage />} />
        <Route path="/playlists" element={<PlaylistsPage />} />
        <Route path="/photos" element={<PhotosPage />} />
        <Route path="/plans" element={<PlansPage />} />
        <Route path="/payments" element={<PaymentsPage />} />
        <Route path="/refunds" element={<RefundsPage />} />
        <Route path="/withdrawals" element={<WithdrawalsPage />} />
        <Route path="/sessions" element={<SessionsPage />} />
        <Route path="/audit-logs" element={<AuditLogsPage />} />
        <Route path="/playback" element={<PlaybackPage />} />
        <Route path="/revenue" element={<RevenuePage />} />
        <Route path="/community" element={<CommunityPage />} />
        <Route path="/products" element={<ProductsPage />} />
        <Route path="/product-categories" element={<CategoriesPage />} />
        <Route path="/market-plans" element={<MarketPlansPage />} />
        <Route path="/market-plan-payments" element={<MarketPlanPaymentsPage />} />
        <Route path="/product-market-revenue" element={<ProductMarketRevenuePage />} />
        <Route path="/notifications" element={<NotificationsPage />} />
        <Route path="/admins" element={<SuperAdminRoute><AdminsPage /></SuperAdminRoute>} />
        <Route path="/profile" element={<ProfilePage />} />
        <Route path="*" element={<NotFound />} />
      </Routes>
    </AdminLayout>
  );
}

function SuperAdminRoute({ children }: { children: React.ReactNode }) {
  const { isLoggedIn, isLoading, admin } = useAuth();
  if (isLoading) return null;
  if (!isLoggedIn) return <Navigate to="/login" replace />;
  if (!admin?.is_super) return <Navigate to="/" replace />;
  return <>{children}</>;
}

const App = () => (
  <QueryClientProvider client={queryClient}>
    <AuthProvider>
      <TooltipProvider>
        <Toaster />
        <Sonner />
        <BrowserRouter>
          <Routes>
            <Route path="/login" element={<GuestOnlyRoute><LoginPage /></GuestOnlyRoute>} />
            <Route path="*" element={<ProtectedLayout />} />
          </Routes>
        </BrowserRouter>
      </TooltipProvider>
    </AuthProvider>
  </QueryClientProvider>
);

export default App;
