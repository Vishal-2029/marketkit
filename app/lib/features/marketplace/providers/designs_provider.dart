import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../models/design_category_model.dart';
import '../models/design_model.dart';
import '../services/market_service.dart';

class DesignsState {
  final bool isLoading;
  final bool isLoadingMore;
  final bool hasMore;
  final int currentPage;
  final List<DesignModel> designs;
  final String searchQuery;
  final String? categoryId;
  final String? error;

  const DesignsState({
    this.isLoading = false,
    this.isLoadingMore = false,
    this.hasMore = true,
    this.currentPage = 1,
    this.designs = const [],
    this.searchQuery = '',
    this.categoryId,
    this.error,
  });

  DesignsState copyWith({
    bool? isLoading,
    bool? isLoadingMore,
    bool? hasMore,
    int? currentPage,
    List<DesignModel>? designs,
    String? searchQuery,
    String? categoryId,
    bool clearCategory = false,
    String? error,
    bool clearError = false,
  }) =>
      DesignsState(
        isLoading: isLoading ?? this.isLoading,
        isLoadingMore: isLoadingMore ?? this.isLoadingMore,
        hasMore: hasMore ?? this.hasMore,
        currentPage: currentPage ?? this.currentPage,
        designs: designs ?? this.designs,
        searchQuery: searchQuery ?? this.searchQuery,
        categoryId: clearCategory ? null : (categoryId ?? this.categoryId),
        error: clearError ? null : (error ?? this.error),
      );
}

class DesignsNotifier extends StateNotifier<DesignsState> {
  final MarketService _service;

  DesignsNotifier(this._service) : super(const DesignsState());

  static const _pageSize = 20;

  Future<void> load() async {
    state = state.copyWith(
        isLoading: true, clearError: true, currentPage: 1, hasMore: true);
    try {
      final designs = await _service.fetchDesigns(
        search: state.searchQuery,
        categoryId: state.categoryId,
        page: 1,
      );
      state = state.copyWith(
        isLoading: false,
        designs: designs,
        currentPage: 1,
        hasMore: designs.length >= _pageSize,
      );
    } catch (_) {
      state = state.copyWith(
        isLoading: false,
        error: 'Could not load designs. Check your connection.',
      );
    }
  }

  Future<void> loadMore() async {
    if (state.isLoadingMore || !state.hasMore || state.isLoading) return;
    final nextPage = state.currentPage + 1;
    state = state.copyWith(isLoadingMore: true);
    try {
      final designs = await _service.fetchDesigns(
        search: state.searchQuery,
        categoryId: state.categoryId,
        page: nextPage,
      );
      state = state.copyWith(
        isLoadingMore: false,
        designs: [...state.designs, ...designs],
        currentPage: nextPage,
        hasMore: designs.length >= _pageSize,
      );
    } catch (_) {
      state = state.copyWith(isLoadingMore: false);
    }
  }

  Future<void> setSearch(String query) async {
    state = state.copyWith(searchQuery: query, clearCategory: true);
    await load();
  }

  Future<void> setCategory(String? categoryId) async {
    state = state.copyWith(
      categoryId: categoryId,
      clearCategory: categoryId == null,
      searchQuery: '',
    );
    await load();
  }
}

final marketServiceProvider = Provider((_) => MarketService());

final designsProvider = StateNotifierProvider<DesignsNotifier, DesignsState>(
  (ref) => DesignsNotifier(ref.read(marketServiceProvider)),
);

final categoriesProvider =
    FutureProvider.autoDispose<List<DesignCategoryModel>>((ref) async {
  return ref.read(marketServiceProvider).fetchCategories();
});

/// A capped preview of designs for one home-page section — separate from
/// [designsProvider] so browsing sections doesn't disturb the main
/// search/filter grid's state.
final sectionDesignsProvider = FutureProvider.autoDispose
    .family<List<DesignModel>, String>((ref, categoryId) async {
  final designs = await ref
      .read(marketServiceProvider)
      .fetchDesigns(categoryId: categoryId);
  return designs.take(10).toList();
});
