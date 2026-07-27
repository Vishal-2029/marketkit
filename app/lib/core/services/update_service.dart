import 'package:dio/dio.dart';
import 'package:open_filex/open_filex.dart';
import 'package:package_info_plus/package_info_plus.dart';
import 'package:path_provider/path_provider.dart';

import '../network/api_endpoints.dart';

class AppVersionInfo {
  final String id;
  final String versionName;
  final int buildNumber;
  final String downloadUrl;
  final String releaseNotes;
  final bool isMandatory;

  const AppVersionInfo({
    required this.id,
    required this.versionName,
    required this.buildNumber,
    required this.downloadUrl,
    required this.releaseNotes,
    required this.isMandatory,
  });

  factory AppVersionInfo.fromJson(Map<String, dynamic> j) => AppVersionInfo(
        id: j['id'] as String,
        versionName: j['version_name'] as String,
        buildNumber: j['build_number'] as int,
        downloadUrl: j['download_url'] as String,
        releaseNotes: (j['release_notes'] as String?) ?? '',
        isMandatory: (j['is_mandatory'] as bool?) ?? false,
      );
}

class UpdateService {
  UpdateService._();
  static final UpdateService instance = UpdateService._();

  // Own plain Dio — no auth interceptors needed for a public endpoint.
  final _dio = Dio(BaseOptions(
    connectTimeout: const Duration(seconds: 8),
    receiveTimeout: const Duration(seconds: 10),
  ));

  /// Cached result of the last [checkForUpdate] call.
  /// Non-null means a newer build is available. Profile screen reads this
  /// instead of making a second network call.
  AppVersionInfo? latestUpdate;

  /// Returns [AppVersionInfo] if the server has a newer build, otherwise null.
  /// Never throws — network/parse errors return null so the app always starts.
  Future<AppVersionInfo?> checkForUpdate() async {
    try {
      final info = await PackageInfo.fromPlatform();
      final currentBuild = int.tryParse(info.buildNumber) ?? 0;

      final res = await _dio.get(
        '${ApiEndpoints.baseUrl}${ApiEndpoints.appVersion}',
      );
      if (res.data['success'] != true || res.data['data'] == null) return null;

      final remote =
          AppVersionInfo.fromJson(res.data['data'] as Map<String, dynamic>);
      // Not a strict ">" — the server always returns the most recently published
      // build (see HandleGetLatest), which can have a lower build_number than what's
      // installed after a version-numbering reset. Any mismatch means "different
      // from what's running," which is what we want to prompt on.
      latestUpdate = remote.buildNumber != currentBuild ? remote : null;
      return latestUpdate;
    } catch (_) {
      return null;
    }
  }

  /// Downloads the APK to the temp directory, reporting progress as 0.0–1.0.
  Future<String> downloadApk(
    String url, {
    void Function(double)? onProgress,
    CancelToken? cancelToken,
  }) async {
    final dir = await getTemporaryDirectory();
    final savePath = '${dir.path}/marketkit_update.apk';

    await _dio.download(
      url,
      savePath,
      cancelToken: cancelToken,
      onReceiveProgress: (received, total) {
        if (total > 0) onProgress?.call(received / total);
      },
    );
    return savePath;
  }

  /// Opens the downloaded APK with the Android system package installer.
  Future<void> installApk(String filePath) async {
    await OpenFilex.open(
      filePath,
      type: 'application/vnd.android.package-archive',
    );
  }
}
