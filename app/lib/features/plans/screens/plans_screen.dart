import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:razorpay_flutter/razorpay_flutter.dart';
import 'package:shimmer/shimmer.dart';
import '../../../core/theme/app_colors.dart';
import '../../auth/providers/auth_provider.dart';
import '../models/plan_model.dart';
import '../providers/plans_provider.dart';
import '../widgets/plan_card.dart';
import 'package:marketkit/core/config/brand.dart';

class PlansScreen extends ConsumerStatefulWidget {
  const PlansScreen({super.key});

  @override
  ConsumerState<PlansScreen> createState() => _PlansScreenState();
}

class _PlansScreenState extends ConsumerState<PlansScreen> {
  late Razorpay _razorpay;

  @override
  void initState() {
    super.initState();
    _razorpay = Razorpay();
    _razorpay.on(Razorpay.EVENT_PAYMENT_SUCCESS, _handleSuccess);
    _razorpay.on(Razorpay.EVENT_PAYMENT_ERROR, _handleFailure);
    _razorpay.on(Razorpay.EVENT_EXTERNAL_WALLET, _handleExternalWallet);
    Future.microtask(() => ref.read(plansProvider.notifier).load());
  }

  @override
  void dispose() {
    _razorpay.clear();
    super.dispose();
  }

  Future<void> _handleBuyTap(PlanModel plan) async {
    final auth = ref.read(authProvider);
    try {
      final order =
          await ref.read(plansProvider.notifier).createOrder(plan.id);
      final keyId = order['key_id'] as String?;
      final orderId = order['order_id'] as String?;
      final amount = order['amount'];
      if (keyId == null || orderId == null || amount == null) {
        if (!mounted) return;
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('Payment setup failed. Please try again.'),
            backgroundColor: kDanger,
          ),
        );
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
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text('Could not initiate payment: $e'),
          backgroundColor: kDanger,
        ),
      );
    }
  }

  void _handleSuccess(PaymentSuccessResponse response) async {
    final orderId = response.orderId;
    final paymentId = response.paymentId;
    final signature = response.signature;

    ScaffoldMessenger.of(context).showSnackBar(
      const SnackBar(
        content: Text('Payment successful! Activating your plan...'),
        backgroundColor: kSuccess,
      ),
    );

    try {
      if (orderId != null && paymentId != null && signature != null) {
        await ref.read(plansProvider.notifier).verifyPayment(
              razorpayOrderId: orderId,
              razorpayPaymentId: paymentId,
              razorpaySignature: signature,
            );
      }
    } catch (e) {
      // Webhook fallback: even if verify fails, the server webhook can still activate.
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text('Activation is taking longer than usual: $e'),
            backgroundColor: kDanger,
          ),
        );
      }
    }

    await ref.read(plansProvider.notifier).refreshAfterPayment();
    if (mounted) context.go('/home');
  }

  void _handleFailure(PaymentFailureResponse response) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text('Payment failed: ${response.message ?? 'Unknown error'}'),
        backgroundColor: kDanger,
      ),
    );
  }

  void _handleExternalWallet(ExternalWalletResponse response) {
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(
            'External wallet selected: ${response.walletName ?? 'Unknown'}'),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final plans = ref.watch(plansProvider);
    final auth = ref.watch(authProvider);
    final activePlanIds = auth.user?.activePlanIds ?? {};

    return Scaffold(
      backgroundColor: kBackground,
      appBar: AppBar(
        backgroundColor: kBackground,
        title: const Text(
          'Choose a Plan',
          style: TextStyle(fontWeight: FontWeight.w700, fontSize: 18),
        ),
        centerTitle: false,
        elevation: 0,
      ),
      body: SafeArea(
        child: plans.isLoading
            ? _shimmerList()
            : plans.error != null
                ? _errorState(plans.error!)
                : plans.plans.isEmpty
                    ? _emptyState()
                    : RefreshIndicator(
                        color: kPrimary,
                        onRefresh: () => ref.read(plansProvider.notifier).load(),
                        child: ListView.separated(
                          padding: const EdgeInsets.all(16),
                          itemCount: plans.plans.length,
                          separatorBuilder: (_, __) =>
                              const SizedBox(height: 12),
                          itemBuilder: (_, i) {
                            final plan = plans.plans[i];
                            return PlanCard(
                              plan: plan,
                              isCurrentPlan: activePlanIds.contains(plan.id),
                              isProcessing: plans.isPaymentProcessing,
                              onBuyTap: () => _handleBuyTap(plan),
                            );
                          },
                        ),
                      ),
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
              onPressed: () => ref.read(plansProvider.notifier).load(),
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
        'No plans available.',
        style: TextStyle(color: kMutedForeground),
      ),
    );
  }
}
