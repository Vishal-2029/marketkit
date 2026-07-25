class VideoReactionState {
  final int likeCount;
  final int dislikeCount;
  final String userReaction; // "like", "dislike", or ""

  const VideoReactionState({
    required this.likeCount,
    required this.dislikeCount,
    required this.userReaction,
  });

  factory VideoReactionState.fromJson(Map<String, dynamic> json) =>
      VideoReactionState(
        likeCount: (json['like_count'] as num?)?.toInt() ?? 0,
        dislikeCount: (json['dislike_count'] as num?)?.toInt() ?? 0,
        userReaction: json['user_reaction'] as String? ?? '',
      );

  static const empty = VideoReactionState(
    likeCount: 0,
    dislikeCount: 0,
    userReaction: '',
  );
}
