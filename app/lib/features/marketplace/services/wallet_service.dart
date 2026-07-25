import '../../../core/network/api_endpoints.dart';
import '../../../core/network/dio_client.dart';
import '../models/purchase_model.dart';
import '../models/wallet_model.dart';

class WalletService {
  final _dio = DioClient().dio;

  Future<WalletSummary> fetchSummary() async {
    final res = await _dio.get(ApiEndpoints.walletSummary);
    return WalletSummary.fromJson(res.data['data'] as Map<String, dynamic>);
  }

  Future<List<WalletTransactionModel>> fetchTransactions({int page = 1}) async {
    final res = await _dio.get(ApiEndpoints.walletTransactions,
        queryParameters: {'page': page});
    final list = res.data['data'] as List<dynamic>;
    return list
        .map((e) => WalletTransactionModel.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  /// Returns {order_id, amount, currency, key_id} — same shape as the design
  /// purchase order, so the Razorpay checkout call is identical.
  Future<Map<String, dynamic>> createTopupOrder(int amountInPaise) async {
    final res = await _dio.post(ApiEndpoints.walletTopupOrder,
        data: {'amount_in_paise': amountInPaise});
    return res.data['data'] as Map<String, dynamic>;
  }

  Future<void> verifyTopup({
    required String razorpayOrderId,
    required String razorpayPaymentId,
    required String razorpaySignature,
  }) async {
    await _dio.post(ApiEndpoints.walletTopupVerify, data: {
      'razorpay_order_id': razorpayOrderId,
      'razorpay_payment_id': razorpayPaymentId,
      'razorpay_signature': razorpaySignature,
    });
  }

  Future<PayoutDetailsModel> fetchPayoutDetails() async {
    final res = await _dio.get(ApiEndpoints.walletPayoutDetails);
    return PayoutDetailsModel.fromJson(
        res.data['data'] as Map<String, dynamic>);
  }

  Future<void> savePayoutDetails({
    required String upiId,
    required String bankAccountNumber,
    required String bankIfsc,
    required String bankHolderName,
  }) async {
    await _dio.put(ApiEndpoints.walletPayoutDetails, data: {
      'upi_id': upiId,
      'bank_account_number': bankAccountNumber,
      'bank_ifsc': bankIfsc,
      'bank_holder_name': bankHolderName,
    });
  }

  Future<void> createWithdrawal({
    required int amountInPaise,
    required String method,
  }) async {
    await _dio.post(ApiEndpoints.walletWithdrawals, data: {
      'amount_in_paise': amountInPaise,
      'method': method,
    });
  }

  Future<List<WithdrawalModel>> fetchMyWithdrawals() async {
    final res = await _dio.get(ApiEndpoints.walletWithdrawals);
    final list = res.data['data'] as List<dynamic>;
    return list
        .map((e) => WithdrawalModel.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  Future<int> fetchFeePercent() async {
    final res = await _dio.get(ApiEndpoints.marketFee);
    return (res.data['data']['fee_percent'] as num?)?.toInt() ?? 0;
  }

  /// Buys a design directly from wallet balance. Throws DioException with
  /// status 400 ("insufficient wallet balance") when the balance is short —
  /// callers fall back to the Razorpay flow.
  Future<PurchaseModel> purchaseWithWallet(String designId) async {
    final res = await _dio
        .post(ApiEndpoints.marketWalletPurchase, data: {'design_id': designId});
    return PurchaseModel.fromJson(res.data['data'] as Map<String, dynamic>);
  }
}
