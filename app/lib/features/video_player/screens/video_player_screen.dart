import 'dart:async';

import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/foundation.dart' show kIsWeb;
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../core/theme/app_colors.dart';
import '../../../shared/widgets/expandable_text.dart';
import '../../downloads/services/downloads_service.dart';
import '../../downloads/services/local_video_proxy.dart';
import '../../downloads/widgets/download_button.dart';
import '../../favorites/widgets/favorite_button.dart';
import '../../home/models/video_model.dart';
import '../../home/services/videos_service.dart';
import '../../library/widgets/library_video_tile.dart';
import '../../photos/models/photo_model.dart';
import '../../photos/services/photos_service.dart';
import '../../photos/widgets/photo_gallery.dart';
import '../models/video_comment_model.dart';
import '../models/video_reaction_model.dart';
import '../services/engagement_service.dart';
import '../services/playback_service.dart';
import '../widgets/netflix_video_player.dart';

class VideoPlayerScreen extends ConsumerStatefulWidget {
  final VideoModel video;

  const VideoPlayerScreen({super.key, required this.video});

  @override
  ConsumerState<VideoPlayerScreen> createState() => _VideoPlayerScreenState();
}

// Maps UI quality label to the HLS variant directory name.
const _qualityVariant = {
  '1080p': 'v0',
  '720p': 'v1',
  '480p': 'v2',
  '360p': 'v3',
  '240p': 'v4',
};

class _VideoPlayerScreenState extends ConsumerState<VideoPlayerScreen> {
  // Resolved playback sources, produced once by _resolveSources(). The actual
  // player controller lives inside NetflixVideoPlayer — this screen only
  // resolves the (signed, entitlement-checked) URLs and hands them over.
  bool _resolving = true;
  bool _resolveError = false;
  bool _isOffline = false;
  String? _autoUrl; // online adaptive HLS master (the "Auto" quality)
  String? _offlineUrl; // in-app loopback URL for a downloaded copy
  Map<String, String> _resolutions = const {}; // quality label -> signed URL

  final _engagementService = EngagementService();
  VideoReactionState _reactions = VideoReactionState.empty;
  List<VideoComment> _comments = [];
  bool _commentsLoading = true;
  bool _postingComment = false;
  final _commentController = TextEditingController();

  List<VideoModel> _playlist = [];
  bool _playlistLoading = true;

  List<PhotoModel> _videoPhotos = [];

  @override
  void initState() {
    super.initState();
    _resolveSources();
    PlaybackService().logPlay(widget.video.id);
    _loadEngagement();
    _loadPlaylist();
    _loadVideoPhotos();
  }

  /// Resolves what the player should play: a downloaded offline copy if one
  /// exists, otherwise the online adaptive HLS master plus a signed URL for
  /// each ready quality variant. All URLs are resolved server-side (signed,
  /// entitlement-checked) — no raw storage keys are ever handled here.
  Future<void> _resolveSources() async {
    if (mounted) setState(() { _resolving = true; _resolveError = false; });
    try {
      // Offline downloads are mobile-only (dart:io / path_provider).
      if (!kIsWeb &&
          await DownloadsService.instance.isDownloaded(widget.video.id)) {
        await LocalVideoProxy.instance.ensureStarted();
        final url = LocalVideoProxy.instance.urlFor(widget.video.id);
        if (!mounted) return;
        setState(() {
          _isOffline = true;
          _offlineUrl = url;
          _resolving = false;
        });
        return;
      }

      final auto = await VideosService().resolveStreamUrl(widget.video.id);

      // Only offer quality variants that actually exist for this video —
      // fetchStreamQualities reports the HLS ladder tiers (NOT the MP4
      // download tiers, which legacy videos are missing even though they
      // stream fine). Resolve their signed playlist URLs in parallel; skip
      // any that fail rather than blocking.
      Set<String> ready;
      try {
        ready =
            (await VideosService().fetchStreamQualities(widget.video.id)).toSet();
      } catch (_) {
        ready = const {};
      }
      final entries = await Future.wait(
        _qualityVariant.entries.where((e) => ready.contains(e.key)).map(
          (e) async {
            try {
              final url = await VideosService()
                  .resolveVariantStreamUrl(widget.video.id, variant: e.value);
              return MapEntry(e.key, url);
            } catch (_) {
              return null;
            }
          },
        ),
      );
      final resolutions = <String, String>{
        for (final e in entries)
          if (e != null) e.key: e.value,
      };

      if (!mounted) return;
      setState(() {
        _isOffline = false;
        _autoUrl = auto;
        _resolutions = resolutions;
        _resolving = false;
      });
    } catch (_) {
      if (!mounted) return;
      setState(() { _resolveError = true; _resolving = false; });
    }
  }

