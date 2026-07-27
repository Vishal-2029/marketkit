import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:razorpay_flutter/razorpay_flutter.dart';
import 'package:shimmer/shimmer.dart';
import '../../../core/theme/app_colors.dart';
import '../../auth/providers/auth_provider.dart';
import '../../marketplace/models/wallet_model.dart';
import '../../marketplace/providers/wallet_provider.dart';
import '../models/market_plan_model.dart';
import '../providers/market_plans_provider.dart';
import '../widgets/market_plan_card.dart';
import 'package:marketkit/core/config/brand.dart';

class MarketPlansTab extends ConsumerStatefulWidget {
  const MarketPlansTab({super.key});

  @override
  ConsumerState<MarketPlansTab> createState() => _MarketPlansTabState();
}

class _MarketPlansTabState extends ConsumerState<MarketPlansTab> {
  late Razorpay _razorpay;

  @override
  void initState() {
    super.initState();
    _razorpay = Razorpay();
    _razorpay.on(Razorpay.EVENT_PAYMENT_SUCCESS, _handleSuccess);
    _razorpay.on(Razorpay.EVENT_PAYMENT_ERROR, _handleFailure);
    _razorpay.on(Razorpay.EVENT_EXTERNAL_WALLET, _handleExternalWallet);
    Future.microtask(() => ref.read(marketPlansProvider.notifier).load());
  }

  @override
  void dispose() {
    _razorpay.clear();
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

      if (wallet.balanceInPaise < plan.priceInPaise) {
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
      final order =
          await ref.read(marketPlansProvider.notifier).createOrder(plan.id);
      final keyId = order['key_id'] as String?;
      final orderId = order['order_id'] as String?;
      final amount = order['amount'];
      if (keyId == null || orderId == null || amount == null) {
        _snack('Payment setup failed. Please try again.', color: kDanger);
        return;
      }
      _razorpay.open({
        'key': keyId,
        'amount': amount,
        'order_id': orderId,
        'currency': order['currency'] ?? 'INR',
        'name': Brand.checkoutName,
        'description': plan.name,
        'prefill': {
          'contact': auth.user?.phone ?? '',
          'email': auth.user?.email ?? '',
        },
      });
    } catch (e) {
      _snack('Could not initiate payment: $e', color: kDanger);
    }
  }

  void _handleSuccess(PaymentSuccessResponse response) async {
    final orderId = response.orderId;
    final paymentId = response.paymentId;
    final signature = response.signature;

    _snack('Payment successful! Activating your plan...', color: kSuccess);

    try {
      if (orderId != null && paymentId != null && signature != null) {
        await ref.read(marketPlansProvider.notifier).verifyPayment(
              razorpayOrderId: orderId,
              razorpayPaymentId: paymentId,
              razorpaySignature: signature,
            );
      }
    } catch (e) {
      _snack('Activation is taking longer than usual: $e', color: kDanger);
    }

    await ref.read(marketPlansProvider.notifier).refreshAfterPayment();
    _snack('Market plan activated!', color: kSuccess);
  }

  void _handleFailure(PaymentFailureResponse response) {
    _snack('Payment failed: ${response.message ?? 'Unknown error'}',
        color: kDanger);
  }

  void _handleExternalWallet(ExternalWalletResponse response) {
    _snack('External wallet selected: ${response.walletName ?? 'Unknown'}');
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
