import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/foundation.dart' show kIsWeb;
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../core/theme/app_colors.dart';
import '../../home/models/playlist_model.dart';
import '../../home/models/video_model.dart';
import '../../home/services/playlists_service.dart';
import '../../home/services/videos_service.dart';
import '../widgets/library_video_tile.dart';

class LibraryScreen extends ConsumerStatefulWidget {
  const LibraryScreen({super.key});

  @override
  ConsumerState<LibraryScreen> createState() => _LibraryScreenState();
}

class _LibraryScreenState extends ConsumerState<LibraryScreen> {
  List<PlaylistModel> _playlists = [];
  List<VideoModel> _standaloneVideos = [];
  bool _loading = true;

  @override
  void initState() {
    super.initState();
    Future.microtask(_load);
  }

  /// Fetches every published video across all pages. The catalog is small
  /// enough (a handful of videos) that this is cheap; stops once a page
  /// comes back short of a full page, so it doesn't over-fetch as it grows.
  Future<List<VideoModel>> _fetchAllVideos() async {
    final svc = VideosService();
    final all = <VideoModel>[];
    var page = 1;
    while (true) {
      final batch = await svc.fetchVideos(page: page);
      all.addAll(batch);
      if (batch.length < 20) break;
      page++;
    }
    return all;
  }

  Future<void> _load() async {
    setState(() => _loading = true);
    final all = await PlaylistsService().fetchPlaylists();

    // For each playlist, fetch detail to get video list with accessible flag.
    final detailed = await Future.wait(
      all.map((p) => PlaylistsService().fetchPlaylistDetail(p.id)),
    );

    final allVideos = await _fetchAllVideos();
    final introVideo = await VideosService().fetchIntroVideo();

    if (!mounted) return;
    final resolved = detailed
        .whereType<PlaylistModel>()
        // Keep only playlists that have at least one accessible video for this user.
        .where((p) => p.videos.any((v) => v.accessible))
        .toList();

    // Videos already shown inside a playlist folder shouldn't be repeated below.
    final inPlaylist = <String>{
      for (final p in resolved) for (final v in p.videos) v.id,
    };

    final standalone = <String, VideoModel>{};
    for (final v in [...allVideos, ?introVideo]) {
      if (v.accessible && !inPlaylist.contains(v.id)) {
        standalone[v.id] = v;
      }
    }

    setState(() {
      _playlists = resolved;
      _standaloneVideos = standalone.values.toList();
      _loading = false;
    });
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: kBackground,
      appBar: AppBar(
        backgroundColor: kBackground,
        title: const Text(
          'My Library',
          style: TextStyle(fontWeight: FontWeight.w700, fontSize: 18),
        ),
        centerTitle: false,
        elevation: 0,
        actions: [
          if (!kIsWeb)
            IconButton(
              icon: const Icon(Icons.download_for_offline_outlined),
              tooltip: 'Downloads',
              onPressed: () => context.push('/downloads'),
            ),
        ],
      ),
      body: SafeArea(
        child: _loading
            ? _shimmer()
            : (_playlists.isEmpty && _standaloneVideos.isEmpty)
                ? _emptyState()
                : RefreshIndicator(
                    color: kPrimary,
                    onRefresh: _load,
                    child: ListView(
                      padding: const EdgeInsets.only(bottom: 32),
                      children: [
                        if (_playlists.isNotEmpty)
                          _PlaylistFolderGrid(playlists: _playlists),
                        if (_standaloneVideos.isNotEmpty)
                          _StandaloneSection(videos: _standaloneVideos),
                      ],
                    ),
                  ),
      ),
    );
  }

  Widget _shimmer() {
    return ListView.builder(
      padding: const EdgeInsets.all(16),
      itemCount: 3,
      itemBuilder: (_, __) => Container(
        height: 180,
        margin: const EdgeInsets.only(bottom: 24),
        decoration: BoxDecoration(
          color: kMuted,
          borderRadius: BorderRadius.circular(12),
        ),
      ),
    );
  }

  Widget _emptyState() {
    return const Center(
      child: Padding(
        padding: EdgeInsets.all(32),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.video_library_outlined, size: 56, color: kMutedForeground),
            SizedBox(height: 12),
            Text(
              'No accessible videos yet',
              style: TextStyle(
                  fontSize: 16,
                  fontWeight: FontWeight.w600,
                  color: kForeground),
            ),
            SizedBox(height: 6),
            Text(
              'Upgrade your plan to unlock content.',
              textAlign: TextAlign.center,
              style: TextStyle(fontSize: 13, color: kMutedForeground),
            ),
          ],
        ),
      ),
    );
  }
}

