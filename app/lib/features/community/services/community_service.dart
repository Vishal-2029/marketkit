import 'package:dio/dio.dart';
import 'package:image_picker/image_picker.dart';
import '../../../core/network/dio_client.dart';
import '../../../core/network/api_endpoints.dart';
import '../../../core/utils/upload_filename.dart';
import '../models/post_model.dart';
import '../models/reply_model.dart';

class CommunityService {
  final _dio = DioClient().dio;

  Future<List<PostModel>> fetchPosts({String? category, int page = 1}) async {
    final params = <String, dynamic>{'page': page};
    if (category != null && category != 'ALL') params['category'] = category;
    final res = await _dio.get(ApiEndpoints.communityPosts, queryParameters: params);
    final list = res.data['data'] as List<dynamic>;
    return list.map((e) => PostModel.fromJson(e as Map<String, dynamic>)).toList();
  }

  Future<PostModel> createPost({
    required String category,
    required String title,
    required String content,
    List<dynamic> images = const [],
  }) async {
    final formData = FormData.fromMap({
      'category': category,
      'title': title,
      'content': content,
    });
    // Read/encode the image parts in parallel (order preserved) so disk reads
    // don't serialize before the upload.
    final files = await Future.wait(images.map((img) async {
      final xfile = img as XFile;
      final bytes = await xfile.readAsBytes();
      final filename = uploadImageFilename(xfile);
      return MapEntry(
        'images',
        MultipartFile.fromBytes(bytes, filename: filename),
      );
    }));
    formData.files.addAll(files);
    final res = await _dio.post(ApiEndpoints.communityPosts, data: formData);
    return PostModel.fromJson(res.data['data'] as Map<String, dynamic>);
  }

  Future<Map<String, dynamic>> fetchPostDetail(String id) async {
    final res = await _dio.get(ApiEndpoints.communityPost(id));
    return res.data['data'] as Map<String, dynamic>;
  }

  Future<ReplyModel> createReply(String postId, String content,
      {dynamic image}) async {
    final formData = FormData.fromMap({'content': content});
    if (image != null) {
      final xfile = image as XFile;
      final bytes = await xfile.readAsBytes();
      final filename = uploadImageFilename(xfile);
      formData.files.add(MapEntry(
        'image',
        MultipartFile.fromBytes(bytes, filename: filename),
      ));
    }
    final res = await _dio.post(ApiEndpoints.communityReplies(postId),
        data: formData);
    return ReplyModel.fromJson(res.data['data'] as Map<String, dynamic>);
  }
}
