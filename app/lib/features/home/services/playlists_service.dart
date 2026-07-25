import '../../../core/network/api_endpoints.dart';
import '../../../core/network/dio_client.dart';
import '../models/playlist_model.dart';

class PlaylistsService {
  final _dio = DioClient().dio;

  Future<List<PlaylistModel>> fetchPlaylists() async {
    try {
      final res = await _dio.get(ApiEndpoints.playlists);
      final list = res.data['data'] as List<dynamic>;
      return list.map((e) => PlaylistModel.fromJson(e as Map<String, dynamic>)).toList();
    } catch (_) {
      return [];
    }
  }

  Future<PlaylistModel?> fetchPlaylistDetail(String id) async {
    try {
      final res = await _dio.get(ApiEndpoints.playlistDetail(id));
      final data = res.data['data'];
      if (data == null) return null;
      return PlaylistModel.fromJson(data as Map<String, dynamic>);
    } catch (_) {
      return null;
    }
  }
}
