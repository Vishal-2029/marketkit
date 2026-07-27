import 'package:flutter/foundation.dart' show kIsWeb;
import 'package:flutter/material.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../../core/services/notification_service.dart';
import '../../core/theme/app_colors.dart';

/// SharedPreferences key for the millisecondsSinceEpoch of the last time the
/// user tapped "Not now" on the prompt. We re-prompt after [_snoozeDuration].
const _kPromptDismissedKey = 'notif_prompt_dismissed_at';

/// How long to wait before re-prompting after the user snoozes the dialog.
const _snoozeDuration = Duration(days: 3);

/// Checks whether OS notifications are disabled for this app and, if so,
/// shows a popup asking the user to enable them. Safe to call repeatedly —
/// it no-ops when notifications are already on, when the user recently
/// dismissed the prompt, or when the platform doesn't support runtime
/// notification permission.
///
/// Call from a `WidgetsBinding.instance.addPostFrameCallback` so the
/// dialog mounts on top of a fully built widget tree.
Future<void> maybeShowNotificationPermissionPrompt(
  BuildContext context,
) async {
  final enabled = await NotificationService().areNotificationsEnabled();
  if (enabled == null || enabled == true) return;

  final prefs = await SharedPreferences.getInstance();
  final dismissedAtMs = prefs.getInt(_kPromptDismissedKey);
  if (dismissedAtMs != null) {
    final dismissedAt = DateTime.fromMillisecondsSinceEpoch(dismissedAtMs);
    if (DateTime.now().difference(dismissedAt) < _snoozeDuration) return;
  }

  if (!context.mounted) return;
  await showDialog<void>(
    context: context,
    barrierDismissible: false,
    builder: (_) => const _NotificationPermissionDialog(),
  );
}

class _NotificationPermissionDialog extends StatefulWidget {
  const _NotificationPermissionDialog();

  @override
  State<_NotificationPermissionDialog> createState() =>
      _NotificationPermissionDialogState();
}

class _NotificationPermissionDialogState
    extends State<_NotificationPermissionDialog> {
  bool _busy = false;

  Future<void> _enable() async {
    if (_busy) return;
    setState(() => _busy = true);

    final granted = await NotificationService().requestPermission();

    if (!mounted) return;
    if (granted) {
      Navigator.of(context).pop();
      return;
    }

    // System refused (or permanently denied) — fall back to settings.
    await NotificationService().openNotificationSettings();
    if (!mounted) return;
    setState(() => _busy = false);
    Navigator.of(context).pop();
  }

  Future<void> _later() async {
    final prefs = await SharedPreferences.getInstance();
    await prefs.setInt(
      _kPromptDismissedKey,
      DateTime.now().millisecondsSinceEpoch,
    );
    if (!mounted) return;
    Navigator.of(context).pop();
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      backgroundColor: kCard,
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(20)),
      contentPadding: const EdgeInsets.fromLTRB(24, 28, 24, 16),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.center,
        children: [
          Container(
            width: 64,
            height: 64,
            decoration: BoxDecoration(
              gradient: kPrimaryGradient,
              shape: BoxShape.circle,
            ),
            alignment: Alignment.center,
            child: const Icon(
              Icons.notifications_active_rounded,
              color: Colors.white,
              size: 32,
            ),
          ),
          const SizedBox(height: 18),
          const Text(
            'Turn on notifications',
            textAlign: TextAlign.center,
            style: TextStyle(
              fontSize: 18,
              fontWeight: FontWeight.w700,
              color: kForeground,
            ),
          ),
          const SizedBox(height: 8),
          const Text(
            "Notifications are turned off. Enable them so you don't miss "
            'new tutorials, gallery updates, and important announcements.',
            textAlign: TextAlign.center,
            style: TextStyle(
              fontSize: 13.5,
              height: 1.45,
              color: kMutedForeground,
            ),
          ),
        ],
      ),
      actionsPadding: const EdgeInsets.fromLTRB(16, 0, 16, 16),
      actions: [
        TextButton(
          onPressed: _busy ? null : _later,
          child: const Text(
            'Not now',
            style: TextStyle(color: kMutedForeground, fontWeight: FontWeight.w600),
          ),
        ),
        ElevatedButton(
          onPressed: _busy ? null : _enable,
          style: ElevatedButton.styleFrom(
            backgroundColor: kPrimary,
            foregroundColor: Colors.white,
            padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 12),
            shape: RoundedRectangleBorder(
              borderRadius: BorderRadius.circular(10),
            ),
          ),
          child: _busy
              ? const SizedBox(
                  width: 16,
                  height: 16,
                  child: CircularProgressIndicator(
                    strokeWidth: 2,
                    color: Colors.white,
                  ),
                )
              : Text(kIsWeb ? 'Enable' : 'Turn on'),
        ),
      ],
    );
  }
}
