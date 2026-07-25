import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/cache/app_cache.dart';
import '../models/post_model.dart';
import '../services/community_service.dart';

enum PostSort { newest, mostReplied }

class PostsState {
  final bool isLoading;
  final bool isLoadingMore;
  final bool hasMore;
  final int currentPage;
  final List<PostModel> allPosts;
  final String activeCategory;
  final String searchQuery;
  final PostSort sortBy;
  final String? error;
  final bool isStale;

  const PostsState({
    this.isLoading = false,
    this.isLoadingMore = false,
    this.hasMore = true,
    this.currentPage = 1,
    this.allPosts = const [],
    this.activeCategory = 'ALL',
    this.searchQuery = '',
    this.sortBy = PostSort.newest,
    this.error,
    this.isStale = false,
  });

  List<PostModel> get posts {
    var list = List<PostModel>.from(allPosts);
    if (searchQuery.isNotEmpty) {
      final q = searchQuery.toLowerCase();
      list = list
          .where((p) =>
              p.title.toLowerCase().contains(q) ||
              p.content.toLowerCase().contains(q))
          .toList();
    }
    if (sortBy == PostSort.mostReplied) {
      list.sort((a, b) => b.replyCount.compareTo(a.replyCount));
    }
    return list;
  }

  PostsState copyWith({
    bool? isLoading,
    bool? isLoadingMore,
    bool? hasMore,
    int? currentPage,
    List<PostModel>? allPosts,
    String? activeCategory,
    String? searchQuery,
    PostSort? sortBy,
    String? error,
    bool? isStale,
    bool clearError = false,
  }) =>
      PostsState(
        isLoading: isLoading ?? this.isLoading,
        isLoadingMore: isLoadingMore ?? this.isLoadingMore,
        hasMore: hasMore ?? this.hasMore,
        currentPage: currentPage ?? this.currentPage,
        allPosts: allPosts ?? this.allPosts,
        activeCategory: activeCategory ?? this.activeCategory,
        searchQuery: searchQuery ?? this.searchQuery,
        sortBy: sortBy ?? this.sortBy,
        error: clearError ? null : (error ?? this.error),
        isStale: isStale ?? this.isStale,
      );
}

class PostsNotifier extends StateNotifier<PostsState> {
  final CommunityService _service;

  PostsNotifier(this._service) : super(const PostsState());

  static const _pageSize = 20;

  String get _cacheKey => 'posts_${state.activeCategory}';

  Future<void> load({String? category}) async {
    state = state.copyWith(
        isLoading: true, clearError: true, currentPage: 1, hasMore: true);

    try {
      final posts = await _service.fetchPosts(
        category: category ??
            (state.activeCategory == 'ALL' ? null : state.activeCategory),
        page: 1,
      );
      await AppCache.putList(
          _cacheKey, posts.map((p) => p.toJson()).toList());
      state = state.copyWith(
        isLoading: false,
        allPosts: posts,
        currentPage: 1,
        hasMore: posts.length >= _pageSize,
        isStale: false,
      );
    } catch (_) {
      await _serveFromCache();
    }
  }

  Future<void> _serveFromCache() async {
    final cached = await AppCache.getList(_cacheKey);
    if (cached != null) {
      state = state.copyWith(
        isLoading: false,
        allPosts: cached.map(PostModel.fromJson).toList(),
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
      final posts = await _service.fetchPosts(
        category:
            state.activeCategory == 'ALL' ? null : state.activeCategory,
        page: nextPage,
      );
      state = state.copyWith(
        isLoadingMore: false,
        allPosts: [...state.allPosts, ...posts],
        currentPage: nextPage,
        hasMore: posts.length >= _pageSize,
      );
    } catch (_) {
      state = state.copyWith(isLoadingMore: false);
    }
  }

  Future<void> setCategory(String cat) async {
    state = state.copyWith(
        activeCategory: cat, searchQuery: '', sortBy: PostSort.newest);
    await load(category: cat == 'ALL' ? null : cat);
  }

  void setSearch(String query) {
    state = state.copyWith(searchQuery: query);
  }

  void setSort(PostSort sort) {
    state = state.copyWith(sortBy: sort);
  }

  Future<void> createPost({
    required String category,
    required String title,
    required String content,
    List<dynamic> images = const [],
  }) async {
    await _service.createPost(
        category: category, title: title, content: content, images: images);
    final savedSearch = state.searchQuery;
    final savedCategory = state.activeCategory;
    await load(category: savedCategory == 'ALL' ? null : savedCategory);
    state = state.copyWith(
      searchQuery: savedSearch,
      activeCategory: savedCategory,
    );
  }
}

final communityServiceProvider = Provider((_) => CommunityService());

final postsProvider = StateNotifierProvider<PostsNotifier, PostsState>(
  (ref) => PostsNotifier(ref.read(communityServiceProvider)),
);
