import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../home/models/video_model.dart';
import '../models/download_entry.dart';
import '../services/downloads_service.dart';

class DownloadsState {
  final List<DownloadEntry> entries;
  final Map<String, double> progress; // id -> 0..1 while downloading

  const DownloadsState({this.entries = const [], this.progress = const {}});

  DownloadsState copyWith({
    List<DownloadEntry>? entries,
    Map<String, double>? progress,
  }) =>
      DownloadsState(
        entries: entries ?? this.entries,
        progress: progress ?? this.progress,
      );

  bool isDownloaded(String id) => entries.any((e) => e.id == id);
  bool isDownloading(String id) => progress.containsKey(id);
  double progressFor(String id) => progress[id] ?? 0;
}

class DownloadsNotifier extends StateNotifier<DownloadsState> {
  DownloadsNotifier() : super(const DownloadsState()) {
    _init();
  }

  final _service = DownloadsService.instance;
  final Map<String, CancelToken> _tokens = {};

  Future<void> _init() async {
    state = state.copyWith(entries: await _service.list());
  }

  Future<void> refresh() async {
    state = state.copyWith(entries: await _service.list());
  }

  void _setProgress(String id, double p) {
    state = state.copyWith(progress: {...state.progress, id: p});
  }

  void _clearProgress(String id) {
    final next = {...state.progress}..remove(id);
    state = state.copyWith(progress: next);
  }

  /// Downloads [video] at [quality]. Throws on failure so the caller can show an error.
  Future<void> startDownload(VideoModel video, {String quality = '720p'}) async {
    final backendQuality = quality;
    if (state.isDownloaded(video.id) || state.isDownloading(video.id)) return;
    final token = CancelToken();
    _tokens[video.id] = token;
    _setProgress(video.id, 0);
    try {
      await _service.download(
        video,
        quality: backendQuality,
        cancelToken: token,
        onProgress: (p) => _setProgress(video.id, p),
      );
      state = state.copyWith(entries: await _service.list());
    } finally {
      _tokens.remove(video.id);
      _clearProgress(video.id);
    }
  }

  void cancel(String id) {
    _tokens[id]?.cancel();
    _tokens.remove(id);
    _clearProgress(id);
  }

  bool _bulkCancelled = false;

  /// Downloads every accessible, not-yet-downloaded video in [videos] one at a
  /// time (sequential — avoids saturating bandwidth/storage on large playlists).
  /// A single video's failure doesn't stop the rest. Call [cancelBulkDownload]
  /// to stop after the in-flight video finishes.
  Future<void> startPlaylistDownload(
    List<VideoModel> videos, {
    String quality = '720p',
  }) async {
    _bulkCancelled = false;
    for (final v in videos) {
      if (_bulkCancelled) break;
      if (!v.accessible) continue;
      if (state.isDownloaded(v.id) || state.isDownloading(v.id)) continue;
      try {
        await startDownload(v, quality: quality);
      } catch (_) {
        // Skip this video and keep going with the rest of the playlist.
      }
    }
  }

  void cancelBulkDownload() {
    _bulkCancelled = true;
    // Also stop whatever video is downloading right now instead of letting
    // it finish before the batch actually stops — at most one entry is ever
    // in `progress` during a sequential playlist download.
    for (final id in state.progress.keys.toList()) {
      cancel(id);
    }
  }

  Future<void> remove(String id) async {
    await _service.delete(id);
    state = state.copyWith(entries: await _service.list());
  }
}

final downloadsProvider =
    StateNotifierProvider<DownloadsNotifier, DownloadsState>(
        (ref) => DownloadsNotifier());
