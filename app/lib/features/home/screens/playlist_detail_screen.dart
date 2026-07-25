import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:shimmer/shimmer.dart';
import '../../../core/theme/app_colors.dart';
import '../../downloads/models/download_quality.dart';
import '../../downloads/providers/downloads_provider.dart';
import '../../downloads/widgets/download_button.dart';
import '../../favorites/widgets/favorite_button.dart';
import '../models/playlist_model.dart';
import '../models/video_model.dart';
import '../services/playlists_service.dart';
import '../services/videos_service.dart';

class PlaylistDetailScreen extends ConsumerStatefulWidget {
  final PlaylistModel playlist;

  const PlaylistDetailScreen({super.key, required this.playlist});

  @override
  ConsumerState<PlaylistDetailScreen> createState() =>
      _PlaylistDetailScreenState();
}

class _PlaylistDetailScreenState extends ConsumerState<PlaylistDetailScreen> {
  bool _loading = true;
  bool _bulkDownloading = false;
  PlaylistModel? _detail;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() => _loading = true);
    final detail =
        await PlaylistsService().fetchPlaylistDetail(widget.playlist.id);
    if (!mounted) return;
    setState(() {
      _detail = detail;
      _loading = false;
    });
  }

  void _onVideoTap(VideoModel video) {
    if (!video.accessible) {
      _showUpgradeDialog();
      return;
    }
    context.push('/video/${video.id}', extra: video);
  }

  static const _fallbackQualities = ['360p', '720p', '1080p'];

  String _formatSize(int sizeBytes) {
    final mb = sizeBytes / (1024 * 1024);
    return '${mb.toStringAsFixed(0)} MB';
  }

  Future<void> _downloadAll() async {
    final videos = (_detail?.videos ?? const <VideoModel>[])
        .where((v) => v.accessible)
        .toList();
    if (videos.isEmpty) return;

    final quality = await _showPlaylistQualityPicker(videos);
    if (quality == null || !mounted) return;

    setState(() => _bulkDownloading = true);
    try {
      await ref
          .read(downloadsProvider.notifier)
          .startPlaylistDownload(videos, quality: quality);
    } finally {
      if (mounted) setState(() => _bulkDownloading = false);
    }
  }

  Future<String?> _showPlaylistQualityPicker(List<VideoModel> videos) {
    final notDownloaded = videos
        .where((v) => !ref.read(downloadsProvider).isDownloaded(v.id))
        .map((v) => v.id)
        .toList();

    return showModalBottomSheet<String>(
      context: context,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(16)),
      ),
      builder: (ctx) => SafeArea(
        child: Padding(
          padding: const EdgeInsets.fromLTRB(16, 12, 16, 8),
          child: FutureBuilder<List<DownloadQuality>>(
            future:
                VideosService().fetchPlaylistDownloadQualities(notDownloaded),
            builder: (context, snapshot) {
              final isLoading =
                  snapshot.connectionState == ConnectionState.waiting;
              final qualities = snapshot.data;
              final failed = snapshot.hasError || (qualities?.isEmpty ?? false);

              final List<Widget> optionTiles;
              if (isLoading) {
                optionTiles = const [
                  Padding(
                    padding: EdgeInsets.symmetric(vertical: 24),
                    child: Center(
                      child: CircularProgressIndicator(color: kGold, strokeWidth: 2),
                    ),
                  ),
                ];
              } else if (failed) {
                // Not every video has a matching size for the same tier yet
                // (e.g. one is still processing) — fall back to a plain
                // quality list, same as the single-video picker does.
                optionTiles = _fallbackQualities
                    .map((q) => ListTile(
                          contentPadding: EdgeInsets.zero,
                          leading: const Icon(Icons.high_quality_outlined, color: kGold),
                          title: Text(q,
                              style: const TextStyle(
                                  fontWeight: FontWeight.w600, fontSize: 14)),
                          onTap: () => Navigator.pop(ctx, q),
                        ))
                    .toList();
              } else {
                optionTiles = qualities!
                    .map((q) => ListTile(
                          contentPadding: EdgeInsets.zero,
                          leading: const Icon(Icons.high_quality_outlined, color: kGold),
                          title: Text(q.quality,
                              style: const TextStyle(
                                  fontWeight: FontWeight.w600, fontSize: 14)),
                          trailing: Text(_formatSize(q.sizeBytes),
                              style: const TextStyle(
                                  fontSize: 12, color: kMutedForeground)),
                          onTap: () => Navigator.pop(ctx, q.quality),
                        ))
                    .toList();
              }

              return Column(
                mainAxisSize: MainAxisSize.min,
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  const Text(
                    'Download quality',
                    style: TextStyle(fontSize: 16, fontWeight: FontWeight.w700),
                  ),
                  const SizedBox(height: 4),
                  Text(
                    'Total size for all ${notDownloaded.length} video(s)',
                    style: const TextStyle(fontSize: 12, color: kMutedForeground),
                  ),
                  const SizedBox(height: 8),
                  ...optionTiles,
                ],
              );
            },
          ),
        ),
      ),
    );
  }

  void _cancelBulkDownload() {
    ref.read(downloadsProvider.notifier).cancelBulkDownload();
  }

  void _showUpgradeDialog() {
    showDialog(
      context: context,
      builder: (_) => AlertDialog(
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
        title: const Text('Subscription Required',
            style: TextStyle(fontWeight: FontWeight.w700)),
        content: const Text(
            'This video requires an active subscription plan. Upgrade to unlock all content.'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('Cancel',
                style: TextStyle(color: kMutedForeground)),
          ),
          ElevatedButton(
            onPressed: () {
              Navigator.pop(context);
              context.go('/plans');
            },
            child: const Text('View Plans'),
          ),
        ],
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final playlist = _detail ?? widget.playlist;
    final downloadableVideos =
        (_detail?.videos ?? const <VideoModel>[]).where((v) => v.accessible).toList();
    final dlState = ref.watch(downloadsProvider);
    final downloadedCount =
        downloadableVideos.where((v) => dlState.isDownloaded(v.id)).length;
    final allDownloaded = downloadableVideos.isNotEmpty &&
        downloadedCount == downloadableVideos.length;

    return Scaffold(
      backgroundColor: kBackground,
      body: CustomScrollView(
        slivers: [
          // Header with thumbnail + title
          SliverAppBar(
            expandedHeight: 200,
            pinned: true,
            backgroundColor: kBackground,
            flexibleSpace: FlexibleSpaceBar(
              background: playlist.thumbnailUrl.isNotEmpty
                  ? CachedNetworkImage(
                      imageUrl: playlist.thumbnailUrl,
                      fit: BoxFit.cover,
                      placeholder: (_, __) => Container(color: kMuted),
                      errorWidget: (_, __, ___) => Container(
                        color: kMuted,
                        child: const Icon(Icons.video_library_rounded,
                            color: kMutedForeground, size: 48),
                      ),
                    )
                  : Container(
                      color: kMuted,
                      child: const Icon(Icons.video_library_rounded,
                          color: kMutedForeground, size: 48),
                    ),
            ),
          ),

          // Playlist info
          SliverToBoxAdapter(
            child: Padding(
              padding: const EdgeInsets.fromLTRB(16, 16, 16, 4),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    playlist.name,
                    style: const TextStyle(
                      fontSize: 20,
                      fontWeight: FontWeight.w700,
                      color: kForeground,
                    ),
                  ),
                  if (playlist.description.isNotEmpty) ...[
                    const SizedBox(height: 6),
                    Text(
                      playlist.description,
                      style: const TextStyle(
                          fontSize: 13, color: kMutedForeground, height: 1.4),
                    ),
                  ],
                  const SizedBox(height: 8),
                  Text(
                    '${_loading ? playlist.videoCount : (_detail?.videos.length ?? 0)} videos',
                    style: const TextStyle(
                        fontSize: 12, color: kMutedForeground),
                  ),
                  if (!_loading && downloadableVideos.isNotEmpty) ...[
                    const SizedBox(height: 12),
                    _downloadAllRow(downloadedCount, downloadableVideos.length,
                        allDownloaded),
                  ],
                  const SizedBox(height: 16),
                  const Divider(color: kBorder, height: 1),
                ],
              ),
            ),
          ),

          // Video list
          if (_loading)
            SliverList(
              delegate: SliverChildBuilderDelegate(
                (_, __) => _shimmerTile(),
                childCount: 5,
              ),
            )
          else if (_detail == null || _detail!.videos.isEmpty)
            const SliverFillRemaining(
              child: Center(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Icon(Icons.video_library_outlined,
                        size: 48, color: kMutedForeground),
                    SizedBox(height: 12),
                    Text('No videos in this playlist yet.',
                        style:
                            TextStyle(fontSize: 14, color: kMutedForeground)),
                  ],
                ),
              ),
            )
          else
            SliverList(
              delegate: SliverChildBuilderDelegate(
                (_, i) {
                  final v = _detail!.videos[i];
                  return _VideoTile(
                    video: v,
                    index: i + 1,
                    onTap: () => _onVideoTap(v),
                  );
                },
                childCount: _detail!.videos.length,
              ),
            ),

          const SliverToBoxAdapter(child: SizedBox(height: 32)),
        ],
      ),
    );
  }

  Widget _downloadAllRow(int downloadedCount, int totalCount, bool allDownloaded) {
    if (allDownloaded) {
      return Row(
        children: const [
          Icon(Icons.download_done_rounded, size: 16, color: kGold),
          SizedBox(width: 6),
          Text('All videos downloaded',
              style: TextStyle(fontSize: 12, fontWeight: FontWeight.w600, color: kGold)),
        ],
      );
    }

    if (_bulkDownloading) {
      return Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          ClipRRect(
            borderRadius: BorderRadius.circular(4),
            child: LinearProgressIndicator(
              value: totalCount > 0 ? downloadedCount / totalCount : 0,
              minHeight: 4,
              backgroundColor: kMuted,
              color: kGold,
            ),
          ),
          const SizedBox(height: 6),
          Row(
            children: [
              Text('Downloading $downloadedCount/$totalCount…',
                  style: const TextStyle(fontSize: 12, color: kMutedForeground)),
              const Spacer(),
              GestureDetector(
                onTap: _cancelBulkDownload,
                child: const Text('Cancel',
                    style: TextStyle(
                        fontSize: 12,
                        fontWeight: FontWeight.w600,
                        color: kMutedForeground)),
              ),
            ],
          ),
        ],
      );
    }

    final remaining = totalCount - downloadedCount;
    return Align(
      alignment: Alignment.centerRight,
      child: OutlinedButton.icon(
        onPressed: _downloadAll,
        style: OutlinedButton.styleFrom(
          foregroundColor: kGold,
          side: const BorderSide(color: kGold),
          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
          minimumSize: Size.zero,
          tapTargetSize: MaterialTapTargetSize.shrinkWrap,
        ),
        icon: const Icon(Icons.download_outlined, size: 16),
        label: Text(
          downloadedCount > 0
              ? 'Download remaining ($remaining)'
              : 'Download all ($totalCount)',
          style: const TextStyle(fontSize: 12, fontWeight: FontWeight.w600),
        ),
      ),
    );
  }

  Widget _shimmerTile() {
    return Shimmer.fromColors(
      baseColor: kMuted,
      highlightColor: kCard,
      child: Container(
        height: 72,
        margin: const EdgeInsets.symmetric(horizontal: 16, vertical: 6),
        decoration: BoxDecoration(
          color: kMuted,
          borderRadius: BorderRadius.circular(10),
        ),
      ),
    );
  }
}

