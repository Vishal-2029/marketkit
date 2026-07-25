import '../../../core/network/api_endpoints.dart';
import '../../../core/network/dio_client.dart';
import '../models/video_comment_model.dart';
import '../models/video_reaction_model.dart';

class EngagementService {
  final _dio = DioClient().dio;

  Future<VideoReactionState> getReactions(String videoId) async {
    final res = await _dio.get(ApiEndpoints.videoReactions(videoId));
    return VideoReactionState.fromJson(res.data['data'] as Map<String, dynamic>);
  }

  Future<VideoReactionState> toggleReaction(String videoId, String reaction) async {
    final res = await _dio.post(
      ApiEndpoints.videoReactions(videoId),
      data: {'reaction': reaction},
    );
    return VideoReactionState.fromJson(res.data['data'] as Map<String, dynamic>);
  }

  Future<List<VideoComment>> getComments(String videoId, {int page = 1}) async {
    final res = await _dio.get(
      ApiEndpoints.videoComments(videoId),
      queryParameters: {'page': page},
    );
    final list = res.data['data'] as List<dynamic>? ?? [];
    return list.map((e) => VideoComment.fromJson(e as Map<String, dynamic>)).toList();
  }

  Future<VideoComment> postComment(String videoId, String content) async {
    final res = await _dio.post(
      ApiEndpoints.videoComments(videoId),
      data: {'content': content},
    );
    return VideoComment.fromJson(res.data['data'] as Map<String, dynamic>);
  }

  Future<void> deleteComment(String videoId, String commentId) async {
    await _dio.delete(ApiEndpoints.videoComment(videoId, commentId));
  }
}
