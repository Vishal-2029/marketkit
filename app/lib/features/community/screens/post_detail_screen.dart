import 'dart:async';
import 'dart:io';
import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/foundation.dart' show kIsWeb;
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:image_picker/image_picker.dart';
import 'package:shimmer/shimmer.dart';
import '../../../core/theme/app_colors.dart';
import '../models/post_model.dart';
import '../providers/post_detail_provider.dart';
import '../widgets/reply_tile.dart';

class PostDetailScreen extends ConsumerStatefulWidget {
  final String postId;
  const PostDetailScreen({super.key, required this.postId});

  @override
  ConsumerState<PostDetailScreen> createState() => _PostDetailScreenState();
}

class _PostDetailScreenState extends ConsumerState<PostDetailScreen> {
  final _replyController = TextEditingController();
  XFile? _replyImage;
  Timer? _pollTimer;

  @override
  void initState() {
    super.initState();
    Future.microtask(
      () => ref.read(postDetailProvider(widget.postId).notifier).load(widget.postId),
    );
    _pollTimer = Timer.periodic(const Duration(seconds: 20), (_) {
      if (mounted) {
        ref.read(postDetailProvider(widget.postId).notifier).load(widget.postId);
      }
    });
  }

  @override
  void dispose() {
    _pollTimer?.cancel();
    _replyController.dispose();
    super.dispose();
  }

  Future<void> _pickReplyImage() async {
    final picked = await ImagePicker().pickImage(
      source: ImageSource.gallery,
      imageQuality: 75,
      maxWidth: 1280,
    );
    if (picked != null && mounted) {
      setState(() => _replyImage = picked);
    }
  }

  Future<void> _sendReply() async {
    final text = _replyController.text.trim();
    if (text.isEmpty && _replyImage == null) return;
    final image = _replyImage;
    try {
      await ref
          .read(postDetailProvider(widget.postId).notifier)
          .addReply(widget.postId, text, image: image);
      _replyController.clear();
      if (mounted) setState(() => _replyImage = null);
    } on DioException catch (e) {
      if (!mounted) return;
      final statusCode = e.response?.statusCode;
      final msg = e.response?.data?['error'] as String? ?? 'Failed to post reply';
      if (statusCode == 400) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text(msg)));
      } else if (statusCode == 429) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text("You're replying too fast — wait a moment")),
        );
      } else {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text(msg)));
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(postDetailProvider(widget.postId));

    return Scaffold(
      backgroundColor: kBackground,
      appBar: AppBar(
        backgroundColor: kBackground,
        foregroundColor: kForeground,
        elevation: 0,
        title: const Text(
          'Question',
          style: TextStyle(fontSize: 16, fontWeight: FontWeight.w600),
        ),
      ),
      body: Column(
        children: [
          Expanded(
            child: state.isLoading
                ? _shimmer()
                : state.post == null
                    ? const Center(child: Text('Post not found', style: TextStyle(color: kMutedForeground)))
                    : CustomScrollView(
                        slivers: [
                          SliverToBoxAdapter(
                            child: _PostHeader(post: state.post!),
                          ),
                          SliverToBoxAdapter(
                            child: Padding(
                              padding: const EdgeInsets.fromLTRB(16, 16, 16, 8),
                              child: Text(
                                'Replies (${state.replies.length})',
                                style: const TextStyle(
                                  fontSize: 14,
                                  fontWeight: FontWeight.w600,
                                  color: kForeground,
                                ),
                              ),
                            ),
                          ),
                          if (state.replies.isEmpty)
                            const SliverToBoxAdapter(
                              child: Padding(
                                padding: EdgeInsets.all(24),
                                child: Center(
                                  child: Text(
                                    'No replies yet. Be the first to help!',
                                    style: TextStyle(color: kMutedForeground),
                                  ),
                                ),
                              ),
                            )
                          else
                            SliverList(
                              delegate: SliverChildBuilderDelegate(
                                (_, i) => ReplyTile(reply: state.replies[i]),
                                childCount: state.replies.length,
                              ),
                            ),
                          const SliverToBoxAdapter(child: SizedBox(height: 16)),
                        ],
                      ),
          ),
          // Reply input bar
          _ReplyBar(
            controller: _replyController,
            isSubmitting: state.isSubmitting,
            replyImage: _replyImage,
            onSend: _sendReply,
            onPickImage: _pickReplyImage,
            onRemoveImage: () => setState(() => _replyImage = null),
          ),
        ],
      ),
    );
  }

  Widget _shimmer() {
    return Shimmer.fromColors(
      baseColor: kMuted,
      highlightColor: kCard,
      child: Column(
        children: [
          Container(
              height: 160,
              margin: const EdgeInsets.all(16),
              decoration: BoxDecoration(
                  color: kMuted, borderRadius: BorderRadius.circular(12))),
          for (int i = 0; i < 3; i++)
            Container(
                height: 60,
                margin: const EdgeInsets.fromLTRB(16, 0, 16, 10),
                decoration: BoxDecoration(
                    color: kMuted, borderRadius: BorderRadius.circular(8))),
        ],
      ),
    );
  }
}

