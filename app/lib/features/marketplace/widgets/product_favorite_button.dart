import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/theme/app_colors.dart';
import '../models/product_model.dart';
import '../providers/product_favorites_provider.dart';

/// Star toggle for liking a product, mirroring the video [FavoriteButton].
class ProductFavoriteButton extends ConsumerWidget {
  final ProductModel product;

  const ProductFavoriteButton({super.key, required this.product});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final isFavorite = ref.watch(
        productFavoritesProvider.select((s) => s.isFavorite(product.id)));

    return GestureDetector(
      onTap: () => ref.read(productFavoritesProvider.notifier).toggle(product),
      child: Container(
        padding: const EdgeInsets.all(4),
        decoration: const BoxDecoration(
          color: Colors.black38,
          shape: BoxShape.circle,
        ),
        child: Icon(
          isFavorite ? Icons.star_rounded : Icons.star_outline_rounded,
          size: 16,
          color: isFavorite ? kPrimary : Colors.white,
        ),
      ),
    );
  }
}