class _StandaloneSection extends StatelessWidget {
  final List<VideoModel> videos;
  const _StandaloneSection({required this.videos});

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 20, 16, 8),
          child: Row(
            children: [
              const Expanded(
                child: Text(
                  'Free Videos',
                  style: TextStyle(
                    fontSize: 15,
                    fontWeight: FontWeight.w700,
                    color: kForeground,
                  ),
                ),
              ),
              Text(
                '${videos.length} ${videos.length == 1 ? 'video' : 'videos'}',
                style: const TextStyle(fontSize: 12, color: kMutedForeground),
              ),
            ],
          ),
        ),
        ListView.separated(
          shrinkWrap: true,
          physics: const NeverScrollableScrollPhysics(),
          padding: const EdgeInsets.symmetric(horizontal: 16),
          itemCount: videos.length,
          separatorBuilder: (_, __) => const SizedBox(height: 10),
          itemBuilder: (_, i) {
            final v = videos[i];
            return LibraryVideoTile(
              video: v,
              onTap: () => context.push('/video/${v.id}', extra: v),
            );
          },
        ),
        const SizedBox(height: 4),
        const Divider(color: kBorder, indent: 16, endIndent: 16),
      ],
    );
  }
}

class _PlaylistFolderGrid extends StatelessWidget {
  final List<PlaylistModel> playlists;
  const _PlaylistFolderGrid({required this.playlists});

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 20, 16, 8),
          child: Row(
            children: [
              const Expanded(
                child: Text(
                  'Playlists',
                  style: TextStyle(
                    fontSize: 15,
                    fontWeight: FontWeight.w700,
                    color: kForeground,
                  ),
                ),
              ),
              Text(
                '${playlists.length} ${playlists.length == 1 ? 'playlist' : 'playlists'}',
                style: const TextStyle(fontSize: 12, color: kMutedForeground),
              ),
            ],
          ),
        ),
        GridView.builder(
          shrinkWrap: true,
          physics: const NeverScrollableScrollPhysics(),
          padding: const EdgeInsets.symmetric(horizontal: 16),
          gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
            crossAxisCount: 2,
            mainAxisSpacing: 12,
            crossAxisSpacing: 12,
            childAspectRatio: 0.78,
          ),
          itemCount: playlists.length,
          itemBuilder: (_, i) => _PlaylistFolderCard(playlist: playlists[i]),
        ),
      ],
    );
  }
}

class _PlaylistFolderCard extends StatelessWidget {
  final PlaylistModel playlist;
  const _PlaylistFolderCard({required this.playlist});

  @override
  Widget build(BuildContext context) {
    final accessibleCount =
        playlist.videos.where((v) => v.accessible).length;

    return GestureDetector(
      onTap: () => context.push('/playlists/${playlist.id}', extra: playlist),
      child: Container(
        decoration: BoxDecoration(
          color: kCard,
          borderRadius: BorderRadius.circular(12),
          boxShadow: [
            BoxShadow(
              color: Colors.black.withValues(alpha: 0.05),
              blurRadius: 6,
              offset: const Offset(0, 2),
            ),
          ],
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            AspectRatio(
              aspectRatio: 16 / 10,
              child: ClipRRect(
                borderRadius:
                    const BorderRadius.vertical(top: Radius.circular(12)),
                child: Stack(
                  fit: StackFit.expand,
                  children: [
                    playlist.thumbnailUrl.isNotEmpty
                        ? CachedNetworkImage(
                            imageUrl: playlist.thumbnailUrl,
                            fit: BoxFit.cover,
                            placeholder: (_, __) => Container(color: kMuted),
                            errorWidget: (_, __, ___) => Container(
                              color: kMuted,
                              child: const Icon(Icons.video_library_rounded,
                                  color: kMutedForeground, size: 28),
                            ),
                          )
                        : Container(
                            color: kMuted,
                            child: const Icon(Icons.video_library_rounded,
                                color: kMutedForeground, size: 28),
                          ),
                    Positioned(
                      bottom: 6,
                      right: 6,
                      child: Container(
                        padding: const EdgeInsets.symmetric(
                            horizontal: 6, vertical: 2),
                        decoration: BoxDecoration(
                          color: Colors.black.withValues(alpha: 0.7),
                          borderRadius: BorderRadius.circular(4),
                        ),
                        child: Text(
                          '$accessibleCount ${accessibleCount == 1 ? 'video' : 'videos'}',
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
            ),
            Padding(
              padding: const EdgeInsets.fromLTRB(8, 8, 8, 8),
              child: Text(
                playlist.name,
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
                style: const TextStyle(
                  fontSize: 12,
                  fontWeight: FontWeight.w600,
                  color: kForeground,
                  height: 1.3,
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
