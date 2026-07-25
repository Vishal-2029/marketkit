import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/cache/app_cache.dart';
import '../models/market_plan_model.dart';
import '../services/market_plans_service.dart';

class MarketPlansState {
  final bool isLoading;
  final List<MarketPlanModel> plans;
  final MarketPlanSubscriptionModel? mySubscription;
  final String? error;
  final bool isPaymentProcessing;
  final bool isStale;

  const MarketPlansState({
    this.isLoading = false,
    this.plans = const [],
    this.mySubscription,
    this.error,
    this.isPaymentProcessing = false,
    this.isStale = false,
  });

  MarketPlansState copyWith({
    bool? isLoading,
    List<MarketPlanModel>? plans,
    MarketPlanSubscriptionModel? mySubscription,
    String? error,
    bool? isPaymentProcessing,
    bool? isStale,
    bool clearError = false,
    bool clearSubscription = false,
  }) =>
      MarketPlansState(
        isLoading: isLoading ?? this.isLoading,
        plans: plans ?? this.plans,
        mySubscription: clearSubscription
            ? null
            : (mySubscription ?? this.mySubscription),
        error: clearError ? null : (error ?? this.error),
        isPaymentProcessing: isPaymentProcessing ?? this.isPaymentProcessing,
        isStale: isStale ?? this.isStale,
      );
}

class MarketPlansNotifier extends StateNotifier<MarketPlansState> {
  final MarketPlansService _service;

  MarketPlansNotifier(this._service) : super(const MarketPlansState());

  static const _cacheKey = 'market_plans';

  Future<void> load() async {
    state = state.copyWith(isLoading: true, clearError: true);

    try {
      final results = await Future.wait([
        _service.fetchPlans(),
        _service.fetchMySubscription(),
      ]);
      final plans = results[0] as List<MarketPlanModel>;
      final mySub = results[1] as MarketPlanSubscriptionModel?;
      await AppCache.putList(
          _cacheKey, plans.map((p) => p.toJson()).toList());
      state = state.copyWith(
        isLoading: false,
        plans: plans,
        mySubscription: mySub,
        clearSubscription: mySub == null,
        isStale: false,
      );
    } catch (_) {
      await _serveFromCache();
    }
  }

  Future<void> _serveFromCache() async {
    final cached = await AppCache.getList(_cacheKey);
    if (cached != null) {
      state = state.copyWith(
        isLoading: false,
        plans: cached.map(MarketPlanModel.fromJson).toList(),
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

  Future<void> subscribeWithWallet(String planId) async {
    state = state.copyWith(isPaymentProcessing: true);
    try {
      await _service.subscribeWithWallet(planId);
      state = state.copyWith(isPaymentProcessing: false);
    } catch (e) {
      state = state.copyWith(isPaymentProcessing: false, error: e.toString());
      rethrow;
    }
  }

  Future<void> cancelMyPlan() async {
    state = state.copyWith(isPaymentProcessing: true);
    try {
      await _service.cancelMyPlan();
      state = state.copyWith(
        isPaymentProcessing: false,
        clearSubscription: true,
      );
    } catch (e) {
      state = state.copyWith(isPaymentProcessing: false, error: e.toString());
      rethrow;
    }
  }

  Future<void> refreshAfterPayment() async {
    await load();
  }
}

final marketPlansServiceProvider = Provider((ref) => MarketPlansService());

final marketPlansProvider =
    StateNotifierProvider<MarketPlansNotifier, MarketPlansState>(
  (ref) => MarketPlansNotifier(ref.read(marketPlansServiceProvider)),
);
