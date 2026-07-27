import 'package:marketkit/core/config/feature_catalog.dart';
import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shimmer/shimmer.dart';
import '../../../core/theme/app_colors.dart';
import '../models/photo_model.dart';
import '../providers/photos_provider.dart';
import '../services/photos_service.dart';
import '../services/photo_downloader.dart';

class GalleryScreen extends ConsumerWidget {
  const GalleryScreen({super.key});

  static final _categoryOrder = FeatureCatalog.postCategoryKeys;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final state = ref.watch(photosProvider);

    return Scaffold(
      backgroundColor: kBackground,
      appBar: AppBar(
        backgroundColor: kBackground,
        foregroundColor: kForeground,
        elevation: 0,
        title: const Text(
          'Gallery',
          style: TextStyle(fontSize: 17, fontWeight: FontWeight.w700),
        ),
      ),
      body: state.isLoading
          ? _Shimmer()
          : _FolderGrid(photos: state.photos),
    );
  }
}

class _FolderGrid extends StatelessWidget {
  final List<PhotoModel> photos;
  const _FolderGrid({required this.photos});

  @override
  Widget build(BuildContext context) {
    // Group by category
    final Map<String, List<PhotoModel>> byCategory = {};
    for (final p in photos) {
      byCategory.putIfAbsent(p.category, () => []).add(p);
    }

    // Sort: known categories first, then any extras
    final categories = [
      ...GalleryScreen._categoryOrder.where(byCategory.containsKey),
      ...byCategory.keys.where((k) => !GalleryScreen._categoryOrder.contains(k)),
    ];

    if (categories.isEmpty) {
      return const Center(
        child: Text('No photos yet', style: TextStyle(color: kMutedForeground)),
      );
    }

    return GridView.builder(
      padding: const EdgeInsets.all(16),
      gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
        crossAxisCount: 2,
        crossAxisSpacing: 12,
        mainAxisSpacing: 12,
      ),
      itemCount: categories.length,
      itemBuilder: (context, i) {
        final cat = categories[i];
        final catPhotos = byCategory[cat]!;
        return _FolderCard(
          category: cat,
          photos: catPhotos,
          onTap: () => Navigator.of(context).push(
            MaterialPageRoute(
              builder: (_) => GalleryFolderScreen(
                category: cat,
                photos: catPhotos,
              ),
            ),
          ),
        );
      },
    );
  }
}

class _FolderCard extends StatelessWidget {
  final String category;
  final List<PhotoModel> photos;
  final VoidCallback onTap;

  const _FolderCard({
    required this.category,
    required this.photos,
    required this.onTap,
  });

  String get _displayName => FeatureCatalog.label(category);

  String _resolvedUrl(PhotoModel p) =>
      p.url.isNotEmpty ? p.url : PhotosService().photoUrl(p.fileKey);

