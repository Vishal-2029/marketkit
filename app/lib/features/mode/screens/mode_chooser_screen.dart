import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../core/services/app_mode_service.dart';
import '../../../core/theme/app_colors.dart';
import '../../../shared/widgets/auth_animated_background.dart';
import '../../auth/providers/auth_provider.dart';

/// Reached in two different contexts:
///  - Pre-registration (no logged-in user yet — from onboarding, or the
///    login screen's "Register" link for a second account on this device):
///    the chosen mode is carried forward to the register form; nothing is
///    persisted here since there's no account yet.
///  - Legacy post-login (an existing account from before mode selection
///    moved to signup time, with no stored mode): behaves as before —
///    persists the mode and lands straight in the dashboard.
class ModeChooserScreen extends ConsumerWidget {
  const ModeChooserScreen({super.key});

  Future<void> _choose(BuildContext context, WidgetRef ref, String mode) async {
    final userId = ref.read(authProvider).user?.id;

    if (userId == null) {
      // Pre-registration — cache pending mode and carry to the register form.
      await AppModeService.setPendingMode(mode);
      if (!context.mounted) return;
      context.push('/register', extra: {'mode': mode});
      return;
    }

    // Legacy post-login: persist locally and server-side, then land in the
    // dashboard for the chosen mode.
    await AppModeService.setMode(userId, mode);
    try {
      await ref.read(authProvider.notifier).setAppMode(mode);
    } catch (_) {
      // Best-effort — local state already updated, don't block navigation.
    }
    if (!context.mounted) return;
    context.go(mode == AppModeService.market ? '/market' : '/home');
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return Scaffold(
      body: AuthAnimatedBackground(
        child: SafeArea(
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 24),
            child: Column(
              children: [
                const Spacer(),
                const Text(
                  'What brings you here?',
                  textAlign: TextAlign.center,
                  style: TextStyle(
                    color: kForeground,
                    fontSize: 24,
                    fontWeight: FontWeight.w700,
                  ),
                ),
                const SizedBox(height: 8),
                const Text(
                  'You can switch anytime from your profile.',
                  textAlign: TextAlign.center,
                  style: TextStyle(color: kMutedForeground, fontSize: 14),
                ),
                const SizedBox(height: 32),
                _ModeCard(
                  icon: Icons.play_circle_outline_rounded,
                  title: 'Learning',
                  subtitle: AppModeService.learningLocked
                      ? 'Coming soon…'
                      : 'Watch embroidery courses and tutorials',
                  locked: AppModeService.learningLocked,
                  onTap: AppModeService.learningLocked
                      ? () => ScaffoldMessenger.of(context).showSnackBar(
                            const SnackBar(
                              content:
                                  Text('Learning is coming soon — stay tuned!'),
                            ),
                          )
                      : () => _choose(context, ref, AppModeService.learning),
                ),
                const SizedBox(height: 20),
                _ModeCard(
                  icon: Icons.storefront_outlined,
                  title: 'Product Market',
                  subtitle: 'Buy and sell embroidery products',
                  onTap: () => _choose(context, ref, AppModeService.market),
                ),
                const Spacer(flex: 2),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class _ModeCard extends StatelessWidget {
  final IconData icon;
  final String title;
  final String subtitle;
  final VoidCallback onTap;

  /// Locked cards render dimmed with a lock badge and a gold subtitle
  /// ("Coming soon…"); tapping them explains instead of navigating.
  final bool locked;

  const _ModeCard({
    required this.icon,
    required this.title,
    required this.subtitle,
    required this.onTap,
    this.locked = false,
  });

  @override
  Widget build(BuildContext context) {
    return Material(
      color: locked ? Colors.white.withValues(alpha: 0.85) : Colors.white,
      borderRadius: BorderRadius.circular(20),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(20),
        child: Container(
          width: double.infinity,
          padding: const EdgeInsets.symmetric(vertical: 32, horizontal: 20),
          child: Column(
            children: [
              Stack(
                clipBehavior: Clip.none,
                children: [
                  Container(
                    width: 64,
                    height: 64,
                    decoration: BoxDecoration(
                      color: kGold.withValues(alpha: locked ? 0.08 : 0.12),
                      shape: BoxShape.circle,
                    ),
                    child: Icon(
                      icon,
                      size: 34,
                      color: locked ? kMutedForeground : kGold,
                    ),
                  ),
                  if (locked)
                    Positioned(
                      right: -4,
                      bottom: -4,
                      child: Container(
                        padding: const EdgeInsets.all(5),
                        decoration: const BoxDecoration(
                          color: kGold,
                          shape: BoxShape.circle,
                        ),
                        child: const Icon(
                          Icons.lock_rounded,
                          size: 14,
                          color: Colors.white,
                        ),
                      ),
                    ),
                ],
              ),
              const SizedBox(height: 14),
              Text(
                title,
                style: TextStyle(
                  fontSize: 18,
                  fontWeight: FontWeight.w700,
                  color: locked ? kMutedForeground : kForeground,
                ),
              ),
              const SizedBox(height: 4),
              Text(
                subtitle,
                textAlign: TextAlign.center,
                style: TextStyle(
                  fontSize: 13,
                  color: locked ? kGold : kMutedForeground,
                  fontWeight: locked ? FontWeight.w600 : FontWeight.w400,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
