import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/theme/app_colors.dart';
import '../../home/models/video_model.dart';
import '../../home/services/videos_service.dart';
import '../models/download_quality.dart';
import '../providers/downloads_provider.dart';

/// Download action button: shows a quality picker on first tap, then
/// progress / "Downloaded (tap to remove)" states.
class DownloadButton extends ConsumerWidget {
  final VideoModel video;

  /// When true the icon renders white (for use on dark surfaces like the video
  /// player). When false it renders kGold / kMutedForeground (for light cards).
  final bool darkBackground;

  const DownloadButton({
    super.key,
    required this.video,
    this.darkBackground = false,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final state = ref.watch(downloadsProvider);
    final notifier = ref.read(downloadsProvider.notifier);

    if (state.isDownloading(video.id)) {
      final p = state.progressFor(video.id);
      final pct = (p * 100).clamp(0, 100).round();
      return IconButton(
        tooltip: p > 0 ? 'Downloading $pct% — tap to cancel' : 'Cancel download',
        onPressed: () => notifier.cancel(video.id),
        icon: SizedBox(
          width: 28,
          height: 28,
          child: Stack(
            alignment: Alignment.center,
            children: [
              CircularProgressIndicator(
                value: p > 0 ? p : null,
                strokeWidth: 2,
                color: kGold,
                backgroundColor: darkBackground ? Colors.white24 : Colors.black12,
              ),
              if (p > 0)
                Text(
                  '$pct',
                  style: TextStyle(
                    fontSize: 8,
                    fontWeight: FontWeight.w700,
                    color: darkBackground ? Colors.white : kForeground,
                  ),
                )
              else
                Icon(Icons.close, size: 10,
                    color: darkBackground ? Colors.white : kForeground),
            ],
          ),
        ),
      );
    }

    if (state.isDownloaded(video.id)) {
      return IconButton(
        tooltip: 'Downloaded — tap to remove',
        icon: const Icon(Icons.download_done_rounded, color: kGold),
        onPressed: () => _confirmRemove(context, notifier),
      );
    }

    return IconButton(
      tooltip: 'Download for offline',
      icon: Icon(
        Icons.download_outlined,
        color: darkBackground ? Colors.white : kMutedForeground,
      ),
      onPressed: () => _showQualityPicker(context, notifier),
    );
  }

  static const _fallbackQualities = ['360p', '720p', '1080p'];

  String _formatSize(int sizeBytes) {
    final mb = sizeBytes / (1024 * 1024);
    return '${mb.toStringAsFixed(0)} MB';
  }

  Future<void> _showQualityPicker(
      BuildContext context, DownloadsNotifier notifier) async {
    final selected = await showModalBottomSheet<String>(
      context: context,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(16)),
      ),
      builder: (ctx) => SafeArea(
        child: Padding(
          padding: const EdgeInsets.fromLTRB(16, 12, 16, 8),
          child: FutureBuilder<List<DownloadQuality>>(
            future: VideosService().fetchDownloadQualities(video.id),
            builder: (context, snapshot) {
              final isLoading = snapshot.connectionState == ConnectionState.waiting;
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
                // Real sizes unavailable — fall back to the plain quality
                // list so downloading still works.
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
                  const SizedBox(height: 8),
                  ...optionTiles,
                ],
              );
            },
          ),
        ),
      ),
    );

    if (selected == null) return;
    try {
      await notifier.startDownload(video, quality: selected);
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Saving $selected copy for offline viewing')),
        );
      }
    } catch (_) {
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Download failed. Please try again.')),
        );
      }
    }
  }

  Future<void> _confirmRemove(
      BuildContext context, DownloadsNotifier notifier) async {
    final ok = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Remove download?'),
        content: const Text(
            'This removes the offline copy from this device. You can download it again later.'),
        actions: [
          TextButton(
              onPressed: () => Navigator.pop(ctx, false),
              child: const Text('Cancel')),
          TextButton(
              onPressed: () => Navigator.pop(ctx, true),
              child: const Text('Remove')),
        ],
      ),
    );
    if (ok == true) await notifier.remove(video.id);
  }
}
