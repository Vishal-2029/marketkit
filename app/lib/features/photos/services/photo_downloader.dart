import 'package:flutter_cache_manager/flutter_cache_manager.dart';
import 'package:gal/gal.dart';

/// Result of attempting to save a photo to the device gallery.
enum PhotoSaveResult { success, permissionDenied, failed }

/// Saves the image at [url] to the device photo gallery.
///
/// Reuses the on-disk cache populated by `cached_network_image`, so a photo the
/// user is already viewing is not re-downloaded. The image is saved from raw
/// bytes (so it doesn't depend on the cache file having a `.jpg` extension) into
/// the default gallery location (no custom album, so only add-only access is
/// needed — works across iOS and all Android versions).
Future<PhotoSaveResult> savePhotoToGallery(String url) async {
  try {
    if (!await Gal.hasAccess()) {
      if (!await Gal.requestAccess()) return PhotoSaveResult.permissionDenied;
    }
    final file = await DefaultCacheManager().getSingleFile(url);
    final bytes = await file.readAsBytes();
    await Gal.putImageBytes(bytes);
    return PhotoSaveResult.success;
  } on GalException {
    return PhotoSaveResult.failed;
  } catch (_) {
    return PhotoSaveResult.failed;
  }
}
