import 'dart:async';

import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart';
import 'package:image_picker/image_picker.dart';
import '../../../core/network/dio_client.dart';
import '../../../core/network/api_endpoints.dart';
import '../../../core/utils/upload_filename.dart';
import '../../../core/services/notification_service.dart';
import '../../../core/storage/secure_storage.dart';
import '../models/user_model.dart';

class AuthService {
  final _dio = DioClient().dio;

  Future<void> register({
    required String name,
    required String email,
    required String phone,
    required String password,
    required String mode,
  }) async {
    await _dio.post(ApiEndpoints.register, data: {
      'name': name,
      'email': email,
      'phone': phone,
      'password': password,
      'mode': mode,
    });
  }

  /// Switches the current user's app mode post-registration. Returns whether
  /// this is the account's first-ever entry into [mode] — the caller uses
  /// this to decide whether to show the animated welcome popup.
  Future<bool> setAppMode(String mode) async {
    final res = await _dio.patch(ApiEndpoints.setAppMode, data: {'mode': mode});
    final data = res.data['data'] as Map<String, dynamic>?;
    return data?['first_time'] as bool? ?? false;
  }

  Future<void> sendOtp({required String email, required String password}) async {
    final res = await _dio.post(
      ApiEndpoints.sendOtp,
      data: {
        'email': email.trim().toLowerCase(),
        'password': password,
      },
    );

    // Backend may return 200 with a failure payload.
    // Make sure LoginScreen does not navigate to /otp on wrong password.
    final data = res.data;
    if (data is Map<String, dynamic>) {
      final success = data['success'];
      final error = data['error'];

      if (success == false) {
        throw Exception((error is String && error.isNotEmpty)
            ? error
            : 'Invalid email or password. Please try again.');
      }
      if (error is String && error.isNotEmpty) {
        // Some APIs use { error: '...' } without success flag
        throw Exception(error);
      }
    }
  }


  Future<UserModel> verifyOtp({required String email, required String otp}) async {
    final normalizedOtp = otp.replaceAll(RegExp(r'\D'), '');
    final res = await _dio.post(ApiEndpoints.verifyOtp, data: {
      'email': email.trim().toLowerCase(),
      'otp': normalizedOtp,
    });
    return _persistLoginResponse(res.data);
  }

  /// Signs in with a Google ID token from [google_sign_in].
  Future<UserModel> loginWithGoogle({required String idToken}) async {
    final res = await _dio.post(ApiEndpoints.googleLogin, data: {
      'id_token': idToken,
    });
    return _persistLoginResponse(res.data);
  }

  Future<UserModel> _persistLoginResponse(dynamic body) async {
    if (body is! Map<String, dynamic>) {
      throw Exception('Unexpected server response');
    }
    final data = body['data'];
    if (data is! Map<String, dynamic>) {
      throw Exception(body['error'] as String? ?? 'Login failed');
    }
    final token = data['token'] as String?;
    if (token == null || token.isEmpty) {
      throw Exception('Missing access token in server response');
    }
    final refreshToken = data['refresh_token'] as String?;

    DioClient().setToken(token);

    try {
      await SecureStorage.saveAccessToken(token);
      if (refreshToken != null) {
        await SecureStorage.saveRefreshToken(refreshToken);
      }
    } catch (e) {
      debugPrint('[Auth] SecureStorage save failed: $e');
    }

    final userJson = data['user'];
    final user = await fetchMe() ??
        (userJson is Map<String, dynamic>
            ? UserModel.fromJson(userJson)
            : throw Exception('Missing user profile in server response'));

    try {
      await SecureStorage.saveUser(user.toJsonString());
    } catch (e) {
      debugPrint('[Auth] SecureStorage.saveUser failed: $e');
    }

    if (!kIsWeb) {
      try {
        unawaited(NotificationService().init());
      } catch (e) {
        debugPrint('[Auth] NotificationService init failed: $e');
      }
    }
    return user;
  }

