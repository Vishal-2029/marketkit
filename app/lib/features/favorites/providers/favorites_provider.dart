import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../home/models/video_model.dart';
import '../models/favorite_entry.dart';
import '../services/favorites_service.dart';

class FavoritesState {
  final List<FavoriteEntry> entries;

  const FavoritesState({this.entries = const []});

  FavoritesState copyWith({List<FavoriteEntry>? entries}) =>
      FavoritesState(entries: entries ?? this.entries);

  bool isFavorite(String videoId) =>
      entries.any((e) => e.video.id == videoId);
}

class FavoritesNotifier extends StateNotifier<FavoritesState> {
  FavoritesNotifier() : super(const FavoritesState()) {
    _init();
  }

  final _service = FavoritesService.instance;

  Future<void> _init() async {
    state = state.copyWith(entries: await _service.list());
  }

  Future<void> refresh() async {
    state = state.copyWith(entries: await _service.list());
  }

  Future<void> toggle(VideoModel video) async {
    if (state.isFavorite(video.id)) {
      await _service.remove(video.id);
    } else {
      await _service.add(video);
    }
    state = state.copyWith(entries: await _service.list());
  }

  Future<void> remove(String videoId) async {
    await _service.remove(videoId);
    state = state.copyWith(entries: await _service.list());
  }
}

final favoritesProvider =
    StateNotifierProvider<FavoritesNotifier, FavoritesState>(
        (ref) => FavoritesNotifier());
