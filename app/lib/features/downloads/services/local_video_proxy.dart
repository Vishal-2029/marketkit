import 'dart:convert';
import 'dart:io';

import 'download_crypto.dart';
import 'downloads_service.dart';

/// A tiny loopback HTTP server that streams the on-the-fly *decrypted* bytes of a
/// downloaded video to the platform player. The plaintext only ever exists in
/// memory while serving a request — it is never written to disk.
class LocalVideoProxy {
  LocalVideoProxy._();
  static final LocalVideoProxy instance = LocalVideoProxy._();

  HttpServer? _server;
  int _port = 0;

  Future<int> ensureStarted() async {
    if (_server != null) return _port;
    final server = await HttpServer.bind(InternetAddress.loopbackIPv4, 0);
    _server = server;
    _port = server.port;
    server.listen(_handle);
    return _port;
  }

  String urlFor(String id) => 'http://127.0.0.1:$_port/$id';

  Future<void> _handle(HttpRequest req) async {
    final res = req.response;
    try {
      final id =
          req.uri.pathSegments.isNotEmpty ? req.uri.pathSegments.last : '';
      final entry = await DownloadsService.instance.entry(id);
      final file = DownloadsService.instance.encFileFor(id);
      if (entry == null || !await file.exists()) {
        res.statusCode = HttpStatus.notFound;
        await res.close();
        return;
      }

      final nonce = base64Decode(entry.nonceB64);
      final total = await file.length();

      res.headers.set(HttpHeaders.acceptRangesHeader, 'bytes');
      res.headers.contentType = ContentType.parse(
          entry.mimeType.isNotEmpty ? entry.mimeType : 'video/mp4');

      var start = 0;
      var end = total - 1;
      final rangeHeader = req.headers.value(HttpHeaders.rangeHeader);
      if (rangeHeader != null && rangeHeader.startsWith('bytes=')) {
        final spec = rangeHeader.substring(6).split('-');
        final hasStart = spec.isNotEmpty && spec[0].isNotEmpty;
        final hasEnd = spec.length > 1 && spec[1].isNotEmpty;
        if (hasStart) {
          start = int.tryParse(spec[0]) ?? 0;
          if (hasEnd) end = int.tryParse(spec[1]) ?? end;
        } else if (hasEnd) {
          // Suffix range "bytes=-N" → the last N bytes.
          final n = int.tryParse(spec[1]) ?? total;
          start = n >= total ? 0 : total - n;
        }
        if (end >= total) end = total - 1;
        if (start < 0 || start > end) start = 0;
        res.statusCode = HttpStatus.partialContent; // 206
        res.headers
            .set(HttpHeaders.contentRangeHeader, 'bytes $start-$end/$total');
      } else {
        res.statusCode = HttpStatus.ok; // 200
      }

      res.headers
          .set(HttpHeaders.contentLengthHeader, (end - start + 1).toString());

      await for (final chunk in DownloadCrypto.instance.decryptFileRange(
        file: file,
        nonce: nonce,
        start: start,
        end: end,
      )) {
        res.add(chunk);
        await res.flush();
      }
      await res.close();
    } catch (_) {
      try {
        res.statusCode = HttpStatus.internalServerError;
        await res.close();
      } catch (_) {}
    }
  }
}