  Future<void> _loadVideoPhotos() async {
    try {
      final photos = await PhotosService().fetchVideoPhotos(widget.video.id);
      if (!mounted) return;
      setState(() => _videoPhotos = photos);
    } catch (_) {
      // Non-critical — the video plays fine without attached photos.
    }
  }

  Future<void> _loadPlaylist() async {
    try {
      final videos = await VideosService().fetchVideos(
        category: widget.video.category,
      );
      if (!mounted) return;
      setState(() {
        _playlist = videos
            .where((v) => v.id != widget.video.id)
            .take(15)
            .toList();
        _playlistLoading = false;
      });
    } catch (_) {
      if (!mounted) return;
      setState(() => _playlistLoading = false);
    }
  }

  Future<void> _loadEngagement() async {
    try {
      final results = await Future.wait([
        _engagementService.getReactions(widget.video.id),
        _engagementService.getComments(widget.video.id),
      ]);
      if (!mounted) return;
      setState(() {
        _reactions = results[0] as VideoReactionState;
        _comments = results[1] as List<VideoComment>;
        _commentsLoading = false;
      });
    } catch (_) {
      if (!mounted) return;
      setState(() => _commentsLoading = false);
    }
  }

  Future<void> _toggleReaction(String reaction) async {
    try {
      final updated = await _engagementService.toggleReaction(widget.video.id, reaction);
      if (!mounted) return;
      setState(() => _reactions = updated);
    } catch (_) {}
  }

