import '../../home/models/video_model.dart';

/// Metadata for a downloaded (encrypted) video, persisted locally so the
/// Downloads screen works fully offline. The actual video bytes live in a
/// separate `<id>.enc` file; [nonceB64] is the AES-CTR nonce for that file.
class DownloadEntry {
  final String id;
  final String title;
  final String description;
  final String category;
  final int durationSeconds;
  final String nonceB64;
  final String? thumbnailPath; // local app-private jpg path
  final int sizeBytes;
  final String mimeType; // content type of the stored video, for playback
  final String quality; // e.g. "720p"
  final DateTime downloadedAt;

  const DownloadEntry({
    required this.id,
    required this.title,
    required this.description,
    required this.category,
    required this.durationSeconds,
    required this.nonceB64,
    required this.thumbnailPath,
    required this.sizeBytes,
    this.mimeType = 'video/mp4',
    this.quality = '720p',
    required this.downloadedAt,
  });

  Map<String, dynamic> toJson() => {
        'id': id,
        'title': title,
        'description': description,
        'category': category,
        'duration_seconds': durationSeconds,
        'nonce': nonceB64,
        'thumbnail_path': thumbnailPath,
        'size_bytes': sizeBytes,
        'mime_type': mimeType,
        'quality': quality,
        'downloaded_at': downloadedAt.toIso8601String(),
      };

  factory DownloadEntry.fromJson(Map<String, dynamic> json) => DownloadEntry(
        id: json['id'] as String,
        title: json['title'] as String? ?? '',
        description: json['description'] as String? ?? '',
        category: json['category'] as String? ?? '',
        durationSeconds: json['duration_seconds'] as int? ?? 0,
        nonceB64: json['nonce'] as String,
        thumbnailPath: json['thumbnail_path'] as String?,
        sizeBytes: json['size_bytes'] as int? ?? 0,
        mimeType: json['mime_type'] as String? ?? 'video/mp4',
        quality: json['quality'] as String? ?? '720p',
        downloadedAt:
            DateTime.tryParse(json['downloaded_at'] as String? ?? '') ??
                DateTime.fromMillisecondsSinceEpoch(0),
      );

  /// Builds a [VideoModel] suitable for the player. Downloaded videos are by
  /// definition accessible and published; [fileKey] is unused for local playback.
  VideoModel toVideoModel() => VideoModel(
        id: id,
        title: title,
        description: description,
        category: category,
        status: 'PUBLISHED',
        fileKey: '',
        durationSeconds: durationSeconds,
        isPreview: false,
        isFree: false,
        accessible: true,
        thumbnailUrl: thumbnailPath,
        progressSeconds: 0,
      );

  String get formattedDuration {
    if (durationSeconds <= 0) return '';
    final m = durationSeconds ~/ 60;
    final s = durationSeconds % 60;
    return '${m.toString().padLeft(2, '0')}:${s.toString().padLeft(2, '0')}';
  }

  String get formattedSize {
    if (sizeBytes <= 0) return '';
    const mb = 1024 * 1024;
    if (sizeBytes >= mb) return '${(sizeBytes / mb).toStringAsFixed(1)} MB';
    return '${(sizeBytes / 1024).toStringAsFixed(0)} KB';
  }
}
