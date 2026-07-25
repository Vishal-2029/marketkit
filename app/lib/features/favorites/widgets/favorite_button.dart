import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/theme/app_colors.dart';
import '../../home/models/video_model.dart';
import '../providers/favorites_provider.dart';

/// Star toggle for bookmarking a video, mirroring [DownloadButton]'s placement
/// and sizing so the two sit naturally side by side.
class FavoriteButton extends ConsumerWidget {
  final VideoModel video;

  /// When true the unstarred icon renders white (for dark surfaces like the
  /// video player); when false it renders kMutedForeground (for light cards).
  final bool darkBackground;

  const FavoriteButton({
    super.key,
    required this.video,
    this.darkBackground = false,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final isFavorite = ref.watch(
        favoritesProvider.select((s) => s.isFavorite(video.id)));

    return IconButton(
      tooltip: isFavorite ? 'Remove from favorites' : 'Add to favorites',
      icon: Icon(
        isFavorite ? Icons.star_rounded : Icons.star_outline_rounded,
        color: isFavorite
            ? kGold
            : (darkBackground ? Colors.white : kMutedForeground),
      ),
      onPressed: () => ref.read(favoritesProvider.notifier).toggle(video),
    );
  }
}
