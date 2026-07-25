import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:open_filex/open_filex.dart';
import 'package:package_info_plus/package_info_plus.dart';
import 'package:url_launcher/url_launcher.dart';
import '../../../core/services/app_mode_service.dart';
import '../../../core/services/update_service.dart';
import '../../../core/theme/app_colors.dart';
import '../../../shared/widgets/avatar_upload_controller.dart';
import '../../../shared/widgets/menu_card.dart';
import '../../../shared/widgets/profile_header.dart';
import '../../../shared/widgets/update_dialog.dart';
import '../../auth/providers/auth_provider.dart';
import '../../market_plans/models/market_plan_model.dart';
import '../../market_plans/providers/market_plans_provider.dart';
import '../models/purchase_model.dart';
import '../providers/products_provider.dart';
import '../providers/my_market_provider.dart';
import '../providers/wallet_provider.dart';
import '../widgets/add_money_sheet.dart';
import '../widgets/withdraw_sheet.dart';

class MarketProfileTab extends ConsumerStatefulWidget {
  final VoidCallback? onViewPlans;

  const MarketProfileTab({super.key, this.onViewPlans});

  @override
  ConsumerState<MarketProfileTab> createState() => _MarketProfileTabState();
}

class _MarketProfileTabState extends ConsumerState<MarketProfileTab> {
  @override
  void initState() {
    super.initState();
    Future.microtask(() => ref.read(marketPlansProvider.notifier).load());
  }

