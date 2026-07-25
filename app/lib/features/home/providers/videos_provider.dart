import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/cache/app_cache.dart';
import '../models/video_model.dart';
import '../services/videos_service.dart';

class VideosState {
  final bool isLoading;
  final bool isLoadingMore;
  final bool hasMore;
  final int currentPage;
  final List<VideoModel> videos;
  final String? error;
  final String activeCategory;
  final String searchQuery;
  final bool isStale;

  const VideosState({
    this.isLoading = false,
    this.isLoadingMore = false,
    this.hasMore = true,
    this.currentPage = 1,
    this.videos = const [],
    this.error,
    this.activeCategory = 'ALL',
    this.searchQuery = '',
    this.isStale = false,
  });

  List<VideoModel> get continueWatching => videos
      .where((v) => v.progressSeconds > 30 && v.accessible)
      .toList();

  // Category is filtered server-side; only text search is applied client-side.
  List<VideoModel> get filtered {
    if (searchQuery.isEmpty) return videos;
    final q = searchQuery.toLowerCase();
    return videos
        .where((v) =>
            v.title.toLowerCase().contains(q) ||
            v.categoryLabel.toLowerCase().contains(q))
        .toList();
  }

  VideosState copyWith({
    bool? isLoading,
    bool? isLoadingMore,
    bool? hasMore,
    int? currentPage,
    List<VideoModel>? videos,
    String? error,
    String? activeCategory,
    String? searchQuery,
    bool? isStale,
    bool clearError = false,
  }) =>
      VideosState(
        isLoading: isLoading ?? this.isLoading,
        isLoadingMore: isLoadingMore ?? this.isLoadingMore,
        hasMore: hasMore ?? this.hasMore,
        currentPage: currentPage ?? this.currentPage,
        videos: videos ?? this.videos,
        error: clearError ? null : (error ?? this.error),
        activeCategory: activeCategory ?? this.activeCategory,
        searchQuery: searchQuery ?? this.searchQuery,
        isStale: isStale ?? this.isStale,
      );
}

class VideosNotifier extends StateNotifier<VideosState> {
  final VideosService _service;

  VideosNotifier(this._service) : super(const VideosState());

  static const _pageSize = 20;

  String get _cacheKey => 'videos_${state.activeCategory}';

  Future<void> load() async {
    state = state.copyWith(
        isLoading: true, clearError: true, currentPage: 1, hasMore: true);

    try {
      final videos = await _service.fetchVideos(
        page: 1,
        category: state.activeCategory,
      );
      await AppCache.putList(
          _cacheKey, videos.map((v) => v.toJson()).toList());
      state = state.copyWith(
        isLoading: false,
        videos: videos,
        currentPage: 1,
        hasMore: videos.length >= _pageSize,
        isStale: false,
      );
    } catch (_) {
      await _serveFromCache();
    }
  }

  /// Optimistically removes a video from Continue Watching (progress → 0),
  /// then clears it server-side. Restores the previous state if the server
  /// call fails. Returns whether the removal stuck.
  Future<bool> removeFromContinueWatching(String videoId) async {
    final previous = state.videos;
    state = state.copyWith(
      videos: [
        for (final v in previous) v.id == videoId ? v.withProgress(0) : v,
      ],
    );
    try {
      await _service.clearProgress(videoId);
      return true;
    } catch (_) {
      if (mounted) state = state.copyWith(videos: previous);
      return false;
    }
  }

  Future<void> _serveFromCache() async {
    final cached = await AppCache.getList(_cacheKey);
    if (cached != null) {
      state = state.copyWith(
        isLoading: false,
        videos: cached.map(VideoModel.fromJson).toList(),
        isStale: true,
        hasMore: false,
      );
    } else {
      state = state.copyWith(
        isLoading: false,
        error: 'No internet connection',
      );
    }
  }

  Future<void> loadMore() async {
    if (state.isLoadingMore || !state.hasMore || state.isLoading) return;
    if (!await AppCache.isOnline) return;

    final nextPage = state.currentPage + 1;
    state = state.copyWith(isLoadingMore: true);
    try {
      final videos = await _service.fetchVideos(
        page: nextPage,
        category: state.activeCategory,
      );
      state = state.copyWith(
        isLoadingMore: false,
        videos: [...state.videos, ...videos],
        currentPage: nextPage,
        hasMore: videos.length >= _pageSize,
      );
    } catch (_) {
      state = state.copyWith(isLoadingMore: false);
    }
  }

  Future<void> setCategory(String category) async {
    state = state.copyWith(activeCategory: category, searchQuery: '');
    await load();
  }

  void setSearch(String query) {
    state = state.copyWith(searchQuery: query);
  }
}

final videosServiceProvider = Provider((ref) => VideosService());

final videosProvider = StateNotifierProvider<VideosNotifier, VideosState>(
  (ref) => VideosNotifier(ref.read(videosServiceProvider)),
);
