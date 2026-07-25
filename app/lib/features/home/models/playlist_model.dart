import 'video_model.dart';

class PlaylistModel {
  final String id;
  final String name;
  final String description;
  final String thumbnailUrl;
  final int videoCount;
  final List<VideoModel> videos;

  const PlaylistModel({
    required this.id,
    required this.name,
    required this.description,
    required this.thumbnailUrl,
    required this.videoCount,
    this.videos = const [],
  });

  factory PlaylistModel.fromJson(Map<String, dynamic> json) => PlaylistModel(
        id: json['id'] as String,
        name: json['name'] as String,
        description: json['description'] as String? ?? '',
        thumbnailUrl: json['thumbnail_url'] as String? ?? '',
        videoCount: json['video_count'] as int? ?? 0,
        videos: (json['videos'] as List<dynamic>?)
                ?.map((e) => VideoModel.fromJson(e as Map<String, dynamic>))
                .toList() ??
            [],
      );
}
