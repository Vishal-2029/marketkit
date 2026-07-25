import '../../../core/network/dio_client.dart';
import '../../../core/network/api_endpoints.dart';
import '../models/market_plan_model.dart';

class MarketPlansService {
  final _dio = DioClient().dio;

  Future<List<MarketPlanModel>> fetchPlans() async {
    final res = await _dio.get(ApiEndpoints.marketPlans);
    final list = res.data['data'] as List<dynamic>? ?? [];
    return list
        .map((e) => MarketPlanModel.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  Future<MarketPlanSubscriptionModel?> fetchMySubscription() async {
    final res = await _dio.get(ApiEndpoints.marketPlansMy);
    final data = res.data['data'];
    if (data == null) return null;
    return MarketPlanSubscriptionModel.fromJson(data as Map<String, dynamic>);
  }

  Future<Map<String, dynamic>> createOrder(String planId) async {
    final res = await _dio.post(ApiEndpoints.marketPlanOrder(planId));
    return res.data['data'] as Map<String, dynamic>;
  }

  Future<void> verifyPayment({
    required String razorpayOrderId,
    required String razorpayPaymentId,
    required String razorpaySignature,
  }) async {
    await _dio.post(
      ApiEndpoints.marketPlanVerify,
      data: {
        'razorpay_order_id': razorpayOrderId,
        'razorpay_payment_id': razorpayPaymentId,
        'razorpay_signature': razorpaySignature,
      },
    );
  }

  Future<void> subscribeWithWallet(String planId) async {
    await _dio.post(ApiEndpoints.marketPlanWallet(planId));
  }

  Future<void> cancelMyPlan() async {
    await _dio.delete(ApiEndpoints.marketPlanCancel);
  }
}
