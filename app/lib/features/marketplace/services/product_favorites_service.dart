import '../../../core/cache/app_cache.dart';
import '../models/product_favorite_entry.dart';
import '../models/product_model.dart';

/// Persists liked products locally (device-only, mirrors [FavoritesService]
/// for videos — no server sync).
class ProductFavoritesService {
  ProductFavoritesService._();
  static final ProductFavoritesService instance = ProductFavoritesService._();

  static const _key = 'product_favorites';

  Future<List<ProductFavoriteEntry>> list() async {
    final raw = await AppCache.getList(_key);
    if (raw == null) return [];
    final entries = raw.map(ProductFavoriteEntry.fromJson).toList();
    entries.sort((a, b) => b.favoritedAt.compareTo(a.favoritedAt));
    return entries;
  }

  Future<void> add(ProductModel product) async {
    final entries = await list();
    entries.removeWhere((e) => e.product.id == product.id);
    entries.insert(
        0, ProductFavoriteEntry(product: product, favoritedAt: DateTime.now()));
    await _save(entries);
  }

  Future<void> remove(String productId) async {
    final entries = await list();
    entries.removeWhere((e) => e.product.id == productId);
    await _save(entries);
  }

  Future<void> _save(List<ProductFavoriteEntry> entries) async {
    await AppCache.putList(_key, entries.map((e) => e.toJson()).toList());
  }
}