  Future<void> _postComment() async {
    final content = _commentController.text.trim();
    if (content.isEmpty || _postingComment) return;
    setState(() => _postingComment = true);
    try {
      final comment = await _engagementService.postComment(widget.video.id, content);
      if (!mounted) return;
      _commentController.clear();
      setState(() => _comments = [comment, ..._comments]);
    } catch (_) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Failed to post comment')),
      );
    } finally {
      if (mounted) setState(() => _postingComment = false);
    }
  }

  Future<void> _deleteComment(String commentId) async {
    try {
      await _engagementService.deleteComment(widget.video.id, commentId);
      if (!mounted) return;
      setState(() => _comments.removeWhere((c) => c.id == commentId));
    } catch (_) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Cannot delete this comment')),
      );
    }
  }

  @override
  void dispose() {
    // The player controller lives in NetflixVideoPlayer and disposes itself
    // (flushing a final progress update via onProgress). Nothing player-related
    // to tear down here.
    _commentController.dispose();
    super.dispose();
  }

  Widget _buildPlayerArea() {
    if (_resolving) {
      return AspectRatio(
        aspectRatio: 16 / 9,
        child: Stack(
          fit: StackFit.expand,
          children: [
            if (widget.video.thumbnailUrl != null)
              CachedNetworkImage(
                imageUrl: widget.video.thumbnailUrl!,
                fit: BoxFit.cover,
              )
            else
              const ColoredBox(color: Colors.black),
            const ColoredBox(color: Color(0x55000000)),
            const Center(child: CircularProgressIndicator(color: kPrimary)),
          ],
        ),
      );
    }

    if (_resolveError) {
      return AspectRatio(
        aspectRatio: 16 / 9,
        child: _ErrorWidget(onRetry: _resolveSources),
      );
    }

    // No download control inside the player — downloading lives in the
    // reaction row below the video.
    return NetflixVideoPlayer(
      // Rebuild from scratch if the video identity changes.
      key: ValueKey(widget.video.id),
      title: widget.video.title,
      videoId: widget.video.id,
      hlsUrl: _isOffline ? null : _autoUrl,
      mp4Resolutions:
          _isOffline ? {'Offline': _offlineUrl!} : _resolutions,
      startPositionSeconds: widget.video.progressSeconds,
      onProgress: (seconds) =>
          PlaybackService().saveProgress(widget.video.id, seconds),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.black,
      appBar: AppBar(
        backgroundColor: Colors.black,
        foregroundColor: Colors.white,
        title: Text(
          widget.video.title,
          style: const TextStyle(fontSize: 16, fontWeight: FontWeight.w600),
          overflow: TextOverflow.ellipsis,
        ),
        leading: const BackButton(),
      ),
      body: Column(
        children: [
          _buildPlayerArea(),
          Expanded(
            child: Container(
              color: kBackground,
              child: SingleChildScrollView(
                padding: const EdgeInsets.fromLTRB(16, 16, 16, 8),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    // Category badge
                    Container(
                      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
                      decoration: BoxDecoration(
                        color: kPrimary.withValues(alpha: 0.15),
                        borderRadius: BorderRadius.circular(6),
                      ),
                      child: Text(
                        widget.video.categoryLabel,
                        style: const TextStyle(
                          fontSize: 12,
                          fontWeight: FontWeight.w600,
                          color: kPrimary,
                        ),
                      ),
                    ),
                    const SizedBox(height: 12),
                    // Title
                    Text(
                      widget.video.title,
                      style: const TextStyle(
                        fontSize: 18,
                        fontWeight: FontWeight.w700,
                        color: kForeground,
                      ),
                    ),
                    const SizedBox(height: 6),
                    // Duration / resume row
                    Row(
                      children: [
                        if (widget.video.durationSeconds > 0) ...[
                          const Icon(Icons.timer_outlined, size: 14, color: kMutedForeground),
                          const SizedBox(width: 4),
                          Text(
                            widget.video.formattedDuration,
                            style: const TextStyle(fontSize: 13, color: kMutedForeground),
                          ),
                        ],
                        if (widget.video.progressSeconds > 0) ...[
                          const SizedBox(width: 12),
                          const Icon(Icons.history, size: 14, color: kMutedForeground),
                          const SizedBox(width: 4),
                          Text(
                            'Resumed at ${_fmtSeconds(widget.video.progressSeconds)}',
                            style: const TextStyle(fontSize: 12, color: kMutedForeground),
                          ),
                        ],
                      ],
                    ),
                    const SizedBox(height: 16),
                    // Like / Dislike / Download row
                    _ReactionRow(
                      reactions: _reactions,
                      onToggle: _toggleReaction,
                      video: widget.video,
                    ),
                    // Description
                    if (widget.video.description.isNotEmpty) ...[
                      const SizedBox(height: 16),
                      const Divider(),
                      const SizedBox(height: 12),
                      const Text(
                        'About this video',
                        style: TextStyle(
                          fontSize: 14,
                          fontWeight: FontWeight.w700,
                          color: kForeground,
                        ),
                      ),
                      const SizedBox(height: 8),
                      ExpandableText(
                        text: widget.video.description,
                        maxLines: 3,
                        style: const TextStyle(
                            fontSize: 14, color: kMutedForeground, height: 1.5),
                      ),
                    ],
                    // Photos section
                    if (_videoPhotos.isNotEmpty) ...[
                      const SizedBox(height: 16),
                      const Divider(),
                      const SizedBox(height: 12),
                      const Text(
                        'Photos',
                        style: TextStyle(
                          fontSize: 14,
                          fontWeight: FontWeight.w700,
                          color: kForeground,
                        ),
                      ),
                      const SizedBox(height: 8),
                      PhotoGallery(photos: _videoPhotos),
                    ],
                    // Playlist section
                    if (_playlistLoading || _playlist.isNotEmpty) ...[
                      const SizedBox(height: 16),
                      const Divider(),
                      const SizedBox(height: 12),
                      _PlaylistSection(
                        categoryLabel: widget.video.categoryLabel,
                        playlist: _playlist,
                        isLoading: _playlistLoading,
                      ),
                    ],
                    const SizedBox(height: 16),
                    const Divider(),
                    const SizedBox(height: 12),
                    // Comments section
                    _CommentsSection(
                      comments: _comments,
                      isLoading: _commentsLoading,
                      onDelete: _deleteComment,
                    ),
                    // Padding so last comment isn't hidden behind input bar
                    const SizedBox(height: 72),
                  ],
                ),
              ),
            ),
          ),
          // Fixed comment input at bottom
          _CommentInputBar(
            controller: _commentController,
            isPosting: _postingComment,
            onPost: _postComment,
          ),
        ],
      ),
    );
  }

  String _fmtSeconds(int s) {
    final m = s ~/ 60;
    final sec = s % 60;
    return '${m.toString().padLeft(2, '0')}:${sec.toString().padLeft(2, '0')}';
  }
}

