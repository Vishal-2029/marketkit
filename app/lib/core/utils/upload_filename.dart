import 'package:image_picker/image_picker.dart';

/// Picks a multipart filename with a supported image extension.
/// Android image_picker paths often lack an extension; the API requires .jpg/.png/.webp.
String uploadImageFilename(XFile file, {String fallback = 'image.jpg'}) {
  var name = file.name.trim();
  if (name.isEmpty) {
    final parts = file.path.replaceAll(r'\', '/').split('/');
    name = parts.isNotEmpty ? parts.last : '';
  }
  if (name.isEmpty) return fallback;
  if (_hasSupportedImageExt(name)) return name;

  final pathLower = file.path.toLowerCase();
  final base =
      name.contains('.') ? name.substring(0, name.lastIndexOf('.')) : name;
  if (pathLower.endsWith('.png')) return '$base.png';
  if (pathLower.endsWith('.webp')) return '$base.webp';
  return '$base.jpg';
}

bool _hasSupportedImageExt(String name) {
  return RegExp(r'\.(jpe?g|png|webp)$', caseSensitive: false).hasMatch(name);
}