class _PostHeader extends StatelessWidget {
  final PostModel post;
  const _PostHeader({required this.post});

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: const EdgeInsets.all(16),
      padding: const EdgeInsets.all(16),
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
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
                decoration: BoxDecoration(
                  color: kPrimary.withOpacity(0.12),
                  borderRadius: BorderRadius.circular(50),
                ),
                child: Text(
                  post.category,
                  style: const TextStyle(
                    fontSize: 10,
                    fontWeight: FontWeight.w600,
                    color: kPrimary,
                  ),
                ),
              ),
              const Spacer(),
              const Icon(Icons.person_outline, size: 13, color: kPrimary),
              const SizedBox(width: 4),
              Text(post.anonName,
                  style: const TextStyle(fontSize: 12, color: kPrimary, fontWeight: FontWeight.w500)),
            ],
          ),
          const SizedBox(height: 10),
          Text(
            post.title,
            style: const TextStyle(
              fontSize: 16,
              fontWeight: FontWeight.w700,
              color: kForeground,
            ),
          ),
          const SizedBox(height: 8),
          Text(
            post.content,
            style: const TextStyle(fontSize: 14, color: kForeground, height: 1.5),
          ),
          if (post.imageUrls.isNotEmpty) ...[
            const SizedBox(height: 12),
            SizedBox(
              height: 160,
              child: ListView.separated(
                scrollDirection: Axis.horizontal,
                itemCount: post.imageUrls.length,
                separatorBuilder: (_, __) => const SizedBox(width: 8),
                itemBuilder: (context, i) => GestureDetector(
                  onTap: () => _showFullscreen(context, post.imageUrls, i),
                  child: ClipRRect(
                    borderRadius: BorderRadius.circular(10),
                    child: CachedNetworkImage(
                      imageUrl: post.imageUrls[i],
                      width: 180,
                      height: 160,
                      fit: BoxFit.cover,
                      placeholder: (_, __) =>
                          Container(width: 180, height: 160, color: kMuted),
                      errorWidget: (_, __, ___) =>
                          Container(width: 180, height: 160, color: kMuted),
                    ),
                  ),
                ),
              ),
            ),
          ],
        ],
      ),
    );
  }

  static void _showFullscreen(
      BuildContext context, List<String> urls, int initial) {
    Navigator.of(context).push(MaterialPageRoute(
      fullscreenDialog: true,
      builder: (_) => _FullscreenImageViewer(urls: urls, initialIndex: initial),
    ));
  }
}

class _FullscreenImageViewer extends StatefulWidget {
  final List<String> urls;
  final int initialIndex;
  const _FullscreenImageViewer(
      {required this.urls, required this.initialIndex});

  @override
  State<_FullscreenImageViewer> createState() => _FullscreenImageViewerState();
}