// ---------------------------------------------------------------------------
// Reaction row
// ---------------------------------------------------------------------------

class _ReactionRow extends StatelessWidget {
  final VideoReactionState reactions;
  final void Function(String) onToggle;
  final VideoModel video;

  const _ReactionRow({
    required this.reactions,
    required this.onToggle,
    required this.video,
  });

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        _ReactionButton(
          icon: Icons.thumb_up_alt_outlined,
          activeIcon: Icons.thumb_up_alt,
          label: '${reactions.likeCount}',
          isActive: reactions.userReaction == 'like',
          onTap: () => onToggle('like'),
        ),
        const SizedBox(width: 12),
        _ReactionButton(
          icon: Icons.thumb_down_alt_outlined,
          activeIcon: Icons.thumb_down_alt,
          label: '${reactions.dislikeCount}',
          isActive: reactions.userReaction == 'dislike',
          onTap: () => onToggle('dislike'),
        ),
        if (!kIsWeb && video.accessible) ...[
          const SizedBox(width: 4),
          DownloadButton(video: video, darkBackground: false),
        ],
        FavoriteButton(video: video, darkBackground: false),
      ],
    );
  }
}

class _ReactionButton extends StatelessWidget {
  final IconData icon;
  final IconData activeIcon;
  final String label;
  final bool isActive;
  final VoidCallback onTap;

  const _ReactionButton({
    required this.icon,
    required this.activeIcon,
    required this.label,
    required this.isActive,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: AnimatedContainer(
        duration: const Duration(milliseconds: 180),
        padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 8),
        decoration: BoxDecoration(
          color: isActive ? kPrimary.withValues(alpha: 0.12) : kMuted,
          borderRadius: BorderRadius.circular(20),
          border: Border.all(
            color: isActive ? kPrimary : kBorder,
            width: isActive ? 1.5 : 1,
          ),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              isActive ? activeIcon : icon,
              size: 18,
              color: isActive ? kPrimary : kMutedForeground,
            ),
            const SizedBox(width: 6),
            Text(
              label,
              style: TextStyle(
                fontSize: 13,
                fontWeight: FontWeight.w600,
                color: isActive ? kPrimary : kMutedForeground,
              ),
            ),
          ],
        ),
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Comments section
// ---------------------------------------------------------------------------

class _CommentsSection extends StatelessWidget {
  final List<VideoComment> comments;
  final bool isLoading;
  final void Function(String) onDelete;

