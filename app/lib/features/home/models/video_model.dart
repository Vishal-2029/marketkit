import 'package:design_express/core/config/feature_catalog.dart';
import '../../../core/network/api_endpoints.dart';

class VideoModel {
  final String id;
  final String title;
  final String description;
  final String category;
  final String status;
  final String fileKey;
  final int durationSeconds;
  final bool isPreview;
  final bool isFree;
  final bool accessible;
  final String? thumbnailUrl;
  final String? lqip;
  final int progressSeconds;

  const VideoModel({
    required this.id,
    required this.title,
    required this.description,
    required this.category,
    required this.status,
    required this.fileKey,
    required this.durationSeconds,
    required this.isPreview,
    required this.isFree,
    required this.accessible,
    this.thumbnailUrl,
    this.lqip,
    this.progressSeconds = 0,
  });

  factory VideoModel.fromJson(Map<String, dynamic> json) => VideoModel(
        id: json['id'] as String,
        title: json['title'] as String,
        description: json['description'] as String? ?? '',
        category: json['category'] as String,
        status: json['status'] as String,
        fileKey: json['file_key'] as String? ?? '',
        durationSeconds: json['duration_seconds'] as int? ?? 0,
        isPreview: json['is_preview'] as bool? ?? false,
        isFree: json['is_free'] as bool? ?? false,
        accessible: json['accessible'] as bool? ?? false,
        thumbnailUrl: _normalizeThumbnailUrl(json['thumbnail_url'] as String?),
        lqip: json['lqip'] as String?,
        progressSeconds: json['progress_seconds'] as int? ?? 0,
      );

  static String? _normalizeThumbnailUrl(String? url) {
    if (url == null || url.trim().isEmpty) return null;

    if (url.startsWith('/uploads/')) {
      return '${ApiEndpoints.staticBase}$url';
    }

    final parsed = Uri.tryParse(url);
    if (parsed != null && parsed.path.startsWith('/uploads/')) {
      final baseUri = Uri.parse(ApiEndpoints.staticBase);
      if (parsed.host == 'localhost' || parsed.host == '127.0.0.1') {
        return baseUri.replace(path: parsed.path, query: parsed.query, fragment: parsed.fragment).toString();
      }
    }

    return url;
  }

  String get formattedDuration {
    if (durationSeconds <= 0) return '';
    final m = durationSeconds ~/ 60;
    final s = durationSeconds % 60;
    return '${m.toString().padLeft(2, '0')}:${s.toString().padLeft(2, '0')}';
  }

  /// Copy with a different saved progress (e.g. 0 when the user removes the
  /// video from Continue Watching).
  VideoModel withProgress(int seconds) => VideoModel(
        id: id,
        title: title,
        description: description,
        category: category,
        status: status,
        fileKey: fileKey,
        durationSeconds: durationSeconds,
        isPreview: isPreview,
        isFree: isFree,
        accessible: accessible,
        thumbnailUrl: thumbnailUrl,
        lqip: lqip,
        progressSeconds: seconds,
      );

  Map<String, dynamic> toJson() => {
        'id': id,
        'title': title,
        'description': description,
        'category': category,
        'status': status,
        'file_key': fileKey,
        'duration_seconds': durationSeconds,
        'is_preview': isPreview,
        'is_free': isFree,
        'accessible': accessible,
        'thumbnail_url': thumbnailUrl,
        'progress_seconds': progressSeconds,
      };

  String get categoryLabel => FeatureCatalog.label(category);
}