class _FullscreenImageViewerState extends State<_FullscreenImageViewer> {
  late final PageController _pageCtrl;
  late int _current;

  @override
  void initState() {
    super.initState();
    _current = widget.initialIndex;
    _pageCtrl = PageController(initialPage: widget.initialIndex);
  }

  @override
  void dispose() {
    _pageCtrl.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.black,
      appBar: AppBar(
        backgroundColor: Colors.black,
        foregroundColor: Colors.white,
        elevation: 0,
        title: widget.urls.length > 1
            ? Text('${_current + 1} / ${widget.urls.length}',
                style: const TextStyle(color: Colors.white70, fontSize: 14))
            : null,
      ),
      body: PageView.builder(
        controller: _pageCtrl,
        itemCount: widget.urls.length,
        onPageChanged: (i) => setState(() => _current = i),
        itemBuilder: (_, i) => InteractiveViewer(
          child: Center(
            child: CachedNetworkImage(imageUrl: widget.urls[i]),
          ),
        ),
      ),
    );
  }
}

class _ReplyBar extends StatelessWidget {
  final TextEditingController controller;
  final bool isSubmitting;
  final XFile? replyImage;
  final VoidCallback onSend;
  final VoidCallback onPickImage;
  final VoidCallback onRemoveImage;

  const _ReplyBar({
    required this.controller,
    required this.isSubmitting,
    required this.replyImage,
    required this.onSend,
    required this.onPickImage,
    required this.onRemoveImage,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: EdgeInsets.only(
        left: 12,
        right: 8,
        top: 8,
        bottom: MediaQuery.of(context).viewInsets.bottom + 8,
      ),
      decoration: const BoxDecoration(
        color: kCard,
        border: Border(top: BorderSide(color: kBorder)),
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          if (replyImage != null) ...[
            Stack(
              children: [
                ClipRRect(
                  borderRadius: BorderRadius.circular(8),
                  child: kIsWeb
                      ? Image.network(replyImage!.path,
                          height: 64, width: 64, fit: BoxFit.cover)
                      : Image.file(File(replyImage!.path),
                          height: 64, width: 64, fit: BoxFit.cover),
                ),
                Positioned(
                  top: 2,
                  right: 2,
                  child: GestureDetector(
                    onTap: onRemoveImage,
                    child: Container(
                      width: 18,
                      height: 18,
                      decoration: const BoxDecoration(
                        color: Colors.black54,
                        shape: BoxShape.circle,
                      ),
                      child: const Icon(Icons.close, size: 12, color: Colors.white),
                    ),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 6),
          ],
          Row(
            children: [
              IconButton(
                icon: const Icon(Icons.add_photo_alternate_outlined,
                    color: kMutedForeground, size: 22),
                onPressed: replyImage == null ? onPickImage : null,
                padding: EdgeInsets.zero,
                constraints: const BoxConstraints(minWidth: 36, minHeight: 36),
              ),
              const SizedBox(width: 4),
              Expanded(
                child: TextField(
                  controller: controller,
                  minLines: 1,
                  maxLines: 3,
                  textCapitalization: TextCapitalization.sentences,
                  decoration: InputDecoration(
                    hintText: 'Write a reply...',
                    hintStyle: const TextStyle(color: kMutedForeground),
                    filled: true,
                    fillColor: kMuted,
                    contentPadding:
                        const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
                    border: OutlineInputBorder(
                      borderRadius: BorderRadius.circular(24),
                      borderSide: BorderSide.none,
                    ),
                  ),
                ),
              ),
              const SizedBox(width: 8),
              isSubmitting
                  ? const SizedBox(
                      width: 40,
                      height: 40,
                      child: CircularProgressIndicator(color: kPrimary, strokeWidth: 2.5),
                    )
                  : IconButton(
                      icon: const Icon(Icons.send_rounded, color: kPrimary),
                      onPressed: onSend,
                    ),
            ],
          ),
        ],
      ),
    );
  }
}