  Future<UserModel?> tryRefresh() async {
    String? savedRefreshToken;
    try {
      savedRefreshToken = await SecureStorage.getRefreshToken().timeout(
        const Duration(seconds: 3),
        onTimeout: () => null,
      );
    } catch (_) {
      return null;
    }

    if (!kIsWeb && savedRefreshToken == null) return null;

    try {
      final res = await _dio.post(
        ApiEndpoints.refresh,
        data: savedRefreshToken != null ? {'refresh_token': savedRefreshToken} : null,
      ).timeout(const Duration(seconds: 5));
      final data = res.data['data'] as Map<String, dynamic>;
      final token = data['token'] as String;
      final newRefreshToken = data['refresh_token'] as String?;

      DioClient().setToken(token);
      await SecureStorage.saveAccessToken(token);
      if (newRefreshToken != null) {
        await SecureStorage.saveRefreshToken(newRefreshToken);
      }

      final user = await fetchMe() ??
          UserModel.fromJson(data['user'] as Map<String, dynamic>);
      await SecureStorage.saveUser(user.toJsonString());

      // Firebase notification setup removed — to be re-added during fresh setup
      if (!kIsWeb) {
        unawaited(NotificationService().init());
      }
      return user;
    } catch (_) {
      return null;
    }
  }

  Future<UserModel?> fetchMe() async {
    try {
      final res = await _dio.get(ApiEndpoints.me);
      final data = res.data['data'] as Map<String, dynamic>;
      return UserModel.fromJson(data);
    } catch (_) {
      return null;
    }
  }

  Future<void> forgotPassword({required String email}) async {
    await _dio.post(ApiEndpoints.forgotPassword, data: {'email': email});
  }

  Future<void> deleteAccount() async {
    await _dio.delete(ApiEndpoints.deleteAccount);
  }

  Future<void> resetPassword({
    required String email,
    required String otp,
    required String newPassword,
  }) async {
    await _dio.post(ApiEndpoints.resetPassword, data: {
      'email': email,
      'otp': otp,
      'new_password': newPassword,
    });
  }

  Future<UserModel> updateProfile({
    required String name,
    required String phone,
  }) async {
    await _dio.patch(ApiEndpoints.updateProfile, data: {
      'name': name,
      'phone': phone,
    });
    final user = await fetchMe();
    if (user == null) throw Exception('Failed to fetch updated profile');
    await SecureStorage.saveUser(user.toJsonString());
    return user;
  }

  Future<String> uploadAvatar(XFile imageFile) async {
    final bytes = await imageFile.readAsBytes();
    final filename = uploadImageFilename(imageFile, fallback: 'avatar.jpg');
    final formData = FormData.fromMap({
      'file': MultipartFile.fromBytes(bytes, filename: filename),
    });
    final res = await _dio.post(ApiEndpoints.avatarUpload, data: formData);
    final avatarUrl = res.data['data']['avatar_url'] as String;
    final user = await fetchMe();
    if (user != null) await SecureStorage.saveUser(user.toJsonString());
    return avatarUrl;
  }

  Future<void> removeAvatar() async {
    await _dio.delete(ApiEndpoints.avatarUpload);
    final user = await fetchMe();
    if (user != null) await SecureStorage.saveUser(user.toJsonString());
  }

  Future<void> changePassword({
    required String currentPassword,
    required String newPassword,
  }) async {
    await _dio.post(ApiEndpoints.changePassword, data: {
      'current_password': currentPassword,
      'new_password': newPassword,
    });
  }

  Future<void> logout() async {
    try {
      final refreshToken = await SecureStorage.getRefreshToken();
      await _dio.post(
        ApiEndpoints.logout,
        data: refreshToken != null ? {'refresh_token': refreshToken} : null,
      );
    } catch (_) {}
    DioClient().clearToken();
    await SecureStorage.clearAll();
  }
}