  Future<void> _switchToLearning() async {
    if (AppModeService.learningLocked) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Learning is coming soon — stay tuned!')),
      );
      return;
    }
    final userId = ref.read(authProvider).user?.id;
    if (userId != null) {
      await AppModeService.setMode(userId, AppModeService.learning);
    }
    try {
      await ref.read(authProvider.notifier).setAppMode(AppModeService.learning);
    } catch (_) {
      // Best-effort — local state already updated, don't block navigation.
    }
    if (!mounted) return;
    context.go('/home');
  }

  Future<void> _confirmCancelPlan() async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Cancel plan?'),
        content: const Text(
          'Your market plan will end immediately. No refund will be issued.',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: const Text('Keep plan'),
          ),
          TextButton(
            onPressed: () => Navigator.pop(ctx, true),
            style: TextButton.styleFrom(foregroundColor: kTerracotta),
            child: const Text('Cancel plan'),
          ),
        ],
      ),
    );
    if (confirmed != true || !mounted) return;

    try {
      await ref.read(marketPlansProvider.notifier).cancelMyPlan();
      await ref.read(marketPlansProvider.notifier).load();
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text('Market plan cancelled'),
          backgroundColor: kSage,
        ),
      );
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text('Could not cancel plan: $e'),
          backgroundColor: kTerracotta,
        ),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    final wallet = ref.watch(walletSummaryProvider);
    final earnings = ref.watch(earningsProvider);
    final myProducts = ref.watch(myProductsProvider);
    final myPurchases = ref.watch(myPurchasesProvider);
    final marketPlans = ref.watch(marketPlansProvider);
    final mySub = marketPlans.mySubscription;

    return RefreshIndicator(
      color: kGold,
      onRefresh: () async {
        ref.invalidate(walletSummaryProvider);
        ref.invalidate(earningsProvider);
        ref.invalidate(myProductsProvider);
        ref.invalidate(myPurchasesProvider);
        await ref.read(marketPlansProvider.notifier).load();
      },
      child: ListView(
        padding: const EdgeInsets.fromLTRB(16, 12, 16, 100),
        children: [
          const SizedBox(height: 4),
          const _MarketProfileHeader(),
          const SizedBox(height: 24),

          // Wallet card
          Container(
            padding: const EdgeInsets.all(16),
            decoration: BoxDecoration(
              gradient: kGoldGradient,
              borderRadius: BorderRadius.circular(14),
            ),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    const Text(
                      'Wallet Balance',
                      style: TextStyle(fontSize: 13, color: Colors.white70),
                    ),
                    const Spacer(),
                    earnings.when(
                      data: (e) => Text(
                        'Earned ${e.formattedTotal} · ${e.totalSales} sale${e.totalSales == 1 ? '' : 's'}',
                        style: const TextStyle(
                            fontSize: 11, color: Colors.white70),
                      ),
                      loading: () => const SizedBox.shrink(),
                      error: (_, __) => const SizedBox.shrink(),
                    ),
                  ],
                ),
                const SizedBox(height: 4),
                wallet.when(
                  data: (w) => Text(
                    w.formattedBalance,
                    style: const TextStyle(
                        fontSize: 28,
                        fontWeight: FontWeight.w700,
                        color: Colors.white),
                  ),
                  loading: () => const SizedBox(
                    height: 34,
                    width: 34,
                    child: CircularProgressIndicator(
                        color: Colors.white, strokeWidth: 2),
                  ),
                  error: (_, __) => const Text('—',
                      style: TextStyle(fontSize: 28, color: Colors.white)),
                ),
                const SizedBox(height: 14),
                Row(
                  children: [
                    _WalletAction(
                      icon: Icons.add_rounded,
                      label: 'Add Money',
                      onTap: () => AddMoneySheet.show(context),
                    ),
                    const SizedBox(width: 8),
                    _WalletAction(
                      icon: Icons.arrow_outward_rounded,
                      label: 'Withdraw',
                      onTap: () {
                        final w = wallet.valueOrNull;
                        if (w == null) return;
                        WithdrawSheet.show(context, w);
                      },
                    ),
                    const SizedBox(width: 8),
                    _WalletAction(
                      icon: Icons.receipt_long_outlined,
                      label: 'History',
                      onTap: () =>
                          context.push('/market/wallet/transactions'),
                    ),
                  ],
                ),
              ],
            ),
          ),
          const SizedBox(height: 20),

          _MarketPlanCard(
            subscription: mySub,
            isProcessing: marketPlans.isPaymentProcessing,
            onViewPlans: widget.onViewPlans,
            onCancel: _confirmCancelPlan,
          ),
          const SizedBox(height: 20),

          // Account
          const SectionHeader(title: 'ACCOUNT'),
          MenuCard(children: [
            MenuTile(
              icon: Icons.account_balance_outlined,
              label: 'Payout details (UPI / bank)',
              onTap: () => context.push('/market/wallet/payout-details'),
            ),
            const MenuDivider(),
            MenuTile(
              icon: AppModeService.learningLocked
                  ? Icons.lock_outline_rounded
                  : Icons.play_circle_outline_rounded,
              label: AppModeService.learningLocked
                  ? 'Learning — Coming soon…'
                  : 'Switch to Learning',
              onTap: _switchToLearning,
            ),
          ]),
          const SizedBox(height: 20),

          // My purchases
          const SectionHeader(title: 'MY PURCHASES'),
          myPurchases.when(
            data: (purchases) => purchases.isEmpty
                ? const _EmptyRow(text: 'No purchases yet.')
                : Column(
                    children: [
                      for (final p in purchases)
                        _PurchaseTile(purchase: p),
                    ],
                  ),
            loading: () => const _LoadingRow(),
            error: (_, __) =>
                const _EmptyRow(text: 'Could not load purchases.'),
          ),
          const SizedBox(height: 20),

          // My products — dedicated analytics screen
          GestureDetector(
            onTap: () => context.push('/market/my-products'),
            behavior: HitTestBehavior.opaque,
            child: const Padding(
              padding: EdgeInsets.symmetric(vertical: 4),
              child: Row(
                children: [
                  Expanded(child: SectionHeader(title: 'MY PRODUCTS')),
                  Icon(Icons.chevron_right_rounded, color: kMutedForeground),
                ],
              ),
            ),
          ),
          myProducts.when(
            data: (products) => products.isEmpty
                ? const _EmptyRow(
                    text: 'No products listed yet. Upload one to start selling!')
                : _EmptyRow(
                    text:
                        '${products.length} product${products.length == 1 ? '' : 's'} · tap to view sales & views',
                  ),
            loading: () => const _LoadingRow(),
            error: (_, __) => const _EmptyRow(text: 'Could not load products.'),
          ),
          const SizedBox(height: 20),

          // About (same update flow as the Learning profile)
          const SectionHeader(title: 'ABOUT'),
          MenuCard(children: [
            if (UpdateService.instance.latestUpdate != null)
              MenuTile(
                icon: Icons.system_update_rounded,
                iconColor: kGold,
                label: 'Update Available',
                trailing: Container(
                  padding:
                      const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                  decoration: BoxDecoration(
                    color: kGold.withValues(alpha: 0.12),
                    borderRadius: BorderRadius.circular(20),
                  ),
                  child: Text(
                    'v${UpdateService.instance.latestUpdate!.versionName}',
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
                      UpdateDialog(info: UpdateService.instance.latestUpdate!),
                ),
              )
            else
              MenuTile(
                icon: Icons.info_outline_rounded,
                label: 'App Version',
                trailing: FutureBuilder<PackageInfo>(
                  future: PackageInfo.fromPlatform(),
                  builder: (context, snapshot) => Text(
                    snapshot.hasData ? 'v${snapshot.data!.version}' : '',
                    style: const TextStyle(
                        fontSize: 13, color: kMutedForeground),
                  ),
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
                style: TextStyle(fontSize: 15, fontWeight: FontWeight.w600),
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
    );
  }
}

class _MarketPlanCard extends StatelessWidget {
  final MarketPlanSubscriptionModel? subscription;
  final bool isProcessing;
  final VoidCallback? onViewPlans;
  final VoidCallback onCancel;

  const _MarketPlanCard({
    required this.subscription,
    required this.isProcessing,
    required this.onViewPlans,
    required this.onCancel,
  });

  @override
  Widget build(BuildContext context) {
    final sub = subscription;
    final active = sub != null && sub.isActive;

    if (!active) {
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
              color: Colors.black.withValues(alpha: 0.04),
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
                'No active plan',
                style: TextStyle(
                  fontSize: 14,
                  fontWeight: FontWeight.w500,
                  color: kMutedForeground,
                ),
              ),
            ),
            TextButton(
              onPressed: onViewPlans,
              child: const Text('View Plans', style: TextStyle(color: kGold)),
            ),
          ],
        ),
      );
    }

    final planName = sub.plan?.name ?? 'Market Plan';
    final expiry = sub.expiryDate;

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
            color: Colors.black.withValues(alpha: 0.04),
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
                  planName,
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
                  color: kSage.withValues(alpha: 0.12),
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
          if (expiry != null) ...[
            const SizedBox(height: 6),
            Text(
              'Valid until ${_formatDate(expiry)}',
              style: const TextStyle(fontSize: 12, color: kMutedForeground),
            ),
          ],
          const SizedBox(height: 8),
          Align(
            alignment: Alignment.centerRight,
            child: TextButton(
              onPressed: isProcessing ? null : onCancel,
              style: TextButton.styleFrom(foregroundColor: kTerracotta),
              child: isProcessing
                  ? const SizedBox(
                      width: 16,
                      height: 16,
                      child: CircularProgressIndicator(strokeWidth: 2),
                    )
                  : const Text('Cancel plan'),
            ),
          ),
        ],
      ),
    );
  }

  static String _formatDate(DateTime d) {
    const months = [
      'Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun',
      'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'
    ];
    return '${d.day.toString().padLeft(2, '0')} ${months[d.month - 1]} ${d.year}';
  }
}

