class PostModel {
  final String id;
  final String anonName;
  final String category;
  final String title;
  final String content;
  final int replyCount;
  final DateTime createdAt;
  final List<String> imageUrls;

  const PostModel({
    required this.id,
    required this.anonName,
    required this.category,
    required this.title,
    required this.content,
    required this.replyCount,
    required this.createdAt,
    this.imageUrls = const [],
  });

  Map<String, dynamic> toJson() => {
        'id': id,
        'anon_name': anonName,
        'category': category,
        'title': title,
        'content': content,
        'reply_count': replyCount,
        'created_at': createdAt.toIso8601String(),
        'image_urls': imageUrls,
      };

  factory PostModel.fromJson(Map<String, dynamic> json) => PostModel(
        id: json['id'] as String,
        anonName: json['anon_name'] as String,
        category: json['category'] as String? ?? 'GENERAL',
        title: json['title'] as String,
        content: json['content'] as String,
        replyCount: json['reply_count'] as int? ?? 0,
        createdAt: DateTime.parse(json['created_at'] as String),
        imageUrls: (json['image_urls'] as List<dynamic>?)
                ?.map((e) => e as String)
                .toList() ??
            [],
      );
}
