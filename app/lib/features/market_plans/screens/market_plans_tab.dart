import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shimmer/shimmer.dart';
import '../../../core/theme/app_colors.dart';
import '../../auth/providers/auth_provider.dart';
import '../../marketplace/models/wallet_model.dart';
import '../../marketplace/providers/wallet_provider.dart';
import '../models/market_plan_model.dart';
import '../providers/market_plans_provider.dart';
import '../widgets/market_plan_card.dart';
import 'package:marketkit/core/payments/checkout.dart';

class MarketPlansTab extends ConsumerStatefulWidget {
  const MarketPlansTab({super.key});

  @override
  ConsumerState<MarketPlansTab> createState() => _MarketPlansTabState();
}

class _MarketPlansTabState extends ConsumerState<MarketPlansTab> {

  @override
  void initState() {
    super.initState();
    Future.microtask(() => ref.read(marketPlansProvider.notifier).load());
  }

  @override
  void dispose() {
    super.dispose();
  }

  void _snack(String msg, {Color? color}) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(msg), backgroundColor: color),
    );
  }

  /// null = dismissed, true = wallet, false = razorpay.
  Future<bool?> _pickPaymentMethod() {
    return showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Choose payment method'),
        content: const Text('How would you like to pay for this plan?'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: const Text('Razorpay'),
          ),
          ElevatedButton(
            onPressed: () => Navigator.pop(ctx, true),
            style: ElevatedButton.styleFrom(
                backgroundColor: kPrimary, foregroundColor: Colors.white),
            child: const Text('Wallet'),
          ),
        ],
      ),
    );
  }

  Future<void> _showInsufficientBalance(
      MarketPlanModel plan, WalletSummary wallet) {
    return showDialog<void>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Insufficient balance'),
        content: Text(
          'Your wallet balance is ${wallet.formattedBalance}, but this plan '
          'costs ${plan.formattedPrice}. Please add money or pay with Razorpay.',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx),
            child: const Text('OK'),
          ),
        ],
      ),
    );
  }

  Future<bool> _confirmWalletSubscribe(
      MarketPlanModel plan, WalletSummary wallet) async {
    final result = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Confirm wallet payment'),
        content: Text(
          'Pay ${plan.formattedPrice} from your wallet balance of '
          '${wallet.formattedBalance}? Your plan will activate immediately.',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: const Text('Cancel'),
          ),
          ElevatedButton(
            onPressed: () => Navigator.pop(ctx, true),
            style: ElevatedButton.styleFrom(
                backgroundColor: kPrimary, foregroundColor: Colors.white),
            child: const Text('Confirm'),
          ),
        ],
      ),
    );
    return result ?? false;
  }

  Future<void> _handleBuyTap(MarketPlanModel plan) async {
    final useWallet = await _pickPaymentMethod();
    if (useWallet == null || !mounted) return;

    if (useWallet) {
      await _handleWalletPay(plan);
    } else {
      await _openRazorpay(plan);
    }
  }

  Future<void> _handleWalletPay(MarketPlanModel plan) async {
    try {
      final wallet = await ref.read(walletServiceProvider).fetchSummary();
      if (!mounted) return;

      if (wallet.balanceMinor < plan.priceMinor) {
        await _showInsufficientBalance(plan, wallet);
        return;
      }

      final confirmed = await _confirmWalletSubscribe(plan, wallet);
      if (!confirmed || !mounted) return;

      await ref.read(marketPlansProvider.notifier).subscribeWithWallet(plan.id);
      ref.invalidate(walletSummaryProvider);
      ref.invalidate(walletTransactionsProvider);
      await ref.read(marketPlansProvider.notifier).refreshAfterPayment();
      _snack('Market plan activated!', color: kSuccess);
    } catch (e) {
      _snack('Wallet payment failed: $e', color: kDanger);
    }
  }

  Future<void> _openRazorpay(MarketPlanModel plan) async {
    final auth = ref.read(authProvider);
    try {
      final raw =
          await ref.read(marketPlansProvider.notifier).createOrder(plan.id);
      final result = await CheckoutService.pay(
        order: CheckoutOrder.fromJson(raw),
        description: plan.name,
        email: auth.user?.email,
        phone: auth.user?.phone,
      );

      _snack('Payment successful! Activating your plan...', color: kSuccess);
      try {
        await ref.read(marketPlansProvider.notifier).verifyPayment(
              razorpayOrderId: result.orderId,
              razorpayPaymentId: result.paymentId,
              razorpaySignature: result.signature,
            );
      } catch (e) {
        // The server webhook still activates the plan if this call fails.
        _snack('Activation is taking longer than usual: $e', color: kDanger);
      }
      await ref.read(marketPlansProvider.notifier).refreshAfterPayment();
    } on CheckoutCancelled {
      // User backed out — nothing to report.
    } on CheckoutFailure catch (e) {
      _snack(e.message, color: kDanger);
    } catch (e) {
      _snack('Could not initiate payment: $e', color: kDanger);
    }
  }

  @override
  Widget build(BuildContext context) {
    final plans = ref.watch(marketPlansProvider);
    final currentPlanId = plans.mySubscription?.planId;

    return plans.isLoading
        ? _shimmerList()
        : plans.error != null
            ? _errorState(plans.error!)
            : plans.plans.isEmpty
                ? _emptyState()
                : RefreshIndicator(
                    color: kPrimary,
                    onRefresh: () =>
                        ref.read(marketPlansProvider.notifier).load(),
                    child: ListView.separated(
                      padding: const EdgeInsets.all(16),
                      itemCount: plans.plans.length,
                      separatorBuilder: (_, __) => const SizedBox(height: 12),
                      itemBuilder: (_, i) {
                        final plan = plans.plans[i];
                        return MarketPlanCard(
                          plan: plan,
                          isCurrentPlan: currentPlanId == plan.id,
                          isProcessing: plans.isPaymentProcessing,
                          onBuyTap: () => _handleBuyTap(plan),
                        );
                      },
                    ),
                  );
  }

  Widget _shimmerList() {
    return ListView.separated(
      padding: const EdgeInsets.all(16),
      itemCount: 3,
      separatorBuilder: (_, __) => const SizedBox(height: 12),
      itemBuilder: (_, __) => Shimmer.fromColors(
        baseColor: kMuted,
        highlightColor: kCard,
        child: Container(
          height: 220,
          decoration: BoxDecoration(
            color: kMuted,
            borderRadius: BorderRadius.circular(16),
          ),
        ),
      ),
    );
  }

  Widget _errorState(String error) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(32),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.error_outline, size: 48, color: kMutedForeground),
            const SizedBox(height: 12),
            Text(error,
                textAlign: TextAlign.center,
                style: const TextStyle(color: kMutedForeground)),
            const SizedBox(height: 16),
            ElevatedButton(
              onPressed: () => ref.read(marketPlansProvider.notifier).load(),
              child: const Text('Retry'),
            ),
          ],
        ),
      ),
    );
  }

  Widget _emptyState() {
    return const Center(
      child: Text(
        'No market plans available.',
        style: TextStyle(color: kMutedForeground),
      ),
    );
  }
}
