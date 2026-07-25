import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import '../services/app_mode_service.dart';
import '../../features/auth/screens/splash_screen.dart';
import '../../features/auth/screens/auth_screen.dart';
import '../../features/auth/screens/otp_screen.dart';
import '../../features/community/screens/community_screen.dart';
import '../../features/community/screens/post_detail_screen.dart';
import '../../features/downloads/screens/downloads_screen.dart';
import '../../features/favorites/screens/favorites_screen.dart';
import '../../features/photos/screens/gallery_screen.dart';
import '../../features/home/models/playlist_model.dart';
import '../../features/home/models/video_model.dart';
import '../../features/home/screens/home_screen.dart';
import '../../features/home/screens/playlist_detail_screen.dart';
import '../../features/library/screens/library_screen.dart';
import '../../features/marketplace/screens/design_detail_screen.dart';
import '../../features/marketplace/screens/marketplace_shell.dart';
import '../../features/marketplace/screens/my_designs_screen.dart';
import '../../features/marketplace/screens/payout_details_screen.dart';
import '../../features/marketplace/screens/purchase_receipt_screen.dart';
import '../../features/marketplace/screens/purchase_thread_screen.dart';
import '../../features/marketplace/screens/wallet_transactions_screen.dart';
import '../../features/mode/screens/mode_chooser_screen.dart';
import '../../features/onboarding/screens/onboarding_carousel_screen.dart';
import '../../features/plans/screens/plans_screen.dart';
import '../../features/auth/screens/forgot_password_screen.dart';
import '../../features/auth/screens/reset_password_screen.dart';
import '../../features/profile/screens/change_password_screen.dart';
import '../../features/profile/screens/edit_profile_screen.dart';
import '../../features/profile/screens/profile_screen.dart';
import '../../features/video_player/screens/video_player_screen.dart';

