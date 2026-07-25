import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../core/services/app_mode_service.dart';
import '../../../core/services/update_service.dart';
import '../../../shared/widgets/auth_animated_background.dart';
import '../../../shared/widgets/update_dialog.dart'; // still needed for mandatory updates
import '../providers/auth_provider.dart';
import '../widgets/auth_widgets.dart';

class SplashScreen extends ConsumerStatefulWidget {
  const SplashScreen({super.key});

  @override
  ConsumerState<SplashScreen> createState() => _SplashScreenState();
}

class _SplashScreenState extends ConsumerState<SplashScreen>
    with SingleTickerProviderStateMixin {
  late final AnimationController _ctrl;
  late final Animation<double> _scale;
  late final Animation<double> _fade;

  @override
  void initState() {
    super.initState();

    _ctrl = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 900),
    );

    _scale = Tween<double>(begin: 0.7, end: 1.0).animate(
      CurvedAnimation(parent: _ctrl, curve: Curves.easeOutBack),
    );

    _fade = Tween<double>(begin: 0.0, end: 1.0).animate(
      CurvedAnimation(
        parent: _ctrl,
        curve: const Interval(0.0, 0.6, curve: Curves.easeIn),
      ),
    );

    _ctrl.forward();

    WidgetsBinding.instance.addPostFrameCallback((_) => _init());
  }

  Future<void> _init() async {
    var loggedIn = false;

    try {
      final results = await Future.wait([
        Future.delayed(const Duration(milliseconds: 1200)),
        ref.read(authProvider.notifier).tryRefresh().timeout(
          const Duration(seconds: 5),
          onTimeout: () => false,
        ),
      ]);
      loggedIn = results[1] as bool;
    } catch (_) {
      loggedIn = false;
    }

    if (!mounted) return;

    // Silently check for an available update (never throws).
    final update = await UpdateService.instance.checkForUpdate();
    if (!mounted) return;

    if (update != null && update.isMandatory) {
      // Block navigation — user must install before using the app.
      await showDialog(
        context: context,
        barrierDismissible: false,
        builder: (_) => UpdateDialog(info: update),
      );
      return; // OS takes over after installApk() is called.
    }

    if (!loggedIn) {
      final seenOnboarding = await AppModeService.hasSeenOnboarding();
      if (!mounted) return;
      context.go(seenOnboarding ? '/login' : '/onboarding');
      return;
    }
    // Open the mode the user chose on first login. No locally-cached mode
    // can mean either a legacy account that predates the chooser, or a fresh
    // install/new device for an account that already has a server-side mode
    // — prefer the server's value before falling back to the chooser.
    final user = ref.read(authProvider).user;
    final mode = user == null
        ? null
        : await AppModeService.resolveMode(user.id, user.currentAppMode);
    if (!mounted) return;
    context.go(AppModeService.landingRoute(mode));
    // Optional updates are surfaced via the Profile screen — no popup here.
  }

  @override
  void dispose() {
    _ctrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: AuthAnimatedBackground(
        child: Center(
          child: FadeTransition(
            opacity: _fade,
            child: ScaleTransition(
              scale: _scale,
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  const AuthLogo(
                    size: 190,
                    titleFontSize: 26,
                    titleColor: Color(0xFF7A2E2E),
                  ),
                  const SizedBox(height: 32),
                  const SizedBox(
                    width: 28,
                    height: 28,
                    child: CircularProgressIndicator(
                      color: Color(0xFF7A2E2E),
                      strokeWidth: 2.5,
                    ),
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}
