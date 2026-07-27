import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import '../../../core/theme/app_colors.dart';
import '../models/reply_model.dart';

class ReplyTile extends StatelessWidget {
  final ReplyModel reply;

  const ReplyTile({super.key, required this.reply});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 10, horizontal: 16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              const Icon(Icons.person, size: 13, color: kPrimary),
              const SizedBox(width: 4),
              Text(
                reply.anonName,
                style: const TextStyle(
                  fontSize: 12,
                  color: kPrimary,
                  fontWeight: FontWeight.w500,
                ),
              ),
              const Spacer(),
              Text(
                _timeAgo(reply.createdAt),
                style: const TextStyle(fontSize: 11, color: kMutedForeground),
              ),
            ],
          ),
          const SizedBox(height: 6),
          Text(
            reply.content,
            style: const TextStyle(fontSize: 14, color: kForeground, height: 1.4),
          ),
          if (reply.imageUrl != null) ...[
            const SizedBox(height: 8),
            GestureDetector(
              onTap: () => _showFullscreen(context, reply.imageUrl!),
              child: ClipRRect(
                borderRadius: BorderRadius.circular(8),
                child: CachedNetworkImage(
                  imageUrl: reply.imageUrl!,
                  width: double.infinity,
                  fit: BoxFit.cover,
                  placeholder: (_, __) => Container(height: 120, color: kMuted),
                  errorWidget: (_, __, ___) => Container(height: 120, color: kMuted),
                ),
              ),
            ),
          ],
          const SizedBox(height: 10),
          const Divider(color: kBorder, height: 1),
        ],
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

  void _showFullscreen(BuildContext context, String url) {
    Navigator.of(context).push(MaterialPageRoute(
      fullscreenDialog: true,
      builder: (_) => Scaffold(
        backgroundColor: Colors.black,
        appBar: AppBar(
          backgroundColor: Colors.black,
          foregroundColor: Colors.white,
          elevation: 0,
        ),
        body: Center(
          child: InteractiveViewer(
            child: CachedNetworkImage(imageUrl: url),
          ),
        ),
      ),
    ));
  }
}
