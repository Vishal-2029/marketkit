import '../../home/models/video_model.dart';

/// A bookmarked video. Unlike [DownloadEntry], this doesn't store any video
/// bytes — it just remembers which online videos the user starred, so the
/// video is re-streamed normally from the player.
class FavoriteEntry {
  final VideoModel video;
  final DateTime favoritedAt;

  const FavoriteEntry({required this.video, required this.favoritedAt});

  Map<String, dynamic> toJson() => {
        'video': video.toJson(),
        'favorited_at': favoritedAt.toIso8601String(),
      };

  factory FavoriteEntry.fromJson(Map<String, dynamic> json) => FavoriteEntry(
        video: VideoModel.fromJson(json['video'] as Map<String, dynamic>),
        favoritedAt:
            DateTime.tryParse(json['favorited_at'] as String? ?? '') ??
                DateTime.fromMillisecondsSinceEpoch(0),
      );
}