  const _CommentsSection({
    required this.comments,
    required this.isLoading,
    required this.onDelete,
  });

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          'Messages with Admin (${comments.length})',
          style: const TextStyle(
            fontSize: 14,
            fontWeight: FontWeight.w700,
            color: kForeground,
          ),
        ),
        const SizedBox(height: 12),
        if (isLoading)
          const Center(
            child: Padding(
              padding: EdgeInsets.symmetric(vertical: 24),
              child: CircularProgressIndicator(color: kPrimary, strokeWidth: 2),
            ),
          )
        else if (comments.isEmpty)
          const Padding(
            padding: EdgeInsets.symmetric(vertical: 24),
            child: Center(
              child: Text(
                'Ask a question — only you and the admin can see this',
                textAlign: TextAlign.center,
                style: TextStyle(color: kMutedForeground, fontSize: 13),
              ),
            ),
          )
        else
          ListView.separated(
            shrinkWrap: true,
            physics: const NeverScrollableScrollPhysics(),
            itemCount: comments.length,
            separatorBuilder: (context, index) => const Divider(height: 1),
            itemBuilder: (_, i) => _CommentTile(
              comment: comments[i],
              onDelete: () => onDelete(comments[i].id),
            ),
          ),
      ],
    );
  }
}

class _CommentTile extends StatelessWidget {
  final VideoComment comment;
  final VoidCallback onDelete;

  const _CommentTile({required this.comment, required this.onDelete});

  String _timeAgo(DateTime dt) {
    final diff = DateTime.now().difference(dt);
    if (diff.inSeconds < 60) return 'just now';
    if (diff.inMinutes < 60) return '${diff.inMinutes}m ago';
    if (diff.inHours < 24) return '${diff.inHours}h ago';
    if (diff.inDays < 7) return '${diff.inDays}d ago';
    return '${(diff.inDays / 7).floor()}w ago';
  }

  @override
  Widget build(BuildContext context) {
    final initial = comment.userName.isNotEmpty
        ? comment.userName[0].toUpperCase()
        : '?';

    return Container(
      padding: const EdgeInsets.symmetric(vertical: 10),
      decoration: comment.isAdmin
          ? BoxDecoration(
              color: kPrimary.withValues(alpha: 0.06),
              borderRadius: BorderRadius.circular(8),
            )
          : null,
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          comment.isAdmin
              ? CircleAvatar(
                  radius: 16,
                  backgroundColor: kPrimary,
                  child: const Icon(Icons.support_agent, size: 16, color: Colors.white),
                )
              : CircleAvatar(
                  radius: 16,
                  backgroundColor: kPrimary.withValues(alpha: 0.2),
                  child: Text(
                    initial,
                    style: const TextStyle(
                      fontSize: 13,
                      fontWeight: FontWeight.w700,
                      color: kPrimary,
                    ),
                  ),
                ),
          const SizedBox(width: 10),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Text(
                      comment.isAdmin ? '${comment.userName} (Admin)' : comment.userName,
                      style: const TextStyle(
                        fontSize: 13,
                        fontWeight: FontWeight.w600,
                        color: kForeground,
                      ),
                    ),
                    const SizedBox(width: 8),
                    Text(
                      _timeAgo(comment.createdAt),
                      style: const TextStyle(
                        fontSize: 11,
                        color: kMutedForeground,
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: 4),
                Text(
                  comment.content,
                  style: const TextStyle(
                    fontSize: 13,
                    color: kMutedForeground,
                    height: 1.4,
                  ),
                ),
              ],
            ),
          ),
          if (!comment.isAdmin)
            IconButton(
              icon: const Icon(Icons.delete_outline, size: 18),
              color: kMutedForeground,
              splashRadius: 16,
              onPressed: onDelete,
              tooltip: 'Delete',
            ),
        ],
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Comment input bar
// ---------------------------------------------------------------------------

class _CommentInputBar extends StatelessWidget {
  final TextEditingController controller;
  final bool isPosting;
  final VoidCallback onPost;

