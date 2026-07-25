import 'dart:convert';
import 'dart:io';
import 'dart:math';
import 'dart:typed_data';

import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:pointycastle/export.dart';

/// AES-256-CTR helpers for offline video downloads.
///
/// CTR mode is used because it allows random access: to decrypt starting at an
/// arbitrary byte offset we simply advance the 128-bit counter by the number of
/// whole blocks and discard the partial first block. That is what makes seeking
/// (HTTP Range requests) work without decrypting the whole file.
class DownloadCrypto {
  DownloadCrypto._();
  static final DownloadCrypto instance = DownloadCrypto._();

  static const _keyName = 'dl_video_key';
  final _secure = const FlutterSecureStorage();
  Future<Uint8List>? _keyFuture;

  /// Cryptographically-secure random bytes (used for the key and per-file nonce).
  static Uint8List randomBytes(int n) {
    final r = Random.secure();
    final b = Uint8List(n);
    for (var i = 0; i < n; i++) {
      b[i] = r.nextInt(256);
    }
    return b;
  }

  /// Returns the single AES key, generating + persisting it once. Memoized as a
  /// Future so concurrent first-time callers share one initialization (avoids a
  /// race that could persist two different keys and corrupt a download).
  Future<Uint8List> _key() => _keyFuture ??= _loadOrCreateKey();

  Future<Uint8List> _loadOrCreateKey() async {
    final existing = await _secure.read(key: _keyName);
    if (existing != null) {
      return base64Decode(existing);
    }
    final key = randomBytes(32);
    await _secure.write(key: _keyName, value: base64Encode(key));
    return key;
  }

  /// Encrypts [source] with AES-CTR ([nonce] = initial counter) and writes the
  /// ciphertext to [sink]. [onBytes] reports cumulative bytes written. Returns
  /// the total number of bytes.
  Future<int> encryptToSink({
    required Stream<List<int>> source,
    required Uint8List nonce,
    required IOSink sink,
    void Function(int written)? onBytes,
  }) async {
    final cipher = CTRStreamCipher(AESEngine())
      ..init(true, ParametersWithIV(KeyParameter(await _key()), nonce));
    var total = 0;
    await for (final chunk in source) {
      final data = chunk is Uint8List ? chunk : Uint8List.fromList(chunk);
      final out = cipher.process(data);
      sink.add(out);
      total += out.length;
      onBytes?.call(total);
    }
    return total;
  }

  /// Streams the decrypted plaintext for the byte window [start]..[end]
  /// (inclusive) of the encrypted [file], in chunks, using CTR seeking.
  Stream<List<int>> decryptFileRange({
    required File file,
    required Uint8List nonce,
    required int start,
    required int end,
  }) async* {
    final key = await _key();
    final iv = _ctrIv(nonce, start ~/ 16);
    final cipher = CTRStreamCipher(AESEngine())
      ..init(false, ParametersWithIV(KeyParameter(key), iv));
    // Align the keystream to [start] by discarding the partial first block.
    final skip = start % 16;
    if (skip > 0) cipher.process(Uint8List(skip));

    final raf = await file.open();
    try {
      await raf.setPosition(start);
      const chunkSize = 256 * 1024;
      var pos = start;
      while (pos <= end) {
        final remaining = end - pos + 1;
        final take = remaining < chunkSize ? remaining : chunkSize;
        final data = await raf.read(take);
        if (data.isEmpty) break;
        yield cipher.process(Uint8List.fromList(data));
        pos += data.length;
      }
    } finally {
      await raf.close();
    }
  }

  /// Returns [nonce] interpreted as a 128-bit big-endian integer plus [blocks].
  Uint8List _ctrIv(Uint8List nonce, int blocks) {
    final iv = Uint8List.fromList(nonce);
    var carry = blocks;
    for (var i = iv.length - 1; i >= 0 && carry > 0; i--) {
      final sum = iv[i] + (carry & 0xff);
      iv[i] = sum & 0xff;
      carry = (carry >> 8) + (sum >> 8);
    }
    return iv;
  }
}
