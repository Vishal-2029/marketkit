import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:package_info_plus/package_info_plus.dart';
import 'package:shared_preferences/shared_preferences.dart';
import 'package:url_launcher/url_launcher.dart';
import '../../../core/services/app_mode_service.dart';
import '../../../core/services/update_service.dart';
import '../../../core/theme/app_colors.dart';
import '../../../shared/widgets/avatar_upload_controller.dart';
import '../../../shared/widgets/menu_card.dart';
import '../../../shared/widgets/profile_header.dart';
import '../../../shared/widgets/update_dialog.dart';
import '../../auth/models/subscription_model.dart';
import '../../auth/providers/auth_provider.dart';

class ProfileScreen extends ConsumerStatefulWidget {
  const ProfileScreen({super.key});

  @override
  ConsumerState<ProfileScreen> createState() => _ProfileScreenState();
}

class _ProfileScreenState extends ConsumerState<ProfileScreen>
    with AvatarUploadController {
  bool _notificationsEnabled = true;
  String _appVersion = '';
  AppVersionInfo? _availableUpdate;

  @override
  void initState() {
    super.initState();
    _loadPrefs();
    _loadVersion();
  }

  Future<void> _loadPrefs() async {
    final prefs = await SharedPreferences.getInstance();
    if (mounted) {
      setState(() {
        _notificationsEnabled = prefs.getBool('notifications_enabled') ?? true;
      });
    }
  }

  Future<void> _loadVersion() async {
    final info = await PackageInfo.fromPlatform();
    if (mounted) {
      setState(() {
        _appVersion = 'v${info.version}';
        _availableUpdate = UpdateService.instance.latestUpdate;
      });
    }
  }

  Future<void> _toggleNotifications(bool value) async {
    setState(() => _notificationsEnabled = value);
    final prefs = await SharedPreferences.getInstance();
    await prefs.setBool('notifications_enabled', value);
  }

  Future<void> _launchUrl(String url) async {
    final uri = Uri.parse(url);
    if (!await launchUrl(uri, mode: LaunchMode.externalApplication)) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Could not open link')),
        );
      }
    }
  }

  Future<void> _switchToMarket() async {
    final userId = ref.read(authProvider).user?.id;
    if (userId != null) {
      await AppModeService.setMode(userId, AppModeService.market);
    }
    try {
      await ref.read(authProvider.notifier).setAppMode(AppModeService.market);
    } catch (_) {
      // Best-effort — local state already updated, don't block navigation.
    }
    if (!mounted) return;
    context.go('/market');
  }

  Future<void> _confirmDeleteAccount() async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Delete Account'),
        content: const Text('This will permanently delete your account from the app. Are you sure?'),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: const Text('Cancel'),
          ),
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(true),
            child: const Text('Delete', style: TextStyle(color: Colors.red)),
          ),
        ],
      ),
    );

    if (confirmed != true) return;

    try {
      await ref.read(authProvider.notifier).deleteAccount();
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Account deleted successfully')),
        );
        context.go('/login');
      }
    } catch (e) {
      if (!mounted) return;
      var message = 'Failed to delete account';
      try {
        final data = (e as dynamic).response?.data;
        if (data is Map && data['error'] != null) {
          message = data['error'] as String;
        }
      } catch (_) {}
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(content: Text(message)),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    final auth = ref.watch(authProvider);
    final user = auth.user;
    final activeSubs = user?.activeSubscriptions ?? [];

    return Scaffold(
      backgroundColor: kBackground,
      appBar: AppBar(
        backgroundColor: kBackground,
        title: const Text(
          'Profile',
          style: TextStyle(fontWeight: FontWeight.w700, fontSize: 18),
        ),
        centerTitle: false,
        elevation: 0,
      ),
      body: SafeArea(
        child: SingleChildScrollView(
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.center,
            children: [
              const SizedBox(height: 16),
              ProfileHeader(
                onAvatarTap: () => showAvatarOptions(user?.avatarUrl != null),
                busy: avatarBusy,
                showCameraBadge: true,
              ),
              const SizedBox(height: 24),

              _SubscriptionCard(subscriptions: activeSubs),
              const SizedBox(height: 20),

              // Account
              const SectionHeader(title: 'ACCOUNT'),
              MenuCard(children: [
                MenuTile(
                  icon: Icons.person_outline_rounded,
                  label: 'Edit Profile',
                  onTap: () => context.push('/profile/edit'),
                ),
                const MenuDivider(),
                MenuTile(
                  icon: Icons.lock_outline_rounded,
                  label: 'Change Password',
                  onTap: () => context.push('/profile/change-password'),
                ),
                const MenuDivider(),
                MenuTile(
                  icon: Icons.download_for_offline_outlined,
                  label: 'My Downloads',
                  onTap: () => context.push('/downloads'),
                ),
                const MenuDivider(),
                MenuTile(
                  icon: Icons.star_outline_rounded,
                  label: 'My Favorites',
                  onTap: () => context.push('/favorites'),
                ),
                const MenuDivider(),
                MenuTile(
                  icon: Icons.storefront_outlined,
                  label: 'Design Market',
                  onTap: _switchToMarket,
                ),
                const MenuDivider(),
                MenuTile(
                  icon: Icons.delete_outline_rounded,
                  iconColor: Colors.red,
                  label: 'Delete Account',
                  onTap: _confirmDeleteAccount,
                ),
              ]),
              const SizedBox(height: 16),

              // Preferences
              const SectionHeader(title: 'PREFERENCES'),
              MenuCard(children: [
                SwitchListTile(
                  value: _notificationsEnabled,
                  onChanged: _toggleNotifications,
                  title: const Text(
                    'Push Notifications',
                    style: TextStyle(fontSize: 14, color: kForeground),
                  ),
                  secondary: const Icon(
                    Icons.notifications_outlined,
                    color: kGold,
                    size: 20,
                  ),
                  activeColor: kGold,
                  contentPadding:
                      const EdgeInsets.symmetric(horizontal: 16, vertical: 4),
                ),
              ]),
              const SizedBox(height: 16),

              // About
              const SectionHeader(title: 'ABOUT'),
              MenuCard(children: [
                MenuTile(
                  icon: Icons.help_outline_rounded,
                  label: 'Help & Support',
                  onTap: () => _launchUrl('mailto:support@stitchcraft.app'),
                ),
                const MenuDivider(),
                MenuTile(
                  icon: Icons.policy_outlined,
                  label: 'Terms & Privacy Policy',
                  onTap: () => _launchUrl('https://stitchcraft.app/privacy'),
                ),
                const MenuDivider(),
                if (_availableUpdate != null)
                  MenuTile(
                    icon: Icons.system_update_rounded,
                    iconColor: kGold,
                    label: 'Update Available',
                    trailing: Container(
                      padding: const EdgeInsets.symmetric(
                          horizontal: 8, vertical: 3),
                      decoration: BoxDecoration(
                        color: kGold.withValues(alpha: 0.12),
                        borderRadius: BorderRadius.circular(20),
                      ),
                      child: Text(
                        'v${_availableUpdate!.versionName}',
                        style: const TextStyle(
                          fontSize: 11,
                          fontWeight: FontWeight.w600,
                          color: kGold,
                        ),
                      ),
                    ),
                    onTap: () => showDialog(
                      context: context,
                      builder: (_) =>
                          UpdateDialog(info: _availableUpdate!),
                    ),
                  )
                else
                  MenuTile(
                    icon: Icons.info_outline_rounded,
                    label: 'App Version',
                    trailing: Text(
                      _appVersion,
                      style: const TextStyle(
                          fontSize: 13, color: kMutedForeground),
                    ),
                    onTap: null,
                  ),
              ]),
              const SizedBox(height: 24),

              SizedBox(
                width: double.infinity,
                child: OutlinedButton.icon(
                  style: OutlinedButton.styleFrom(
                    foregroundColor: kTerracotta,
                    side: const BorderSide(color: kTerracotta),
                    padding: const EdgeInsets.symmetric(vertical: 14),
                    shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(10),
                    ),
                  ),
                  icon: const Icon(Icons.logout_rounded),
                  label: const Text(
                    'Sign Out',
                    style:
                        TextStyle(fontSize: 15, fontWeight: FontWeight.w600),
                  ),
                  onPressed: () async {
                    await ref.read(authProvider.notifier).logout();
                    if (context.mounted) context.go('/login');
                  },
                ),
              ),
              const SizedBox(height: 16),
            ],
          ),
        ),
      ),
    );
  }
}

