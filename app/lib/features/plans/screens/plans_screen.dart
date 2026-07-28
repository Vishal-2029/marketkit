import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:shimmer/shimmer.dart';
import '../../../core/theme/app_colors.dart';
import '../../auth/providers/auth_provider.dart';
import '../models/plan_model.dart';
import '../providers/plans_provider.dart';
import '../widgets/plan_card.dart';
import 'package:marketkit/core/payments/checkout.dart';

class PlansScreen extends ConsumerStatefulWidget {
  const PlansScreen({super.key});

  @override
  ConsumerState<PlansScreen> createState() => _PlansScreenState();
}

class _PlansScreenState extends ConsumerState<PlansScreen> {
  @override
  void initState() {
    super.initState();
    Future.microtask(() => ref.read(plansProvider.notifier).load());
  }

  void _snack(String msg, {Color? color}) {
    if (!mounted) return;
    ScaffoldMessenger.of(context)
        .showSnackBar(SnackBar(content: Text(msg), backgroundColor: color));
  }

  Future<void> _handleBuyTap(PlanModel plan) async {
    final auth = ref.read(authProvider);
    try {
      final raw = await ref.read(plansProvider.notifier).createOrder(plan.id);
      final result = await CheckoutService.pay(
        order: CheckoutOrder.fromJson(raw),
        description: plan.name,
        email: auth.user?.email,
        phone: auth.user?.phone,
      );

      _snack('Payment successful! Activating your plan...', color: kSuccess);
      try {
        await ref.read(plansProvider.notifier).verifyPayment(
              razorpayOrderId: result.orderId,
              razorpayPaymentId: result.paymentId,
              razorpaySignature: result.signature,
            );
      } catch (e) {
        // The server webhook still activates the plan if this call fails.
        _snack('Activation is taking longer than usual: $e', color: kDanger);
      }

      await ref.read(plansProvider.notifier).refreshAfterPayment();
      if (mounted) context.go('/home');
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
