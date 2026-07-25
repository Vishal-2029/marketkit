import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../models/product_category_model.dart';
import '../models/product_model.dart';
import '../services/market_service.dart';

class ProductsState {
  final bool isLoading;
  final bool isLoadingMore;
  final bool hasMore;
  final int currentPage;
  final List<ProductModel> products;
  final String searchQuery;
  final String? categoryId;
  final String? error;

  const ProductsState({
    this.isLoading = false,
    this.isLoadingMore = false,
    this.hasMore = true,
    this.currentPage = 1,
    this.products = const [],
    this.searchQuery = '',
    this.categoryId,
    this.error,
  });

  ProductsState copyWith({
    bool? isLoading,
    bool? isLoadingMore,
    bool? hasMore,
    int? currentPage,
    List<ProductModel>? products,
    String? searchQuery,
    String? categoryId,
    bool clearCategory = false,
    String? error,
    bool clearError = false,
  }) =>
      ProductsState(
        isLoading: isLoading ?? this.isLoading,
        isLoadingMore: isLoadingMore ?? this.isLoadingMore,
        hasMore: hasMore ?? this.hasMore,
        currentPage: currentPage ?? this.currentPage,
        products: products ?? this.products,
        searchQuery: searchQuery ?? this.searchQuery,
        categoryId: clearCategory ? null : (categoryId ?? this.categoryId),
        error: clearError ? null : (error ?? this.error),
      );
}

class ProductsNotifier extends StateNotifier<ProductsState> {
  final MarketService _service;

  ProductsNotifier(this._service) : super(const ProductsState());

  static const _pageSize = 20;

  Future<void> load() async {
    state = state.copyWith(
        isLoading: true, clearError: true, currentPage: 1, hasMore: true);
    try {
      final products = await _service.fetchProducts(
        search: state.searchQuery,
        categoryId: state.categoryId,
        page: 1,
      );
      state = state.copyWith(
        isLoading: false,
        products: products,
        currentPage: 1,
        hasMore: products.length >= _pageSize,
      );
    } catch (_) {
      state = state.copyWith(
        isLoading: false,
        error: 'Could not load products. Check your connection.',
      );
    }
  }

  Future<void> loadMore() async {
    if (state.isLoadingMore || !state.hasMore || state.isLoading) return;
    final nextPage = state.currentPage + 1;
    state = state.copyWith(isLoadingMore: true);
    try {
      final products = await _service.fetchProducts(
        search: state.searchQuery,
        categoryId: state.categoryId,
        page: nextPage,
      );
      state = state.copyWith(
        isLoadingMore: false,
        products: [...state.products, ...products],
        currentPage: nextPage,
        hasMore: products.length >= _pageSize,
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

final productsProvider = StateNotifierProvider<ProductsNotifier, ProductsState>(
  (ref) => ProductsNotifier(ref.read(marketServiceProvider)),
);

final categoriesProvider =
    FutureProvider.autoDispose<List<ProductCategoryModel>>((ref) async {
  return ref.read(marketServiceProvider).fetchCategories();
});

/// A capped preview of products for one home-page section — separate from
/// [productsProvider] so browsing sections doesn't disturb the main
/// search/filter grid's state.
final sectionProductsProvider = FutureProvider.autoDispose
    .family<List<ProductModel>, String>((ref, categoryId) async {
  final products = await ref
      .read(marketServiceProvider)
      .fetchProducts(categoryId: categoryId);
  return products.take(10).toList();
});
