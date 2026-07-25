import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart';
import '../storage/secure_storage.dart';
import 'api_endpoints.dart';

// Matches "key: value" (Dart Map.toString style, used for request/response
// body dumps) or quoted "key": "value" (JSON style) for known PII/secret
// field names, so debug-only request logging never prints them in plaintext.
final _sensitiveFieldPattern = RegExp(
  r'''(["']?\b(?:password|current_password|new_password|email|phone|otp|token|refresh_token|access_token|'''
  r'''bank_account_number|bank_ifsc|upi_id|bank_holder_name|name|device_info|avatar_url)\b["']?\s*:\s*)("[^"]*"|[^,}\n]+)''',
  caseSensitive: false,
);

String _redactPII(String s) =>
    s.replaceAllMapped(_sensitiveFieldPattern, (m) => '${m[1]}[REDACTED]');

class DioClient {
  static final DioClient _instance = DioClient._internal();
  factory DioClient() => _instance;

  late final Dio dio;

  // In-memory access token — single source of truth during a session
  String? _accessToken;

  // Called when the server reports this session has been replaced by a new
  // login on another device. Wire this up in main.dart to navigate to /login.
  static VoidCallback? onSessionReplaced;

  DioClient._internal() {
    dio = Dio(BaseOptions(
      baseUrl: ApiEndpoints.baseUrl,
      connectTimeout: const Duration(seconds: 10),
      receiveTimeout: const Duration(seconds: 30),
      headers: {'Content-Type': 'application/json'},
      extra: {'withCredentials': true},
    ));

    dio.interceptors.add(_AuthInterceptor(this));

    if (kDebugMode) {
      dio.interceptors.add(LogInterceptor(
        requestBody: true,
        responseBody: true,
        error: true,
        logPrint: (obj) {
          final s = obj.toString();
          // dio's LogInterceptor prints header lines as " Authorization: ..."
          // (leading space) — trim before matching so the auth header is
          // actually dropped instead of just looking filtered.
          if (s.trim().startsWith('Authorization:')) return;
          debugPrint(_redactPII(s));
        },
      ));
    }
  }

  String? get accessToken => _accessToken;

  void setToken(String token) {
    _accessToken = token;
  }

  void clearToken() {
    _accessToken = null;
  }
}

class _AuthInterceptor extends Interceptor {
  final DioClient client;
  bool _isRefreshing = false;
  final List<({RequestOptions options, ErrorInterceptorHandler handler})>
      _pendingRequests = [];

  _AuthInterceptor(this.client);

  @override
  void onRequest(RequestOptions options, RequestInterceptorHandler handler) {
    // BaseOptions sets application/json globally; that breaks multipart uploads.
    if (options.data is FormData) {
      options.headers.remove('Content-Type');
    }
    final token = client.accessToken;
    if (token != null) {
      options.headers['Authorization'] = 'Bearer $token';
    }
    handler.next(options);
  }

  @override
  void onError(DioException err, ErrorInterceptorHandler handler) async {
    // Detect session_replaced before attempting refresh — no point retrying.
    // response.data isn't always a JSON map (a proxy/gateway error can return
    // plain text or HTML), so guard the type before indexing into it.
    final responseData = err.response?.data;
    final errorCode = responseData is Map ? responseData['error'] as String? : null;
    if (errorCode == 'session_replaced') {
      client.clearToken();
      await SecureStorage.clearAll();
      DioClient.onSessionReplaced?.call();
      handler.next(err);
      return;
    }

    if (err.response?.statusCode == 401 &&
        !err.requestOptions.path.contains('/refresh') &&
        !err.requestOptions.path.contains('/login') &&
        !err.requestOptions.path.contains('/send-otp') &&
        !err.requestOptions.path.contains('/verify-otp')) {
      if (_isRefreshing) {
        // Queue the request *and its handler* so it gets resolved/rejected once
        // the in-flight refresh finishes — otherwise its Future hangs forever.
        _pendingRequests.add((options: err.requestOptions, handler: handler));
        return;
      }

      _isRefreshing = true;

      try {
        // Send saved refresh token in body (mobile has no cookie jar).
        final savedRefreshToken = await SecureStorage.getRefreshToken();
        final response = await client.dio.post(
          ApiEndpoints.refresh,
          data: savedRefreshToken != null ? {'refresh_token': savedRefreshToken} : null,
          options: Options(extra: {'skipAuth': true}),
        );

        final newToken = response.data['data']['token'] as String;
        final newRefreshToken = response.data['data']['refresh_token'] as String?;
        client.setToken(newToken);
        await SecureStorage.saveAccessToken(newToken);
        if (newRefreshToken != null) {
          await SecureStorage.saveRefreshToken(newRefreshToken);
        }

        // Retry the queued requests, resolving/rejecting each one's handler so
        // their callers' Futures complete.
        final pending = List.of(_pendingRequests);
        _pendingRequests.clear();
        for (final p in pending) {
          try {
            final r = await client.dio.fetch(
                p.options..headers['Authorization'] = 'Bearer $newToken');
            p.handler.resolve(r);
          } on DioException catch (e) {
            p.handler.reject(e);
          }
        }
        _isRefreshing = false;

        // Retry the original request
        final retryResponse = await client.dio.fetch(err.requestOptions
          ..headers['Authorization'] = 'Bearer $newToken');
        handler.resolve(retryResponse);
      } catch (_) {
        _isRefreshing = false;
        // Fail every queued request's caller too, so none hang.
        final pending = List.of(_pendingRequests);
        _pendingRequests.clear();
        for (final p in pending) {
          p.handler.reject(err);
        }
        client.clearToken();
        await SecureStorage.clearAll();
        handler.next(err);
      }
      return;
    }

    handler.next(err);
  }
}
