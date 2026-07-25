import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/theme/app_colors.dart';
import '../models/design_model.dart';
import '../providers/design_favorites_provider.dart';

/// Star toggle for liking a design, mirroring the video [FavoriteButton].
class DesignFavoriteButton extends ConsumerWidget {
  final DesignModel design;

  const DesignFavoriteButton({super.key, required this.design});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final isFavorite = ref.watch(
        designFavoritesProvider.select((s) => s.isFavorite(design.id)));

    return GestureDetector(
      onTap: () => ref.read(designFavoritesProvider.notifier).toggle(design),
      child: Container(
        padding: const EdgeInsets.all(4),
        decoration: const BoxDecoration(
          color: Colors.black38,
          shape: BoxShape.circle,
        ),
        child: Icon(
          isFavorite ? Icons.star_rounded : Icons.star_outline_rounded,
          size: 16,
          color: isFavorite ? kGold : Colors.white,
        ),
      ),
    );
  }
}
