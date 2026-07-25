import 'package:dio/dio.dart';
import 'package:file_picker/file_picker.dart';
import 'package:image_picker/image_picker.dart';
import 'package:path_provider/path_provider.dart';
import '../../../core/network/api_endpoints.dart';
import '../../../core/network/dio_client.dart';
import '../../../core/utils/upload_filename.dart';
import '../models/design_category_model.dart';
import '../models/design_model.dart';
import '../models/design_thread_message.dart';
import '../models/earnings_model.dart';
import '../models/purchase_model.dart';

class MarketService {
  final _dio = DioClient().dio;

  Future<List<DesignModel>> fetchDesigns({
    String? search,
    String? categoryId,
    int page = 1,
  }) async {
    final params = <String, dynamic>{'page': page};
    if (search != null && search.isNotEmpty) params['search'] = search;
    if (categoryId != null && categoryId.isNotEmpty) {
      params['category_id'] = categoryId;
    }
    final res = await _dio.get(
      ApiEndpoints.marketDesigns,
      queryParameters: params,
    );
    final list = res.data['data'] as List<dynamic>;
    return list
        .map((e) => DesignModel.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  Future<List<DesignCategoryModel>> fetchCategories() async {
    final res = await _dio.get(ApiEndpoints.marketCategories);
    final list = res.data['data'] as List<dynamic>;
    return list
        .map((e) => DesignCategoryModel.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  Future<DesignModel> fetchDesign(String id) async {
    final res = await _dio.get(ApiEndpoints.marketDesign(id));
    return DesignModel.fromJson(res.data['data'] as Map<String, dynamic>);
  }

  Future<DesignModel> uploadDesign({
    required String title,
    required String description,
    required int priceInPaise,
    required PlatformFile file,
    required List<XFile> previews,
    required String categoryId,
    String? categoryOther,
  }) async {
    final formData = FormData.fromMap({
      'title': title,
      'description': description,
      'price_in_paise': priceInPaise.toString(),
      'category_id': categoryId,
      if (categoryOther != null && categoryOther.isNotEmpty)
        'category_other': categoryOther,
    });

    final fileBytes = file.bytes;
    if (fileBytes != null) {
      formData.files.add(
        MapEntry(
          'file',
          MultipartFile.fromBytes(fileBytes, filename: file.name),
        ),
      );
    } else {
      formData.files.add(
        MapEntry(
          'file',
          await MultipartFile.fromFile(file.path!, filename: file.name),
        ),
      );
    }

    final previewParts = await Future.wait(
      previews.map((img) async {
        final bytes = await img.readAsBytes();
        return MapEntry(
          'previews',
          MultipartFile.fromBytes(bytes, filename: uploadImageFilename(img)),
        );
      }),
    );
    formData.files.addAll(previewParts);

    final res = await _dio.post(ApiEndpoints.marketDesigns, data: formData);
    return DesignModel.fromJson(res.data['data'] as Map<String, dynamic>);
  }

  Future<void> deleteDesign(String id) async {
    await _dio.delete(ApiEndpoints.marketDesign(id));
  }

  Future<List<DesignModel>> fetchMyDesigns() async {
    final res = await _dio.get(ApiEndpoints.marketMyDesigns);
    final list = res.data['data'] as List<dynamic>;
    return list
        .map((e) => DesignModel.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  /// Returns {view_count, sales_count, revenue_in_paise} for one of the
  /// caller's own designs — how many times it sold and total earned from it.
  Future<Map<String, dynamic>> fetchMyDesignStats(String designId) async {
    final res = await _dio.get(ApiEndpoints.marketMyDesignStats(designId));
    return res.data['data'] as Map<String, dynamic>;
  }

  Future<List<PurchaseModel>> fetchMyPurchases() async {
    final res = await _dio.get(ApiEndpoints.marketMyPurchases);
    final list = res.data['data'] as List<dynamic>;
    return list
        .map((e) => PurchaseModel.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  Future<EarningsModel> fetchEarnings() async {
    final res = await _dio.get(ApiEndpoints.marketEarnings);
    return EarningsModel.fromJson(res.data['data'] as Map<String, dynamic>);
  }

  Future<Map<String, dynamic>> createOrder(String designId) async {
    final res = await _dio.post(
      ApiEndpoints.marketOrder,
      data: {'design_id': designId},
    );
    return res.data['data'] as Map<String, dynamic>;
  }

  Future<String> verifyPurchase({
    required String razorpayOrderId,
    required String razorpayPaymentId,
    required String razorpaySignature,
  }) async {
    final res = await _dio.post(
      ApiEndpoints.marketVerify,
      data: {
        'razorpay_order_id': razorpayOrderId,
        'razorpay_payment_id': razorpayPaymentId,
        'razorpay_signature': razorpaySignature,
      },
    );
    final data = res.data['data'] as Map<String, dynamic>?;
    return data?['purchase_id'] as String? ?? '';
  }

  /// Returns {url, file_name} for a purchased (or own) design file.
  Future<Map<String, dynamic>> fetchDownloadUrl(String designId) async {
    final res = await _dio.get(ApiEndpoints.marketDownloadUrl(designId));
    return res.data['data'] as Map<String, dynamic>;
  }

  /// Downloads the invoice PDF for a purchase to a local temp file and
  /// returns its path.
  Future<String> downloadInvoice(String purchaseId) async {
    final dir = await getTemporaryDirectory();
    final savePath = '${dir.path}/invoice-$purchaseId.pdf';
    await _dio.download(ApiEndpoints.marketInvoice(purchaseId), savePath);
    return savePath;
  }

  /// Saves a remote file (e.g. signed design URL) to [savePath] on disk.
  Future<void> downloadFileToPath(String url, String savePath) async {
    await _dio.download(url, savePath);
  }

  /// The buyer's private support thread for a purchased design — only they
  /// and admins can see it; the seller never has access.
  Future<List<DesignThreadMessage>> fetchDesignMessages(String designId) async {
    final res = await _dio.get(ApiEndpoints.marketDesignMessages(designId));
    final list = res.data['data'] as List<dynamic>;
    return list
        .map((e) => DesignThreadMessage.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  Future<DesignThreadMessage> postDesignMessage(
    String designId,
    String content,
  ) async {
    final res = await _dio.post(
      ApiEndpoints.marketDesignMessages(designId),
      data: {'content': content},
    );
    return DesignThreadMessage.fromJson(
      res.data['data'] as Map<String, dynamic>,
    );
  }
}