class _SubscriptionCard extends StatelessWidget {
  final List<SubscriptionModel> subscriptions;
  const _SubscriptionCard({required this.subscriptions});

  @override
  Widget build(BuildContext context) {
    if (subscriptions.isEmpty) {
      return Container(
        width: double.infinity,
        padding: const EdgeInsets.all(16),
        decoration: BoxDecoration(
          color: kCard,
          borderRadius: BorderRadius.circular(14),
          border: const Border(
            left: BorderSide(color: kMutedForeground, width: 4),
          ),
          boxShadow: [
            BoxShadow(
              color: Colors.black.withOpacity(0.04),
              blurRadius: 6,
              offset: const Offset(0, 2),
            ),
          ],
        ),
        child: Row(
          children: [
            const Icon(Icons.workspace_premium_outlined,
                color: kMutedForeground, size: 20),
            const SizedBox(width: 10),
            const Expanded(
              child: Text(
                'No active subscription',
                style: TextStyle(
                  fontSize: 14,
                  fontWeight: FontWeight.w500,
                  color: kMutedForeground,
                ),
              ),
            ),
            TextButton(
              onPressed: () => context.go('/plans'),
              child: const Text('View Plans', style: TextStyle(color: kGold)),
            ),
          ],
        ),
      );
    }

    return Column(
      children: [
        for (var i = 0; i < subscriptions.length; i++) ...[
          if (i > 0) const SizedBox(height: 12),
          _SingleSubscriptionCard(subscription: subscriptions[i]),
        ],
      ],
    );
  }
}