  const _CommentInputBar({
    required this.controller,
    required this.isPosting,
    required this.onPost,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      color: kCard,
      padding: EdgeInsets.fromLTRB(
        12,
        8,
        8,
        8 + MediaQuery.of(context).viewInsets.bottom,
      ),
      child: Row(
        children: [
          Expanded(
            child: TextField(
              controller: controller,
              textInputAction: TextInputAction.send,
              onSubmitted: (_) => onPost(),
              decoration: InputDecoration(
                hintText: 'Message admin…',
                hintStyle: const TextStyle(color: kMutedForeground, fontSize: 13),
                filled: true,
                fillColor: kMuted,
                contentPadding:
                    const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
                border: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(20),
                  borderSide: BorderSide.none,
                ),
              ),
              style: const TextStyle(fontSize: 13, color: kForeground),
            ),
          ),
          const SizedBox(width: 4),
          isPosting
              ? const SizedBox(
                  width: 40,
                  height: 40,
                  child: Center(
                    child: SizedBox(
                      width: 20,
                      height: 20,
                      child: CircularProgressIndicator(
                        color: kPrimary,
                        strokeWidth: 2,
                      ),
                    ),
                  ),
                )
              : IconButton(
                  icon: const Icon(Icons.send_rounded),
                  color: kPrimary,
                  onPressed: onPost,
                  tooltip: 'Post comment',
                ),
        ],
      ),
    );
  }
}

// ---------------------------------------------------------------------------
// Error widget
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Playlist section
// ---------------------------------------------------------------------------

class _PlaylistSection extends StatelessWidget {
  final String categoryLabel;
  final List<VideoModel> playlist;
  final bool isLoading;

  const _PlaylistSection({
    required this.categoryLabel,
    required this.playlist,
    required this.isLoading,
  });

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            const Icon(Icons.playlist_play_rounded, size: 20, color: kPrimary),
            const SizedBox(width: 8),
            Text(
              'More $categoryLabel Videos',
              style: const TextStyle(
                fontSize: 14,
                fontWeight: FontWeight.w700,
                color: kForeground,
              ),
            ),
            if (!isLoading && playlist.isNotEmpty) ...[
              const SizedBox(width: 8),
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 7, vertical: 2),
                decoration: BoxDecoration(
                  color: kPrimary.withValues(alpha: 0.15),
                  borderRadius: BorderRadius.circular(10),
                ),
                child: Text(
                  '${playlist.length}',
                  style: const TextStyle(
                    fontSize: 11,
                    fontWeight: FontWeight.w700,
                    color: kPrimary,
                  ),
                ),
              ),
            ],
          ],
        ),
        const SizedBox(height: 12),
        if (isLoading)
          const Padding(
            padding: EdgeInsets.symmetric(vertical: 16),
            child: LinearProgressIndicator(color: kPrimary, backgroundColor: kMuted),
          )
        else
          ListView.separated(
            shrinkWrap: true,
            physics: const NeverScrollableScrollPhysics(),
            itemCount: playlist.length,
            separatorBuilder: (_, __) => const SizedBox(height: 10),
            itemBuilder: (_, i) {
              final v = playlist[i];
              return LibraryVideoTile(
                video: v,
                // Replace (not push) so switching videos from inside the
                // player doesn't stack up player screens — Back should
                // return to wherever the player was originally opened from
                // (Library, Playlist, Home, etc.), not to another player.
                onTap: () => context.replace('/video/${v.id}', extra: v),
              );
            },
          ),
      ],
    );
  }
}

// ---------------------------------------------------------------------------
// Error widget
// ---------------------------------------------------------------------------

class _ErrorWidget extends StatelessWidget {
  final VoidCallback onRetry;
  const _ErrorWidget({required this.onRetry});

  @override
  Widget build(BuildContext context) {
    return Container(
      color: Colors.black,
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          const Icon(Icons.error_outline, color: Colors.white54, size: 48),
          const SizedBox(height: 12),
          const Text('Failed to load video', style: TextStyle(color: Colors.white70)),
          const SizedBox(height: 16),
          TextButton(
            onPressed: onRetry,
            child: const Text('Retry', style: TextStyle(color: kPrimary)),
          ),
        ],
      ),
    );
  }
}
