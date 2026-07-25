import '../../../core/network/dio_client.dart';

class PlaybackService {
  final _dio = DioClient().dio;

  Future<void> logPlay(String videoId) async {
    try {
      await _dio.post('/user/videos/$videoId/playback-log');
    } catch (_) {}
  }

  Future<void> saveProgress(String videoId, int positionSeconds) async {
    try {
      await _dio.post(
        '/user/videos/$videoId/progress',
        data: {'position_seconds': positionSeconds},
      );
    } catch (_) {}
  }
}
