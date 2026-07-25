import '../../../core/cache/app_cache.dart';
import '../../home/models/video_model.dart';
import '../models/favorite_entry.dart';

/// Persists starred videos locally (device-only, like Downloads — no server sync).
class FavoritesService {
  FavoritesService._();
  static final FavoritesService instance = FavoritesService._();

  static const _key = 'favorites';

  Future<List<FavoriteEntry>> list() async {
    final raw = await AppCache.getList(_key);
    if (raw == null) return [];
    final entries = raw.map(FavoriteEntry.fromJson).toList();
    entries.sort((a, b) => b.favoritedAt.compareTo(a.favoritedAt));
    return entries;
  }

  Future<void> add(VideoModel video) async {
    final entries = await list();
    entries.removeWhere((e) => e.video.id == video.id);
    entries.insert(0, FavoriteEntry(video: video, favoritedAt: DateTime.now()));
    await _save(entries);
  }

  Future<void> remove(String videoId) async {
    final entries = await list();
    entries.removeWhere((e) => e.video.id == videoId);
    await _save(entries);
  }

  Future<void> _save(List<FavoriteEntry> entries) async {
    await AppCache.putList(_key, entries.map((e) => e.toJson()).toList());
  }
}