class _PurchaseTile extends ConsumerWidget {
  final PurchaseModel purchase;
  const _PurchaseTile({required this.purchase});

  Future<void> _download(BuildContext context, WidgetRef ref) async {
    try {
      final data = await ref
          .read(marketServiceProvider)
          .fetchDownloadUrl(purchase.productId);
      final url = data['url'] as String?;
      if (url == null) throw Exception('no url');
      await launchUrl(Uri.parse(url), mode: LaunchMode.externalApplication);
    } catch (_) {
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Download failed. Please try again.')),
        );
      }
    }
  }

  Future<void> _downloadInvoice(BuildContext context, WidgetRef ref) async {
    try {
      final path =
          await ref.read(marketServiceProvider).downloadInvoice(purchase.id);
      await OpenFilex.open(path, type: 'application/pdf');
    } catch (_) {
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
              content: Text('Could not open invoice. Please try again.')),
        );
      }
    }
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final product = purchase.product;
    return Container(
      margin: const EdgeInsets.only(bottom: 8),
      padding: const EdgeInsets.all(10),
      decoration: BoxDecoration(
        color: kCard,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: kBorder),
      ),
      child: Row(
        children: [
          ClipRRect(
            borderRadius: BorderRadius.circular(8),
            child: product != null && product.previewUrls.isNotEmpty
                ? CachedNetworkImage(
                    imageUrl: product.previewUrls.first,
                    width: 48,
                    height: 48,
                    fit: BoxFit.cover,
                  )
                : Container(
                    width: 48,
                    height: 48,
                    color: kMuted,
                    child: const Icon(Icons.design_services_outlined,
                        size: 20, color: kMutedForeground),
                  ),
          ),
          const SizedBox(width: 10),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  product?.title ?? 'Product',
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(
                      fontSize: 13,
                      fontWeight: FontWeight.w600,
                      color: kForeground),
                ),
                const SizedBox(height: 2),
                Text(
                  'Paid ${purchase.formattedAmount}',
                  style: const TextStyle(
                      fontSize: 11, color: kMutedForeground),
                ),
              ],
            ),
          ),
          IconButton(
            onPressed: () => _download(context, ref),
            icon: const Icon(Icons.download_rounded, size: 18),
            color: kGold,
            tooltip: 'Download product file',
            visualDensity: VisualDensity.compact,
          ),
          IconButton(
            onPressed: () => _downloadInvoice(context, ref),
            icon: const Icon(Icons.receipt_long_outlined, size: 18),
            color: kGold,
            tooltip: 'Download invoice',
            visualDensity: VisualDensity.compact,
          ),
        ],
      ),
    );
  }
}

