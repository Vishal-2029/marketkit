import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import 'package:shimmer/shimmer.dart';
import '../../../core/theme/app_colors.dart';
import '../models/photo_model.dart';
import '../services/photos_service.dart';
import '../services/photo_downloader.dart';

class PhotoGallery extends StatelessWidget {
  final List<PhotoModel> photos;

  const PhotoGallery({super.key, required this.photos});

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      height: 130,
      child: ListView.separated(
        scrollDirection: Axis.horizontal,
        padding: const EdgeInsets.symmetric(horizontal: 16),
        itemCount: photos.length,
        separatorBuilder: (_, __) => const SizedBox(width: 10),
        itemBuilder: (_, i) => _PhotoTile(photo: photos[i]),
      ),
    );
  }
}

class _PhotoTile extends StatelessWidget {
  final PhotoModel photo;
  const _PhotoTile({required this.photo});

  @override
  Widget build(BuildContext context) {
    final url = photo.url.isNotEmpty
        ? photo.url
        : PhotosService().photoUrl(photo.fileKey);

    return GestureDetector(
      onTap: () => _openViewer(context, url, photo.title),
      child: ClipRRect(
        borderRadius: BorderRadius.circular(12),
        child: CachedNetworkImage(
          imageUrl: url,
          width: 130,
          height: 130,
          fit: BoxFit.cover,
          placeholder: (_, __) => Shimmer.fromColors(
            baseColor: kMuted,
            highlightColor: kCard,
            child: Container(width: 130, height: 130, color: kMuted),
          ),
          errorWidget: (_, __, ___) => Container(
            width: 130,
            height: 130,
            color: kCard,
            child: const Icon(Icons.broken_image_outlined,
                color: kMutedForeground, size: 32),
          ),
        ),
      ),
    );
  }

  void _openViewer(BuildContext context, String url, String title) {
    Navigator.of(context).push(
      MaterialPageRoute(
        builder: (_) => _PhotoViewerScreen(url: url, title: title),
      ),
    );
  }
}

class _PhotoViewerScreen extends StatefulWidget {
  final String url;
  final String title;

  const _PhotoViewerScreen({required this.url, required this.title});

  @override
  State<_PhotoViewerScreen> createState() => _PhotoViewerScreenState();
}

class _PhotoViewerScreenState extends State<_PhotoViewerScreen> {
  bool _saving = false;

  Future<void> _download() async {
    if (_saving) return;
    setState(() => _saving = true);

    final result = await savePhotoToGallery(widget.url);

    if (!mounted) return;
    const messages = {
      PhotoSaveResult.success: 'Saved to gallery',
      PhotoSaveResult.permissionDenied: 'Allow photo access to download',
      PhotoSaveResult.failed: 'Download failed',
    };
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(messages[result]!)),
    );
    setState(() => _saving = false);
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.black,
      appBar: AppBar(
        backgroundColor: Colors.black,
        foregroundColor: Colors.white,
        title: Text(widget.title,
            style: const TextStyle(fontSize: 15, fontWeight: FontWeight.w600)),
        actions: [
          _saving
              ? const Padding(
                  padding: EdgeInsets.only(right: 16),
                  child: Center(
                    child: SizedBox(
                      width: 18,
                      height: 18,
                      child: CircularProgressIndicator(
                          strokeWidth: 2, color: Colors.white),
                    ),
                  ),
                )
              : IconButton(
                  icon: const Icon(Icons.download_rounded),
                  color: Colors.white,
                  tooltip: 'Download',
                  onPressed: _download,
                ),
        ],
      ),
      body: Center(
        child: InteractiveViewer(
          child: CachedNetworkImage(
            imageUrl: widget.url,
            fit: BoxFit.contain,
            placeholder: (_, __) =>
                const CircularProgressIndicator(color: kGold),
            errorWidget: (_, __, ___) => const Icon(Icons.broken_image_outlined,
                color: Colors.white54, size: 56),
          ),
        ),
      ),
    );
  }
}
