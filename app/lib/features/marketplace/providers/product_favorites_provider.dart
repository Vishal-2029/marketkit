import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../models/product_favorite_entry.dart';
import '../models/product_model.dart';
import '../services/product_favorites_service.dart';

class ProductFavoritesState {
  final List<ProductFavoriteEntry> entries;

  const ProductFavoritesState({this.entries = const []});

  ProductFavoritesState copyWith({List<ProductFavoriteEntry>? entries}) =>
      ProductFavoritesState(entries: entries ?? this.entries);

  bool isFavorite(String productId) =>
      entries.any((e) => e.product.id == productId);
}

class ProductFavoritesNotifier extends StateNotifier<ProductFavoritesState> {
  ProductFavoritesNotifier() : super(const ProductFavoritesState()) {
    _init();
  }

  final _service = ProductFavoritesService.instance;

  Future<void> _init() async {
    state = state.copyWith(entries: await _service.list());
  }

  Future<void> refresh() async {
    state = state.copyWith(entries: await _service.list());
  }

  Future<void> toggle(ProductModel product) async {
    if (state.isFavorite(product.id)) {
      await _service.remove(product.id);
    } else {
      await _service.add(product);
    }
    state = state.copyWith(entries: await _service.list());
  }

  Future<void> remove(String productId) async {
    await _service.remove(productId);
    state = state.copyWith(entries: await _service.list());
  }
}

final productFavoritesProvider =
    StateNotifierProvider<ProductFavoritesNotifier, ProductFavoritesState>(
        (ref) => ProductFavoritesNotifier());
