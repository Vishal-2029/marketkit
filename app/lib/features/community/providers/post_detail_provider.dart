import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../models/post_model.dart';
import '../models/reply_model.dart';
import '../services/community_service.dart';
import 'posts_provider.dart';

class PostDetailState {
  final bool isLoading;
  final PostModel? post;
  final List<ReplyModel> replies;
  final bool isSubmitting;
  final String? error;

  const PostDetailState({
    this.isLoading = false,
    this.post,
    this.replies = const [],
    this.isSubmitting = false,
    this.error,
  });

  PostDetailState copyWith({
    bool? isLoading,
    PostModel? post,
    List<ReplyModel>? replies,
    bool? isSubmitting,
    String? error,
    bool clearError = false,
  }) =>
      PostDetailState(
        isLoading: isLoading ?? this.isLoading,
        post: post ?? this.post,
        replies: replies ?? this.replies,
        isSubmitting: isSubmitting ?? this.isSubmitting,
        error: clearError ? null : (error ?? this.error),
      );
}

class PostDetailNotifier extends StateNotifier<PostDetailState> {
  final CommunityService _service;

  PostDetailNotifier(this._service) : super(const PostDetailState());

  Future<void> load(String postId) async {
    state = state.copyWith(isLoading: true, clearError: true);
    try {
      final data = await _service.fetchPostDetail(postId);
      final post = PostModel.fromJson(data);
      final repliesRaw = data['replies'] as List<dynamic>? ?? [];
      final replies = repliesRaw
          .map((e) => ReplyModel.fromJson(e as Map<String, dynamic>))
          .toList();
      state = state.copyWith(isLoading: false, post: post, replies: replies);
    } catch (e) {
      state = state.copyWith(isLoading: false, error: e.toString());
    }
  }

  Future<void> addReply(String postId, String content, {dynamic image}) async {
    state = state.copyWith(isSubmitting: true, clearError: true);
    try {
      final reply = await _service.createReply(postId, content, image: image);
      state = state.copyWith(
        isSubmitting: false,
        replies: [...state.replies, reply],
      );
    } catch (e) {
      state = state.copyWith(isSubmitting: false, error: e.toString());
      rethrow;
    }
  }
}

final postDetailProvider = StateNotifierProvider.autoDispose
    .family<PostDetailNotifier, PostDetailState, String>(
  (ref, _) => PostDetailNotifier(ref.read(communityServiceProvider)),
);