class _SingleSubscriptionCard extends StatelessWidget {
  final SubscriptionModel subscription;
  const _SingleSubscriptionCard({required this.subscription});

  @override
  Widget build(BuildContext context) {
    final sub = subscription;

    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: kCard,
        borderRadius: BorderRadius.circular(14),
        border: const Border(
          left: BorderSide(color: kSage, width: 4),
        ),
        boxShadow: [
          BoxShadow(
            color: Colors.black.withOpacity(0.04),
            blurRadius: 6,
            offset: const Offset(0, 2),
          ),
        ],
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              const Icon(Icons.workspace_premium_rounded,
                  color: kSage, size: 20),
              const SizedBox(width: 8),
              Expanded(
                child: Text(
                  sub.planName,
                  style: const TextStyle(
                    fontSize: 15,
                    fontWeight: FontWeight.w700,
                    color: kForeground,
                  ),
                ),
              ),
              Container(
                padding:
                    const EdgeInsets.symmetric(horizontal: 10, vertical: 3),
                decoration: BoxDecoration(
                  color: kSage.withOpacity(0.12),
                  borderRadius: BorderRadius.circular(20),
                ),
                child: const Text(
                  'Active',
                  style: TextStyle(
                    fontSize: 11,
                    fontWeight: FontWeight.w600,
                    color: kSage,
                  ),
                ),
              ),
            ],
          ),
          if (sub.expiresAt != null) ...[
            const SizedBox(height: 6),
            Text(
              'Valid until ${_formatDate(sub.expiresAt!)}',
              style: const TextStyle(fontSize: 12, color: kMutedForeground),
            ),
          ],
          const SizedBox(height: 12),
          Row(
            children: [
              _FeatureBadge(label: 'Wilcom 2006', enabled: sub.hasWillcom),
              const SizedBox(width: 8),
              _FeatureBadge(label: 'E4', enabled: sub.hasE4),
              const SizedBox(width: 8),
              _FeatureBadge(label: 'meCAD', enabled: sub.hasMecad),
            ],
          ),
        ],
      ),
    );
  }

  String _formatDate(DateTime d) {
    const months = [
      'Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun',
      'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'
    ];
    return '${d.day.toString().padLeft(2, '0')} ${months[d.month - 1]} ${d.year}';
  }
}

class _FeatureBadge extends StatelessWidget {
  final String label;
  final bool enabled;

  const _FeatureBadge({required this.label, required this.enabled});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
      decoration: BoxDecoration(
        color: enabled ? kSage.withOpacity(0.12) : kMuted,
        borderRadius: BorderRadius.circular(6),
      ),
      child: Text(
        label,
        style: TextStyle(
          fontSize: 11,
          fontWeight: FontWeight.w600,
          color: enabled ? kSage : kMutedForeground,
        ),
      ),
    );
  }
}
