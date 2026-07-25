import 'dart:convert';
import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import '../../../core/theme/app_colors.dart';
import '../models/video_model.dart';

class VideoCard extends StatelessWidget {
  final VideoModel video;
  final VoidCallback onTap;

  const VideoCard({super.key, required this.video, required this.onTap});

  Widget _thumbnailPlaceholder() => Container(
        decoration: BoxDecoration(
          gradient: LinearGradient(
            begin: Alignment.topLeft,
            end: Alignment.bottomRight,
            colors: [kGold.withOpacity(0.3), kCream.withOpacity(0.5)],
          ),
        ),
        child: const Icon(Icons.play_circle_outline_rounded, color: Colors.white70, size: 36),
      );

  Widget _lqipPlaceholder(String lqip) {
    try {
      final b64 = lqip.contains(',') ? lqip.split(',').last : lqip;
      return Image(
        image: MemoryImage(base64Decode(b64)),
        fit: BoxFit.cover,
        width: double.infinity,
        height: double.infinity,
      );
    } catch (_) {
      return _thumbnailPlaceholder();
    }
  }

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
            // Thumbnail area
            AspectRatio(
              aspectRatio: 16 / 9,
              child: Stack(
                fit: StackFit.expand,
                children: [
                  if (video.thumbnailUrl?.isNotEmpty ?? false)
                    CachedNetworkImage(
                      imageUrl: video.thumbnailUrl!,
                      fit: BoxFit.cover,
                      width: double.infinity,
                      height: double.infinity,
                      fadeInDuration: const Duration(milliseconds: 300),
                      placeholder: (_, __) => video.lqip != null
                          ? _lqipPlaceholder(video.lqip!)
                          : _thumbnailPlaceholder(),
                      errorWidget: (_, __, ___) => _thumbnailPlaceholder(),
                    )
                  else
                    _thumbnailPlaceholder(),

                  // Category badge (top-left)
                  Positioned(
                    top: 8,
                    left: 8,
                    child: Container(
                      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                      decoration: BoxDecoration(
                        color: kGold.withOpacity(0.9),
                        borderRadius: BorderRadius.circular(6),
                      ),
                      child: Text(
                        video.categoryLabel,
                        style: const TextStyle(
                          color: Colors.white,
                          fontSize: 10,
                          fontWeight: FontWeight.w600,
                        ),
                      ),
                    ),
                  ),

                  // Duration badge (bottom-right)
                  if (video.formattedDuration.isNotEmpty)
                    Positioned(
                      bottom: 8,
                      right: 8,
                      child: Container(
                        padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                        decoration: BoxDecoration(
                          color: Colors.black54,
                          borderRadius: BorderRadius.circular(4),
                        ),
                        child: Text(
                          video.formattedDuration,
                          style: const TextStyle(
                            color: Colors.white,
                            fontSize: 10,
                            fontWeight: FontWeight.w500,
                          ),
                        ),
                      ),
                    ),

                  // Lock overlay for inaccessible videos
                  if (!video.accessible)
                    Container(
                      color: Colors.black38,
                      child: const Center(
                        child: Icon(Icons.lock_rounded, color: Colors.white70, size: 28),
                      ),
                    ),
                ],
              ),
            ),

            // Title & meta
            Padding(
              padding: const EdgeInsets.all(10),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    video.title,
                    maxLines: 2,
                    overflow: TextOverflow.ellipsis,
                    style: const TextStyle(
                      fontSize: 13,
                      fontWeight: FontWeight.w600,
                      color: kForeground,
                      height: 1.3,
                    ),
                  ),
                  if (!video.accessible) ...[
                    const SizedBox(height: 4),
                    const Text(
                      'Subscription required',
                      style: TextStyle(fontSize: 11, color: kMutedForeground),
                    ),
                  ],
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}
