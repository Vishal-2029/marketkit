import '../../../core/cache/app_cache.dart';
import '../models/design_favorite_entry.dart';
import '../models/design_model.dart';

/// Persists liked designs locally (device-only, mirrors [FavoritesService]
/// for videos — no server sync).
class DesignFavoritesService {
  DesignFavoritesService._();
  static final DesignFavoritesService instance = DesignFavoritesService._();

  static const _key = 'design_favorites';

  Future<List<DesignFavoriteEntry>> list() async {
    final raw = await AppCache.getList(_key);
    if (raw == null) return [];
    final entries = raw.map(DesignFavoriteEntry.fromJson).toList();
    entries.sort((a, b) => b.favoritedAt.compareTo(a.favoritedAt));
    return entries;
  }

  Future<void> add(DesignModel design) async {
    final entries = await list();
    entries.removeWhere((e) => e.design.id == design.id);
    entries.insert(
        0, DesignFavoriteEntry(design: design, favoritedAt: DateTime.now()));
    await _save(entries);
  }

  Future<void> remove(String designId) async {
    final entries = await list();
    entries.removeWhere((e) => e.design.id == designId);
    await _save(entries);
  }

  Future<void> _save(List<DesignFavoriteEntry> entries) async {
    await AppCache.putList(_key, entries.map((e) => e.toJson()).toList());
  }
}
