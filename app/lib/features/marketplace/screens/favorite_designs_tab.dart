import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../core/theme/app_colors.dart';
import '../providers/design_favorites_provider.dart';
import '../widgets/design_card.dart';

/// Grid of the buyer's starred designs — device-only, mirrors the video
/// FavoritesScreen but embedded as a Design Market tab instead of a pushed
/// route, since the wireframe puts it alongside Design/Upload/Profile.
class FavoriteDesignsTab extends ConsumerWidget {
  const FavoriteDesignsTab({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final entries = ref.watch(designFavoritesProvider).entries;

    if (entries.isEmpty) {
      return Center(
        child: Padding(
          padding: const EdgeInsets.all(32),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: const [
              Icon(Icons.star_outline_rounded,
                  size: 56, color: kMutedForeground),
              SizedBox(height: 12),
              Text('No favorite designs yet',
                  style: TextStyle(
                      fontSize: 16,
                      fontWeight: FontWeight.w600,
                      color: kForeground)),
              SizedBox(height: 6),
              Text('Tap the star icon on a design to save it here.',
                  textAlign: TextAlign.center,
                  style: TextStyle(fontSize: 13, color: kMutedForeground)),
            ],
          ),
        ),
      );
    }

    return GridView.builder(
      padding: const EdgeInsets.fromLTRB(16, 12, 16, 100),
      gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
        crossAxisCount: 2,
        mainAxisSpacing: 12,
        crossAxisSpacing: 12,
        childAspectRatio: 0.8,
      ),
      itemCount: entries.length,
      itemBuilder: (_, i) {
        final design = entries[i].design;
        return DesignCard(
          design: design,
          onTap: () => context.push('/market/design/${design.id}'),
        );
      },
    );
  }
}