final appRouter = GoRouter(
  initialLocation: '/',
  routes: [
    GoRoute(
      path: '/',
      builder: (context, state) => const SplashScreen(),
    ),
    // Login and signup are one combined screen (AuthScreen) — which side
    // shows is local widget state, toggled in place with no route change.
    // These two routes are just the two entry points into it.
    GoRoute(
      path: '/login',
      builder: (context, state) => const AuthScreen(),
    ),
    GoRoute(
      path: '/register',
      builder: (context, state) {
        final extra = state.extra as Map<String, String>?;
        return AuthScreen(mode: extra?['mode'], startOnSignup: true);
      },
    ),
    GoRoute(
      path: '/forgot-password',
      builder: (context, state) => const ForgotPasswordScreen(),
    ),
    GoRoute(
      path: '/reset-password',
      builder: (context, state) {
        final email = state.extra as String? ?? '';
        return ResetPasswordScreen(email: email);
      },
    ),
    GoRoute(
      path: '/otp',
      builder: (context, state) {
        final extra = state.extra as Map<String, String>? ?? {};
        return OtpScreen(
          email: extra['email'] ?? '',
          password: extra['password'],
          mode: extra['mode'],
        );
      },
    ),
    // Video player — outside ShellRoute so no bottom nav bar is shown
    GoRoute(
      path: '/video/:id',
      builder: (context, state) {
        final video = state.extra as VideoModel?;
        if (video == null) return const SizedBox.shrink();
        return VideoPlayerScreen(video: video);
      },
    ),
    // Post detail — outside ShellRoute for full-screen view
    GoRoute(
      path: '/community/:id',
      builder: (context, state) => PostDetailScreen(
        postId: state.pathParameters['id']!,
      ),
    ),
    // First-launch intro carousel — shown once per device, before any account
    // exists. Full-screen, no bottom nav.
    GoRoute(
      path: '/onboarding',
      builder: (context, state) => const OnboardingCarouselScreen(),
    ),
    // Mode chooser — full-screen, no bottom nav. Reached either pre-registration
    // (from onboarding, or the login screen's "Register" link) or, for legacy
    // logged-in accounts with no stored mode, post-login.
    GoRoute(
      path: '/choose-mode',
      builder: (context, state) => const ModeChooserScreen(),
    ),
    // Design market — full-screen with its own internal bottom nav
    GoRoute(
      path: '/market',
      builder: (context, state) => const MarketplaceShell(),
    ),
    GoRoute(
      path: '/market/design/:id',
      builder: (context, state) => DesignDetailScreen(
        designId: state.pathParameters['id']!,
      ),
    ),
    GoRoute(
      path: '/market/design/:id/messages',
      builder: (context, state) => PurchaseThreadScreen(
        designId: state.pathParameters['id']!,
        designTitle: (state.extra as String?) ?? 'Design',
      ),
    ),
    // Wallet sub-screens — full-screen, no bottom nav
    GoRoute(
      path: '/market/wallet/transactions',
      builder: (context, state) => const WalletTransactionsScreen(),
    ),
    GoRoute(
      path: '/market/wallet/payout-details',
      builder: (context, state) => const PayoutDetailsScreen(),
    ),
    GoRoute(
      path: '/market/purchase/:id/receipt',
      builder: (context, state) => PurchaseReceiptScreen(
        purchaseId: state.pathParameters['id']!,
      ),
    ),
    GoRoute(
      path: '/market/my-designs',
      builder: (context, state) => const MyDesignsScreen(),
    ),
    // Playlist detail — full-screen, no bottom nav
    GoRoute(
      path: '/playlists/:id',
      builder: (context, state) {
        final playlist = state.extra as PlaylistModel?;
        if (playlist == null) return const SizedBox.shrink();
        return PlaylistDetailScreen(playlist: playlist);
      },
    ),
    // Gallery — full-screen, no bottom nav
    GoRoute(
      path: '/gallery',
      builder: (context, state) => const GalleryScreen(),
    ),
    // Downloads — full-screen, no bottom nav
    GoRoute(
      path: '/downloads',
      builder: (context, state) => const DownloadsScreen(),
    ),
    // Favorites — full-screen, no bottom nav
    GoRoute(
      path: '/favorites',
      builder: (context, state) => const FavoritesScreen(),
    ),
    // Profile sub-screens — outside ShellRoute so no bottom nav shown
    GoRoute(
      path: '/profile/edit',
      builder: (context, state) => const EditProfileScreen(),
    ),
    GoRoute(
      path: '/profile/change-password',
      builder: (context, state) => const ChangePasswordScreen(),
    ),
    // Shell route with bottom navigation for the main app
    ShellRoute(
      builder: (context, state, child) => _AppShell(child: child),
      routes: [
        // While Learning is locked ("Coming soon"), its landing tab bounces
        // to the market so no navigation or old stored mode slips past the
        // UI locks.
        GoRoute(
          path: '/home',
          redirect: (context, state) =>
              AppModeService.learningLocked ? '/market' : null,
          builder: (context, state) => const HomeScreen(),
        ),
        GoRoute(
          path: '/library',
          builder: (context, state) => const LibraryScreen(),
        ),
        GoRoute(
          path: '/plans',
          builder: (context, state) => const PlansScreen(),
        ),
        GoRoute(
          path: '/community',
          builder: (context, state) => const CommunityScreen(),
        ),
        GoRoute(
          path: '/profile',
          builder: (context, state) => const ProfileScreen(),
        ),
      ],
    ),
  ],
);

class _AppShell extends StatelessWidget {
  final Widget child;
  const _AppShell({required this.child});

  int _currentIndex(BuildContext context) {
    final location = GoRouterState.of(context).uri.toString();
    if (location.startsWith('/library')) return 1;
    if (location.startsWith('/plans')) return 2;
    if (location.startsWith('/community')) return 3;
    if (location.startsWith('/profile')) return 4;
    return 0;
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: child,
      bottomNavigationBar: BottomNavigationBar(
        currentIndex: _currentIndex(context),
        onTap: (i) {
          switch (i) {
            case 0:
              context.go('/home');
            case 1:
              context.go('/library');
            case 2:
              context.go('/plans');
            case 3:
              context.go('/community');
            case 4:
              context.go('/profile');
          }
        },
        items: const [
          BottomNavigationBarItem(
            icon: Icon(Icons.home_outlined),
            activeIcon: Icon(Icons.home_rounded),
            label: 'Home',
          ),
          BottomNavigationBarItem(
            icon: Icon(Icons.video_library_outlined),
            activeIcon: Icon(Icons.video_library_rounded),
            label: 'Library',
          ),
          BottomNavigationBarItem(
            icon: Icon(Icons.workspace_premium_outlined),
            activeIcon: Icon(Icons.workspace_premium_rounded),
            label: 'Plans',
          ),
          BottomNavigationBarItem(
            icon: Icon(Icons.forum_outlined),
            activeIcon: Icon(Icons.forum_rounded),
            label: 'Community',
          ),
          BottomNavigationBarItem(
            icon: Icon(Icons.person_outline_rounded),
            activeIcon: Icon(Icons.person_rounded),
            label: 'Profile',
          ),
        ],
      ),
    );
  }
}
