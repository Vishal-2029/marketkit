import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../core/theme/app_colors.dart';
import '../providers/product_favorites_provider.dart';
import '../widgets/product_card.dart';

/// Grid of the buyer's starred products — device-only, mirrors the video
/// FavoritesScreen but embedded as a Product Market tab instead of a pushed
/// route, since the wireframe puts it alongside Product/Upload/Profile.
class FavoriteProductsTab extends ConsumerWidget {
  const FavoriteProductsTab({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final entries = ref.watch(productFavoritesProvider).entries;

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
              Text('No favorite products yet',
                  style: TextStyle(
                      fontSize: 16,
                      fontWeight: FontWeight.w600,
                      color: kForeground)),
              SizedBox(height: 6),
              Text('Tap the star icon on a product to save it here.',
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
        final product = entries[i].product;
        return ProductCard(
          product: product,
          onTap: () => context.push('/market/product/${product.id}'),
        );
      },
    );
  }
}