  @override
  Widget build(BuildContext context) {
    final coverUrl = _resolvedUrl(photos.first);

    return GestureDetector(
      onTap: onTap,
      child: ClipRRect(
        borderRadius: BorderRadius.circular(14),
        child: Stack(
          fit: StackFit.expand,
          children: [
            CachedNetworkImage(
              imageUrl: coverUrl,
              fit: BoxFit.cover,
              placeholder: (context, url) => Container(color: kMuted),
              errorWidget: (context, url, error) => Container(
                color: kCard,
                child: const Icon(Icons.photo_library_outlined,
                    color: kMutedForeground, size: 32),
              ),
            ),
            // Dark gradient at the bottom
            Container(
              decoration: BoxDecoration(
                gradient: LinearGradient(
                  begin: Alignment.topCenter,
                  end: Alignment.bottomCenter,
                  colors: [
                    Colors.transparent,
                    Colors.black.withValues(alpha: 0.65),
                  ],
                ),
              ),
            ),
            Positioned(
              bottom: 10,
              left: 12,
              right: 12,
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    _displayName,
                    style: const TextStyle(
                      color: Colors.white,
                      fontSize: 14,
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                  const SizedBox(height: 2),
                  Text(
                    '${photos.length} photo${photos.length != 1 ? 's' : ''}',
                    style: const TextStyle(color: Colors.white70, fontSize: 11),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _Shimmer extends StatelessWidget {
  @override
  Widget build(BuildContext context) {
    return Shimmer.fromColors(
      baseColor: kMuted,
      highlightColor: kCard,
      child: GridView.count(
        crossAxisCount: 2,
        crossAxisSpacing: 12,
        mainAxisSpacing: 12,
        padding: const EdgeInsets.all(16),
        children: List.generate(
          4,
          (_) => Container(
            decoration: BoxDecoration(
              color: kMuted,
              borderRadius: BorderRadius.circular(14),
            ),
          ),
        ),
      ),
    );
  }
}

// ──────────────────────────────────────────────────────────────────────────────

class GalleryFolderScreen extends StatelessWidget {
  final String category;
  final List<PhotoModel> photos;

  const GalleryFolderScreen({
    super.key,
    required this.category,
    required this.photos,
  });

  String get _displayName => FeatureCatalog.label(category);

  String _resolvedUrl(PhotoModel p) =>
      p.url.isNotEmpty ? p.url : PhotosService().photoUrl(p.fileKey);

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: kBackground,
      appBar: AppBar(
        backgroundColor: kBackground,
        foregroundColor: kForeground,
        elevation: 0,
        title: Text(
          _displayName,
          style: const TextStyle(fontSize: 17, fontWeight: FontWeight.w700),
        ),
      ),
      body: GridView.builder(
        padding: const EdgeInsets.all(10),
        gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
          crossAxisCount: 3,
          crossAxisSpacing: 6,
          mainAxisSpacing: 6,
        ),
        itemCount: photos.length,
        itemBuilder: (context, i) {
          final url = _resolvedUrl(photos[i]);
          return GestureDetector(
            onTap: () => Navigator.of(context).push(
              MaterialPageRoute(
                fullscreenDialog: true,
                builder: (_) => _GalleryViewer(
                  photos: photos,
                  initialIndex: i,
                  resolveUrl: _resolvedUrl,
                ),
              ),
            ),
            child: ClipRRect(
              borderRadius: BorderRadius.circular(8),
              child: CachedNetworkImage(
                imageUrl: url,
                fit: BoxFit.cover,
                placeholder: (context, url) => Container(color: kMuted),
                errorWidget: (context, url, error) => Container(
                  color: kCard,
                  child: const Icon(Icons.broken_image_outlined,
                      color: kMutedForeground),
                ),
              ),
            ),
          );
        },
      ),
    );
  }
}

// ──────────────────────────────────────────────────────────────────────────────

class _GalleryViewer extends StatefulWidget {
  final List<PhotoModel> photos;
  final int initialIndex;
  final String Function(PhotoModel) resolveUrl;

  const _GalleryViewer({
    required this.photos,
    required this.initialIndex,
    required this.resolveUrl,
  });

  @override
  State<_GalleryViewer> createState() => _GalleryViewerState();
}

class _GalleryViewerState extends State<_GalleryViewer> {
  late final PageController _pageCtrl;
  late int _current;
  bool _saving = false;

  @override
  void initState() {
    super.initState();
    _current = widget.initialIndex;
    _pageCtrl = PageController(initialPage: widget.initialIndex);
  }

  @override
  void dispose() {
    _pageCtrl.dispose();
    super.dispose();
  }

  Future<void> _downloadCurrent() async {
    if (_saving) return;
    setState(() => _saving = true);

    final result =
        await savePhotoToGallery(widget.resolveUrl(widget.photos[_current]));

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
        elevation: 0,
        title: widget.photos.length > 1
            ? Text(
                '${_current + 1} / ${widget.photos.length}',
                style: const TextStyle(color: Colors.white70, fontSize: 14),
              )
            : Text(
                widget.photos[_current].title,
                style: const TextStyle(fontSize: 14, fontWeight: FontWeight.w500),
              ),
        actions: [
          _saving
              ? const Padding(
                  padding: EdgeInsets.only(right: 16),
                  child: SizedBox(
                    width: 20,
                    height: 20,
                    child: Center(
                      child: SizedBox(
                        width: 18,
                        height: 18,
                        child: CircularProgressIndicator(
                            strokeWidth: 2, color: Colors.white),
                      ),
                    ),
                  ),
                )
              : IconButton(
                  icon: const Icon(Icons.download_rounded),
                  color: Colors.white,
                  tooltip: 'Download',
                  onPressed: _downloadCurrent,
                ),
        ],
      ),
      body: PageView.builder(
        controller: _pageCtrl,
        itemCount: widget.photos.length,
        onPageChanged: (i) => setState(() => _current = i),
        itemBuilder: (context, i) => InteractiveViewer(
          child: Center(
            child: CachedNetworkImage(
              imageUrl: widget.resolveUrl(widget.photos[i]),
              fit: BoxFit.contain,
              placeholder: (context, url) =>
                  const CircularProgressIndicator(color: kPrimary),
              errorWidget: (context, url, error) => const Icon(
                  Icons.broken_image_outlined,
                  color: Colors.white54,
                  size: 56),
            ),
          ),
        ),
      ),
    );
  }
}
