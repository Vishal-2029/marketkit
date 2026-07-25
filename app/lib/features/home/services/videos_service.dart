import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart';
import '../../../core/network/dio_client.dart';
import '../../../core/network/api_endpoints.dart';
import '../../downloads/models/download_quality.dart';
import '../models/video_model.dart';

class _CachedUrl {
  final String url;
  final DateTime expiresAt;
  _CachedUrl(this.url, this.expiresAt);
}

class VideosService {
  final _dio = DioClient().dio;

  static final Map<String, _CachedUrl> _urlCache = {};

  String? _getCachedUrl(String key) {
    final entry = _urlCache[key];
    if (entry == null || DateTime.now().isAfter(entry.expiresAt)) {
      _urlCache.remove(key);
      return null;
    }
    return entry.url;
  }

  void _cacheUrl(String key, String url) {
    _urlCache[key] = _CachedUrl(url, DateTime.now().add(const Duration(minutes: 100)));
  }

  Future<VideoModel?> fetchIntroVideo() async {
    try {
      final res = await _dio.get(ApiEndpoints.introVideo);
      final data = res.data['data'];
      if (data == null) return null;
      return VideoModel.fromJson(data as Map<String, dynamic>);
    } catch (_) {
      return null;
    }
  }

  Future<List<VideoModel>> fetchLatestVideos() async {
    try {
      final res = await _dio.get(ApiEndpoints.latestVideos);
      final list = res.data['data'] as List<dynamic>;
      return list.map((e) => VideoModel.fromJson(e as Map<String, dynamic>)).toList();
    } catch (_) {
      return [];
    }
  }

  Future<List<VideoModel>> fetchVideos({int page = 1, String? category}) async {
    final res = await _dio.get(
      ApiEndpoints.videos,
      queryParameters: {
        'page': page,
        if (category != null && category != 'ALL') 'category': category,
      },
    );
    final list = res.data['data'] as List<dynamic>;
    return list
        .map((e) => VideoModel.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  /// Resolves the final signed storage URL via Dio before handing it to the
  /// native video player. The Bearer token is injected automatically by
  /// DioClient's auth interceptor — no token in the URL.
  Future<String> resolveStreamUrl(String videoId) async {
    final cacheKey = '$videoId:stream';
    final cached = _getCachedUrl(cacheKey);
    if (cached != null) return cached;

    final path = ApiEndpoints.videoStream(videoId);
    try {
      final response = await DioClient().dio.get(
        path,
        options: Options(
          followRedirects: false,
          validateStatus: (s) => s != null && s < 400,
        ),
      );
      if (response.statusCode == 307 || response.statusCode == 302) {
        final location = response.headers.value('location');
        if (location != null && location.isNotEmpty) {
          _cacheUrl(cacheKey, location);
          return location;
        }
      }
    } catch (e) {
      debugPrint('resolveStreamUrl error: $e');
    }
    return '${ApiEndpoints.baseUrl}$path';
  }

  /// Returns the signed URL for a specific HLS variant playlist (e.g. "v1" = 720p).
  /// Throws if the requested variant isn't available for this video — callers
  /// must handle this explicitly (e.g. show an error) rather than silently
  /// falling back to a different quality than the one the user picked.
  Future<String> resolveVariantStreamUrl(String videoId, {required String variant}) async {
    final cacheKey = '$videoId:$variant';
    final cached = _getCachedUrl(cacheKey);
    if (cached != null) return cached;

    final path = '${ApiEndpoints.videoStreamUrl(videoId)}?variant=$variant';
    final res = await DioClient().dio.get(path);
    // Responses are wrapped: {"success": true, "data": {"url": ...}}.
    final url = (res.data['data'] as Map<String, dynamic>?)?['url'] as String?;
    if (url == null || url.isEmpty) {
      throw Exception('No stream URL returned for variant $variant');
    }
    _cacheUrl(cacheKey, url);
    return url;
  }

  /// Returns the signed URL for a per-quality MP4 download.
  /// Falls back to the original file on the server if no MP4 exists for [quality].
  Future<String> resolveDownloadUrl(String videoId, {String quality = '720p'}) async {
    final path = ApiEndpoints.videoDownloadUrl(videoId, quality);
    try {
      final res = await DioClient().dio.get(path);
      // Responses are wrapped: {"success": true, "data": {"url": ...}}.
      final url =
          (res.data['data'] as Map<String, dynamic>?)?['url'] as String?;
      if (url != null && url.isNotEmpty) return url;
    } catch (e) {
      debugPrint('resolveDownloadUrl error: $e');
    }
    return resolveStreamUrl(videoId);
  }

  /// Returns the real, available download qualities (with exact byte sizes)
  /// for this video — only tiers that actually exist are included.
  Future<List<DownloadQuality>> fetchDownloadQualities(String videoId) async {
    final res = await DioClient().dio.get(ApiEndpoints.videoQualities(videoId));
    // Responses are wrapped: {"success": true, "data": {"qualities": [...]}}.
    final list = (res.data['data']
            as Map<String, dynamic>?)?['qualities'] as List<dynamic>? ??
        [];
    return list
        .map((e) => DownloadQuality.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  /// Clears saved playback progress, removing the video from Continue
  /// Watching. Throws on failure so callers can restore optimistic UI.
  Future<void> clearProgress(String videoId) async {
    await _dio.delete('/user/videos/$videoId/progress');
  }

  /// Returns the quality tiers available for STREAMING. These come from the
  /// HLS ladder (always complete for published HLS videos), not from the
  /// per-quality MP4 download files — legacy videos can stream every quality
  /// while having no MP4s at all. Falls back to the MP4 tiers when talking to
  /// an older API that doesn't report stream_qualities yet.
  Future<List<String>> fetchStreamQualities(String videoId) async {
    final res = await DioClient().dio.get(ApiEndpoints.videoQualities(videoId));
    final data = res.data['data'] as Map<String, dynamic>?;
    final stream =
        (data?['stream_qualities'] as List<dynamic>?)?.cast<String>();
    if (stream != null && stream.isNotEmpty) return stream;
    final list = data?['qualities'] as List<dynamic>? ?? [];
    return [
      for (final e in list) (e as Map<String, dynamic>)['quality'] as String,
    ];
  }

  /// Returns the total download size per quality tier across [videoIds], for
  /// "download all" on a playlist. Only includes a tier if every video in the
  /// list actually has it ready — a tier that's only partly available would
  /// make the shown total misleading once some videos silently fall back to
  /// a different quality at download time.
  Future<List<DownloadQuality>> fetchPlaylistDownloadQualities(
      List<String> videoIds) async {
    if (videoIds.isEmpty) return const [];
    final perVideo = await Future.wait(
      videoIds.map((id) => fetchDownloadQualities(id).catchError((_) => const <DownloadQuality>[])),
    );
    if (perVideo.any((qualities) => qualities.isEmpty)) return const [];

    final totals = <String, int>{};
    for (final qualities in perVideo) {
      final byTier = {for (final q in qualities) q.quality: q.sizeBytes};
      if (totals.isEmpty) {
        totals.addAll(byTier);
      } else {
        totals.removeWhere((tier, _) => !byTier.containsKey(tier));
        for (final tier in totals.keys.toList()) {
          totals[tier] = totals[tier]! + byTier[tier]!;
        }
      }
    }
    return totals.entries
        .map((e) => DownloadQuality(quality: e.key, sizeBytes: e.value))
        .toList();
  }
}
