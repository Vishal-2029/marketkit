import 'package:shared_preferences/shared_preferences.dart';

/// The app runs in one of two modes chosen on first login: 'learning'
/// (videos, the classic shell) or 'market' (the product marketplace). The
/// choice is stored per user so a second account on the same device gets its
/// own chooser, and it can be flipped anytime from either profile screen.
class AppModeService {
  static const learning = 'learning';
  static const market = 'market';

  /// Learning kill-switch. While true the Learning side is locked behind
  /// "Coming soon…" everywhere (mode chooser, market profile) and users
  /// whose stored mode is learning land in the Product Market instead. Flip
  /// to false to re-open Learning — no other change needed.
  static const bool learningLocked = false;

  static String _key(String userId) => 'app_mode_$userId';

  /// Device-level (not user-scoped) flag: has the onboarding intro carousel
  /// already been shown on this device? Set once, on completing (or
  /// skipping) the carousel, and never cleared — even across logout/re-login.
  static const _onboardingSeenKey = 'has_seen_onboarding';

  /// Pre-registration only: the mode last picked on the chooser, so Login →
  /// Register can skip bouncing back through the chooser. Cleared once
  /// registration completes or the user deliberately returns to the chooser.
  static const _pendingModeKey = 'pending_app_mode';

  static Future<bool> hasSeenOnboarding() async {
    final prefs = await SharedPreferences.getInstance();
    return prefs.getBool(_onboardingSeenKey) ?? false;
  }

  static Future<void> setSeenOnboarding() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setBool(_onboardingSeenKey, true);
  }

  static Future<String?> getPendingMode() async {
    final prefs = await SharedPreferences.getInstance();
    return prefs.getString(_pendingModeKey);
  }

  static Future<void> setPendingMode(String mode) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_pendingModeKey, mode);
  }

  static Future<void> clearPendingMode() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.remove(_pendingModeKey);
  }

  /// Device-level (not user-scoped) copy of the last mode any user was in on
  /// this device. Used as a fallback when the per-user key isn't available
  /// yet (e.g. the user object hasn't loaded), so relaunch can still land in
  /// the right mode.
  static const _lastModeKey = 'last_app_mode';

  static Future<String?> getMode(String userId) async {
    final prefs = await SharedPreferences.getInstance();
    return prefs.getString(_key(userId));
  }

  /// The last mode used on this device, regardless of which account.
  static Future<String?> getLastMode() async {
    final prefs = await SharedPreferences.getInstance();
    return prefs.getString(_lastModeKey);
  }

  static Future<void> setMode(String userId, String mode) async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_key(userId), mode);
    // Also record it device-wide so relaunch has a fallback if the per-user
    // value is missing (fresh install / a wiped web localStorage).
    await prefs.setString(_lastModeKey, mode);
  }

  /// Resolves which mode to open for [userId] on login/relaunch, in priority
  /// order: the per-user local value, then the device-wide fallback, then the
  /// account's server-side [serverMode], then null (→ mode chooser for a
  /// brand-new account that has never picked one). Whatever is resolved is
  /// written back locally so subsequent reads are stable.
  static Future<String?> resolveMode(String userId, String? serverMode) async {
    var mode = await getMode(userId);
    mode ??= await getLastMode();
    if ((mode == null || mode.isEmpty) &&
        serverMode != null &&
        serverMode.isNotEmpty) {
      mode = serverMode;
    }
    if (mode != null && mode.isNotEmpty) {
      await setMode(userId, mode);
      return mode;
    }
    return null;
  }

  /// Route for a stored mode; null mode (first login) → the chooser.
  static String landingRoute(String? mode) {
    if (mode == null) return '/choose-mode';
    if (mode == learning && learningLocked) return '/market';
    return mode == market ? '/market' : '/home';
  }
}
