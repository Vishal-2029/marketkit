import '../../../core/network/dio_client.dart';
import '../../../core/network/api_endpoints.dart';
import '../models/plan_model.dart';

class PlansService {
  final _dio = DioClient().dio;

  Future<List<PlanModel>> fetchPlans() async {
    final res = await _dio.get(ApiEndpoints.plans);
    final list = res.data['data'] as List<dynamic>;
    return list
        .map((e) => PlanModel.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  Future<Map<String, dynamic>> createOrder(String planId) async {
    final res = await _dio.post(
      ApiEndpoints.userPaymentOrder,
      data: {'plan_id': planId},
    );
    return res.data['data'] as Map<String, dynamic>;
  }

  Future<void> verifyPayment({
    required String razorpayOrderId,
    required String razorpayPaymentId,
    required String razorpaySignature,
  }) async {
    await _dio.post(
      ApiEndpoints.userPaymentVerify,
      data: {
        'razorpay_order_id': razorpayOrderId,
        'razorpay_payment_id': razorpayPaymentId,
        'razorpay_signature': razorpaySignature,
      },
    );
  }
}
