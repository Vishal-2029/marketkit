class PhotoModel {
  final String id;
  final String title;
  final String category;
  final String fileKey;
  final String mimeType;
  final String url;

  const PhotoModel({
    required this.id,
    required this.title,
    this.category = 'GENERAL',
    required this.fileKey,
    required this.mimeType,
    required this.url,
  });

  Map<String, dynamic> toJson() => {
        'id': id,
        'title': title,
        'category': category,
        'file_key': fileKey,
        'mime_type': mimeType,
        'url': url,
      };

  factory PhotoModel.fromJson(Map<String, dynamic> json) => PhotoModel(
        id: json['id'] as String,
        title: json['title'] as String,
        category: json['category'] as String? ?? 'GENERAL',
        fileKey: json['file_key'] as String? ?? '',
        mimeType: json['mime_type'] as String? ?? '',
        url: json['url'] as String? ?? '',
      );
}
