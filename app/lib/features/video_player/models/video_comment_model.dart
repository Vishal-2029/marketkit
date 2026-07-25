class VideoComment {
  final String id;
  final String userName;
  final String content;
  final DateTime createdAt;
  final bool isAdmin;

  const VideoComment({
    required this.id,
    required this.userName,
    required this.content,
    required this.createdAt,
    this.isAdmin = false,
  });

  factory VideoComment.fromJson(Map<String, dynamic> json) => VideoComment(
        id: json['id'] as String,
        userName: json['user_name'] as String? ?? 'User',
        content: json['content'] as String,
        createdAt: DateTime.parse(json['created_at'] as String),
        isAdmin: json['is_admin'] as bool? ?? false,
      );
}
