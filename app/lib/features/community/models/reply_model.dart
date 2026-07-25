class ReplyModel {
  final String id;
  final String anonName;
  final String content;
  final DateTime createdAt;
  final String? imageUrl;

  const ReplyModel({
    required this.id,
    required this.anonName,
    required this.content,
    required this.createdAt,
    this.imageUrl,
  });

  factory ReplyModel.fromJson(Map<String, dynamic> json) => ReplyModel(
        id: json['id'] as String,
        anonName: json['anon_name'] as String,
        content: json['content'] as String,
        createdAt: DateTime.parse(json['created_at'] as String),
        imageUrl: json['image_url'] as String?,
      );
}
