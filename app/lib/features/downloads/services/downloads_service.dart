import 'dart:convert';
import 'dart:io';

import 'package:dio/dio.dart';
import 'package:path_provider/path_provider.dart';

import '../../../core/network/api_endpoints.dart';
import '../../../core/network/dio_client.dart';
import '../../home/models/video_model.dart';
import '../../home/services/videos_service.dart';
import '../models/download_entry.dart';
import 'download_crypto.dart';

/// Stores videos AES-encrypted in the app's private sandbox and manages the
/// local index. Nothing is written anywhere the user (or other apps) can reach.
class DownloadsService {
  DownloadsService._();
  static final DownloadsService instance = DownloadsService._();

  bool _loaded = false;
  String _basePath = '';
  List<DownloadEntry> _entries = [];

  Future<void> ensureLoaded() async {
    if (_loaded) return;
    final base = await getApplicationSupportDirectory();
    final dir = Directory('${base.path}/downloads');
    if (!await dir.exists()) await dir.create(recursive: true);
    _basePath = dir.path;

    final idx = File('$_basePath/index.json');
    if (await idx.exists()) {
      try {
        final raw = jsonDecode(await idx.readAsString()) as List<dynamic>;
        _entries = raw
            .map((e) => DownloadEntry.fromJson(e as Map<String, dynamic>))
            .toList();
      } catch (_) {
        _entries = [];
      }
    }
    _loaded = true;
  }

  File encFileFor(String id) => File('$_basePath/$id.enc');
  File _thumbFileFor(String id) => File('$_basePath/$id.jpg');
  File get _indexFile => File('$_basePath/index.json');

  Future<List<DownloadEntry>> list() async {
    await ensureLoaded();
    return List.unmodifiable(_entries);
  }

  Future<DownloadEntry?> entry(String id) async {
    await ensureLoaded();
    for (final e in _entries) {
      if (e.id == id) return e;
    }
    return null;
  }

  Future<bool> isDownloaded(String id) async {
    await ensureLoaded();
    return _entries.any((e) => e.id == id) && await encFileFor(id).exists();
  }

  /// Downloads, encrypts, and stores [video] at the given [quality].
  /// Throws if the video is not accessible or the network fetch fails.
  Future<DownloadEntry> download(
    VideoModel video, {
    String quality = '720p',
    void Function(double progress)? onProgress,
    CancelToken? cancelToken,
  }) async {
    if (!video.accessible) {
      throw StateError('This video is not available for download.');
    }
    await ensureLoaded();

    final url = await VideosService().resolveDownloadUrl(video.id, quality: quality);
    final nonce = DownloadCrypto.randomBytes(16);
    final encFile = encFileFor(video.id);
    final partFile = File('${encFile.path}.part');

    try {
      // A signed R2 URL needs no auth; the API stream fallback does — use the
      // authenticated client only in that case.
      final dio =
          url.startsWith(ApiEndpoints.baseUrl) ? DioClient().dio : Dio();
      final resp = await dio.get<ResponseBody>(
        url,
        options: Options(responseType: ResponseType.stream),
        cancelToken: cancelToken,
      );
      final total = int.tryParse(
              resp.headers.value(Headers.contentLengthHeader) ?? '') ??
          0;
      final contentType =
          resp.headers.value(Headers.contentTypeHeader)?.split(';').first.trim();
      final mimeType = (contentType != null && contentType.startsWith('video/'))
          ? contentType
          : 'video/mp4';

      final sink = partFile.openWrite();
      var written = 0;
      try {
        written = await DownloadCrypto.instance.encryptToSink(
          source: resp.data!.stream,
          nonce: nonce,
          sink: sink,
          onBytes: (n) {
            if (total > 0) onProgress?.call((n / total).clamp(0.0, 1.0));
          },
        );
      } finally {
        await sink.close();
      }

      if (await encFile.exists()) await encFile.delete();
      await partFile.rename(encFile.path);

      // Cache the thumbnail for offline display (best-effort, not sensitive).
      String? thumbPath;
      final thumbUrl = video.thumbnailUrl;
      if (thumbUrl != null && thumbUrl.isNotEmpty && thumbUrl.startsWith('http')) {
        try {
          final tf = _thumbFileFor(video.id);
          await Dio().download(thumbUrl, tf.path);
          thumbPath = tf.path;
        } catch (_) {}
      }

      final entry = DownloadEntry(
        id: video.id,
        title: video.title,
        description: video.description,
        category: video.category,
        durationSeconds: video.durationSeconds,
        nonceB64: base64Encode(nonce),
        thumbnailPath: thumbPath,
        sizeBytes: written,
        mimeType: mimeType,
        quality: quality,
        downloadedAt: DateTime.now(),
      );
      _entries.removeWhere((e) => e.id == entry.id);
      _entries.insert(0, entry);
      await _saveIndex();
      return entry;
    } catch (e) {
      if (await partFile.exists()) {
        try {
          await partFile.delete();
        } catch (_) {}
      }
      rethrow;
    }
  }

  Future<void> delete(String id) async {
    await ensureLoaded();
    final enc = encFileFor(id);
    if (await enc.exists()) await enc.delete();
    final th = _thumbFileFor(id);
    if (await th.exists()) await th.delete();
    _entries.removeWhere((e) => e.id == id);
    await _saveIndex();
  }

  Future<void> _saveIndex() async {
    await _indexFile
        .writeAsString(jsonEncode(_entries.map((e) => e.toJson()).toList()));
  }
}
