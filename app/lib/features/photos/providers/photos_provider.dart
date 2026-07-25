import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/cache/app_cache.dart';
import '../models/photo_model.dart';
import '../services/photos_service.dart';

class PhotosState {
  final bool isLoading;
  final List<PhotoModel> photos;
  final String? error;
  final bool isStale;

  const PhotosState({
    this.isLoading = false,
    this.photos = const [],
    this.error,
    this.isStale = false,
  });

  PhotosState copyWith({
    bool? isLoading,
    List<PhotoModel>? photos,
    String? error,
    bool? isStale,
    bool clearError = false,
  }) =>
      PhotosState(
        isLoading: isLoading ?? this.isLoading,
        photos: photos ?? this.photos,
        error: clearError ? null : (error ?? this.error),
        isStale: isStale ?? this.isStale,
      );
}

class PhotosNotifier extends StateNotifier<PhotosState> {
  final PhotosService _service;

  PhotosNotifier(this._service) : super(const PhotosState());

  static const _cacheKey = 'photos';

  Future<void> load() async {
    state = state.copyWith(isLoading: true, clearError: true);

    try {
      final photos = await _service.fetchPublished();
      await AppCache.putList(
          _cacheKey, photos.map((p) => p.toJson()).toList());
      state = state.copyWith(isLoading: false, photos: photos, isStale: false);
    } catch (_) {
      await _serveFromCache();
    }
  }

  Future<void> _serveFromCache() async {
    final cached = await AppCache.getList(_cacheKey);
    if (cached != null) {
      state = state.copyWith(
        isLoading: false,
        photos: cached.map(PhotoModel.fromJson).toList(),
        isStale: true,
      );
    } else {
      state = state.copyWith(
        isLoading: false,
        error: 'No internet connection',
      );
    }
  }
}

final photosServiceProvider = Provider((ref) => PhotosService());

final photosProvider = StateNotifierProvider<PhotosNotifier, PhotosState>(
  (ref) => PhotosNotifier(ref.read(photosServiceProvider)),
);
