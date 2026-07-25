import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/cache/app_cache.dart';
import '../../auth/providers/auth_provider.dart';
import '../models/plan_model.dart';
import '../services/plans_service.dart';

class PlansState {
  final bool isLoading;
  final List<PlanModel> plans;
  final String? error;
  final bool isPaymentProcessing;
  final bool isStale;

  const PlansState({
    this.isLoading = false,
    this.plans = const [],
    this.error,
    this.isPaymentProcessing = false,
    this.isStale = false,
  });

  PlansState copyWith({
    bool? isLoading,
    List<PlanModel>? plans,
    String? error,
    bool? isPaymentProcessing,
    bool? isStale,
    bool clearError = false,
  }) =>
      PlansState(
        isLoading: isLoading ?? this.isLoading,
        plans: plans ?? this.plans,
        error: clearError ? null : (error ?? this.error),
        isPaymentProcessing: isPaymentProcessing ?? this.isPaymentProcessing,
        isStale: isStale ?? this.isStale,
      );
}

class PlansNotifier extends StateNotifier<PlansState> {
  final PlansService _service;
  final Ref _ref;

  PlansNotifier(this._service, this._ref) : super(const PlansState());

  static const _cacheKey = 'plans';

  Future<void> load() async {
    state = state.copyWith(isLoading: true, clearError: true);

    try {
      final plans = await _service.fetchPlans();
      await AppCache.putList(
          _cacheKey, plans.map((p) => p.toJson()).toList());
      state = state.copyWith(isLoading: false, plans: plans, isStale: false);
    } catch (_) {
      await _serveFromCache();
    }
  }

  Future<void> _serveFromCache() async {
    final cached = await AppCache.getList(_cacheKey);
    if (cached != null) {
      state = state.copyWith(
        isLoading: false,
        plans: cached.map(PlanModel.fromJson).toList(),
        isStale: true,
      );
    } else {
      state = state.copyWith(
        isLoading: false,
        error: 'No internet connection',
      );
    }
  }

  Future<Map<String, dynamic>> createOrder(String planId) async {
    state = state.copyWith(isPaymentProcessing: true);
    try {
      final order = await _service.createOrder(planId);
      state = state.copyWith(isPaymentProcessing: false);
      return order;
    } catch (e) {
      state = state.copyWith(isPaymentProcessing: false, error: e.toString());
      rethrow;
    }
  }

  Future<void> verifyPayment({
    required String razorpayOrderId,
    required String razorpayPaymentId,
    required String razorpaySignature,
  }) async {
    state = state.copyWith(isPaymentProcessing: true);
    try {
      await _service.verifyPayment(
        razorpayOrderId: razorpayOrderId,
        razorpayPaymentId: razorpayPaymentId,
        razorpaySignature: razorpaySignature,
      );
      state = state.copyWith(isPaymentProcessing: false);
    } catch (e) {
      state = state.copyWith(isPaymentProcessing: false, error: e.toString());
      rethrow;
    }
  }

  Future<void> refreshAfterPayment() async {
    await _ref.read(authProvider.notifier).fetchMe();
  }
}

final plansServiceProvider = Provider((ref) => PlansService());

final plansProvider = StateNotifierProvider<PlansNotifier, PlansState>(
  (ref) => PlansNotifier(ref.read(plansServiceProvider), ref),
);