class _EmptyRow extends StatelessWidget {
  final String text;
  const _EmptyRow({required this.text});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: kCard,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: kBorder),
      ),
      child: Text(text,
          style: const TextStyle(fontSize: 13, color: kMutedForeground)),
    );
  }
}

class _WalletAction extends StatelessWidget {
  final IconData icon;
  final String label;
  final VoidCallback onTap;

  const _WalletAction({
    required this.icon,
    required this.label,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return Expanded(
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(10),
        child: Container(
          padding: const EdgeInsets.symmetric(vertical: 9),
          decoration: BoxDecoration(
            color: Colors.white.withValues(alpha: 0.18),
            borderRadius: BorderRadius.circular(10),
          ),
          child: Column(
            children: [
              Icon(icon, size: 18, color: Colors.white),
              const SizedBox(height: 3),
              Text(
                label,
                style: const TextStyle(
                    fontSize: 11,
                    fontWeight: FontWeight.w600,
                    color: Colors.white),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _LoadingRow extends StatelessWidget {
  const _LoadingRow();

  @override
  Widget build(BuildContext context) {
    return const Padding(
      padding: EdgeInsets.symmetric(vertical: 24),
      child: Center(
        child: CircularProgressIndicator(color: kGold, strokeWidth: 2),
      ),
    );
  }
}

/// Profile header with avatar upload — same flow as Learning ProfileScreen.
class _MarketProfileHeader extends ConsumerStatefulWidget {
  const _MarketProfileHeader();

  @override
  ConsumerState<_MarketProfileHeader> createState() =>
      _MarketProfileHeaderState();
}

class _MarketProfileHeaderState extends ConsumerState<_MarketProfileHeader>
    with AvatarUploadController {
  @override
  Widget build(BuildContext context) {
    final user = ref.watch(authProvider).user;
    return ProfileHeader(
      onAvatarTap: () => showAvatarOptions(user?.avatarUrl != null),
      busy: avatarBusy,
      showCameraBadge: true,
    );
  }
}