class _VideoTile extends StatelessWidget {
  final VideoModel video;
  final int index;
  final VoidCallback onTap;

  const _VideoTile({
    required this.video,
    required this.index,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: onTap,
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
        child: Row(
          children: [
            // Thumbnail or index
            ClipRRect(
              borderRadius: BorderRadius.circular(8),
              child: Stack(
                children: [
                  video.thumbnailUrl != null
                      ? CachedNetworkImage(
                          imageUrl: video.thumbnailUrl!,
                          width: 90,
                          height: 60,
                          fit: BoxFit.cover,
                          placeholder: (_, __) =>
                              Container(width: 90, height: 60, color: kMuted),
                          errorWidget: (_, __, ___) =>
                              Container(width: 90, height: 60, color: kMuted,
                                  child: const Icon(Icons.videocam_outlined,
                                      color: kMutedForeground)),
                        )
                      : Container(
                          width: 90,
                          height: 60,
                          color: kMuted,
                          child: const Icon(Icons.videocam_outlined,
                              color: kMutedForeground),
                        ),
                  // Lock overlay for inaccessible videos
                  if (!video.accessible)
                    Container(
                      width: 90,
                      height: 60,
                      color: Colors.black.withValues(alpha: 0.5),
                      child: const Icon(Icons.lock_rounded,
                          color: Colors.white, size: 20),
                    ),
                ],
              ),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    video.title,
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                    style: TextStyle(
                      fontSize: 13,
                      fontWeight: FontWeight.w600,
                      color: video.accessible ? kForeground : kMutedForeground,
                      height: 1.3,
                    ),
                  ),
                  const SizedBox(height: 4),
                  Row(
                    children: [
                      Container(
                        padding: const EdgeInsets.symmetric(
                            horizontal: 6, vertical: 2),
                        decoration: BoxDecoration(
                          color: kGold.withValues(alpha: 0.12),
                          borderRadius: BorderRadius.circular(4),
                        ),
                        child: Text(
                          video.categoryLabel,
                          style: const TextStyle(
                              fontSize: 10,
                              fontWeight: FontWeight.w600,
                              color: kGold),
                        ),
                      ),
                      if (video.durationSeconds > 0) ...[
                        const SizedBox(width: 8),
                        Text(
                          video.formattedDuration,
                          style: const TextStyle(
                              fontSize: 11, color: kMutedForeground),
                        ),
                      ],
                    ],
                  ),
                ],
              ),
            ),
            FavoriteButton(video: video),
            if (video.accessible) DownloadButton(video: video),
            const SizedBox(width: 4),
            Icon(
              video.accessible
                  ? Icons.play_circle_outline_rounded
                  : Icons.lock_outline_rounded,
              color: video.accessible ? kGold : kMutedForeground,
              size: 22,
            ),
          ],
        ),
      ),
    );
  }
}
