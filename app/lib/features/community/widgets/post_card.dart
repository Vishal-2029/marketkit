import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import '../../../core/theme/app_colors.dart';
import '../models/post_model.dart';

class PostCard extends StatelessWidget {
  final PostModel post;
  final VoidCallback onTap;

  const PostCard({super.key, required this.post, required this.onTap});

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        margin: const EdgeInsets.only(bottom: 12),
        padding: const EdgeInsets.all(14),
        decoration: BoxDecoration(
          color: kCard,
          borderRadius: BorderRadius.circular(12),
          border: Border.all(color: kBorder),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                _CategoryBadge(post.category),
                const Spacer(),
                Text(
                  _timeAgo(post.createdAt),
                  style: const TextStyle(fontSize: 11, color: kMutedForeground),
                ),
              ],
            ),
            const SizedBox(height: 8),
            Text(
              post.title,
              style: const TextStyle(
                fontSize: 14,
                fontWeight: FontWeight.w600,
                color: kForeground,
              ),
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
            ),
            const SizedBox(height: 4),
            Text(
              post.content,
              style: const TextStyle(fontSize: 13, color: kMutedForeground),
              maxLines: 3,
              overflow: TextOverflow.ellipsis,
            ),
            if (post.imageUrls.isNotEmpty) ...[
              const SizedBox(height: 10),
              ClipRRect(
                borderRadius: BorderRadius.circular(8),
                child: CachedNetworkImage(
                  imageUrl: post.imageUrls.first,
                  height: 140,
                  width: double.infinity,
                  fit: BoxFit.cover,
                  placeholder: (_, __) => Container(height: 140, color: kMuted),
                  errorWidget: (_, __, ___) => Container(height: 140, color: kMuted),
                ),
              ),
            ],
            const SizedBox(height: 10),
            Row(
              children: [
                const Icon(Icons.person_outline, size: 13, color: kGold),
                const SizedBox(width: 4),
                Text(post.anonName,
                    style: const TextStyle(fontSize: 12, color: kGold, fontWeight: FontWeight.w500)),
                const Spacer(),
                if (post.imageUrls.isNotEmpty) ...[
                  const Icon(Icons.photo_camera_outlined, size: 13, color: kMutedForeground),
                  const SizedBox(width: 3),
                  Text('${post.imageUrls.length}',
                      style: const TextStyle(fontSize: 12, color: kMutedForeground)),
                  const SizedBox(width: 10),
                ],
                const Icon(Icons.chat_bubble_outline, size: 13, color: kMutedForeground),
                const SizedBox(width: 4),
                Text('${post.replyCount}',
                    style: const TextStyle(fontSize: 12, color: kMutedForeground)),
              ],
            ),
          ],
        ),
      ),
    );
  }

  String _timeAgo(DateTime dt) {
    final diff = DateTime.now().difference(dt);
    if (diff.inMinutes < 1) return 'just now';
    if (diff.inHours < 1) return '${diff.inMinutes}m ago';
    if (diff.inDays < 1) return '${diff.inHours}h ago';
    if (diff.inDays < 30) return '${diff.inDays}d ago';
    return '${(diff.inDays / 30).floor()}mo ago';
  }
}

class _CategoryBadge extends StatelessWidget {
  final String category;
  const _CategoryBadge(this.category);

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
      decoration: BoxDecoration(
        color: kGold.withOpacity(0.12),
        borderRadius: BorderRadius.circular(50),
      ),
      child: Text(
        category,
        style: const TextStyle(
          fontSize: 10,
          fontWeight: FontWeight.w600,
          color: kGold,
          letterSpacing: 0.3,
        ),
      ),
    );
  }
}
