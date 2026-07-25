import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../models/design_favorite_entry.dart';
import '../models/design_model.dart';
import '../services/design_favorites_service.dart';

class DesignFavoritesState {
  final List<DesignFavoriteEntry> entries;

  const DesignFavoritesState({this.entries = const []});

  DesignFavoritesState copyWith({List<DesignFavoriteEntry>? entries}) =>
      DesignFavoritesState(entries: entries ?? this.entries);

  bool isFavorite(String designId) =>
      entries.any((e) => e.design.id == designId);
}

class DesignFavoritesNotifier extends StateNotifier<DesignFavoritesState> {
  DesignFavoritesNotifier() : super(const DesignFavoritesState()) {
    _init();
  }

  final _service = DesignFavoritesService.instance;

  Future<void> _init() async {
    state = state.copyWith(entries: await _service.list());
  }

  Future<void> refresh() async {
    state = state.copyWith(entries: await _service.list());
  }

  Future<void> toggle(DesignModel design) async {
    if (state.isFavorite(design.id)) {
      await _service.remove(design.id);
    } else {
      await _service.add(design);
    }
    state = state.copyWith(entries: await _service.list());
  }

  Future<void> remove(String designId) async {
    await _service.remove(designId);
    state = state.copyWith(entries: await _service.list());
  }
}

final designFavoritesProvider =
    StateNotifierProvider<DesignFavoritesNotifier, DesignFavoritesState>(
        (ref) => DesignFavoritesNotifier());
