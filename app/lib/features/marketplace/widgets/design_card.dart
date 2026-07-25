import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import '../../../core/theme/app_colors.dart';
import '../models/design_model.dart';
import 'design_favorite_button.dart';

class DesignCard extends StatelessWidget {
  final DesignModel design;
  final VoidCallback onTap;

  const DesignCard({super.key, required this.design, required this.onTap});

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
                  design.previewUrls.isNotEmpty
                      ? CachedNetworkImage(
                          imageUrl: design.previewUrls.first,
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
                    child: DesignFavoriteButton(design: design),
                  ),
                  if (design.isPurchased)
                    Positioned(
                      top: 6,
                      right: 6,
                      child: GestureDetector(
                        onTap: () => context.push(
                          '/market/design/${design.id}/messages',
                          extra: design.title,
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
                  if (design.isPurchased || design.isMine)
                    Positioned(
                      top: design.isPurchased ? 34 : 6,
                      right: 6,
                      child: Container(
                        padding: const EdgeInsets.symmetric(
                            horizontal: 8, vertical: 3),
                        decoration: BoxDecoration(
                          color: design.isMine ? kGold : kSage,
                          borderRadius: BorderRadius.circular(20),
                        ),
                        child: Text(
                          design.isMine ? 'Yours' : 'Owned',
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
                    design.title,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(
                        fontSize: 13,
                        fontWeight: FontWeight.w600,
                        color: kForeground),
                  ),
                  if (design.sellerName.isNotEmpty) ...[
                    const SizedBox(height: 2),
                    Row(
                      children: [
                        Flexible(
                          child: Text(
                            design.sellerName,
                            maxLines: 1,
                            overflow: TextOverflow.ellipsis,
                            style: const TextStyle(
                                fontSize: 11, color: kMutedForeground),
                          ),
                        ),
                        if (design.featuredSeller) ...[
                          const SizedBox(width: 4),
                          const Icon(Icons.workspace_premium,
                              size: 12, color: kGold),
                        ],
                      ],
                    ),
                  ],
                  const SizedBox(height: 4),
                  Row(
                    children: [
                      Text(
                        design.formattedPrice,
                        style: const TextStyle(
                            fontSize: 14,
                            fontWeight: FontWeight.w700,
                            color: kGold),
                      ),
                      const Spacer(),
                      Text(
                        design.fileFormat.toUpperCase(),
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
