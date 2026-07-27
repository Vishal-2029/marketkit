import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import '../../../core/theme/app_colors.dart';
import '../models/product_model.dart';
import 'product_favorite_button.dart';

class ProductCard extends StatelessWidget {
  final ProductModel product;
  final VoidCallback onTap;

  const ProductCard({super.key, required this.product, required this.onTap});

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        decoration: BoxDecoration(
          color: kCard,
          borderRadius: BorderRadius.circular(14),
          border: Border.all(color: kBorder),
        ),
        clipBehavior: Clip.antiAlias,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Expanded(
              child: Stack(
                fit: StackFit.expand,
                children: [
                  product.previewUrls.isNotEmpty
                      ? CachedNetworkImage(
                          imageUrl: product.previewUrls.first,
                          fit: BoxFit.cover,
                          placeholder: (_, __) =>
                              Container(color: kMuted),
                          errorWidget: (_, __, ___) => Container(
                            color: kMuted,
                            child: const Icon(Icons.image_not_supported_outlined,
                                color: kMutedForeground),
                          ),
                        )
                      : Container(
                          color: kMuted,
                          child: const Icon(Icons.design_services_outlined,
                              color: kMutedForeground, size: 32),
                        ),
                  Positioned(
                    top: 6,
                    left: 6,
                    child: ProductFavoriteButton(product: product),
                  ),
                  if (product.isPurchased)
                    Positioned(
                      top: 6,
                      right: 6,
                      child: GestureDetector(
                        onTap: () => context.push(
                          '/market/product/${product.id}/messages',
                          extra: product.title,
                        ),
                        child: Container(
                          padding: const EdgeInsets.all(4),
                          decoration: const BoxDecoration(
                            color: Colors.black38,
                            shape: BoxShape.circle,
                          ),
                          child: const Icon(Icons.help_outline_rounded,
                              size: 16, color: Colors.white),
                        ),
                      ),
                    ),
                  if (product.isPurchased || product.isMine)
                    Positioned(
                      top: product.isPurchased ? 34 : 6,
                      right: 6,
                      child: Container(
                        padding: const EdgeInsets.symmetric(
                            horizontal: 8, vertical: 3),
                        decoration: BoxDecoration(
                          color: product.isMine ? kPrimary : kSuccess,
                          borderRadius: BorderRadius.circular(20),
                        ),
                        child: Text(
                          product.isMine ? 'Yours' : 'Owned',
                          style: const TextStyle(
                              fontSize: 10,
                              color: Colors.white,
                              fontWeight: FontWeight.w600),
                        ),
                      ),
                    ),
                ],
              ),
            ),
            Padding(
              padding: const EdgeInsets.all(10),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    product.title,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(
                        fontSize: 13,
                        fontWeight: FontWeight.w600,
                        color: kForeground),
                  ),
                  if (product.sellerName.isNotEmpty) ...[
                    const SizedBox(height: 2),
                    Row(
                      children: [
                        Flexible(
                          child: Text(
                            product.sellerName,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: const TextStyle(
                                fontSize: 11, color: kMutedForeground),
                          ),
                        ),
                        if (product.featuredSeller) ...[
                          const SizedBox(width: 4),
                          const Icon(Icons.workspace_premium,
                              size: 12, color: kPrimary),
                        ],
                      ],
                    ),
                  ],
                  const SizedBox(height: 4),
                  Row(
                    children: [
                      Text(
                        product.formattedPrice,
                        style: const TextStyle(
                            fontSize: 14,
                            fontWeight: FontWeight.w700,
                            color: kPrimary),
                      ),
                      const Spacer(),
                      Text(
                        product.fileFormat.toUpperCase(),
                        style: const TextStyle(
                            fontSize: 10, color: kMutedForeground),
                      ),
                    ],
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
