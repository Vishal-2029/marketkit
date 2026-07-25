import 'package:flutter_secure_storage/flutter_secure_storage.dart';

class SecureStorage {
  static const _storage = FlutterSecureStorage(
    aOptions: AndroidOptions(
      encryptedSharedPreferences: true,
    ),
    iOptions: IOSOptions(
      accessibility: KeychainAccessibility.first_unlock_this_device,
    ),
  );

  static const _keyAccessToken = 'access_token';
  static const _keyRefreshToken = 'refresh_token';
  static const _keyUserJson = 'user_json';

  // Access token (in-memory-like but survives app restart for refresh flow)
  static Future<void> saveAccessToken(String token) =>
      _storage.write(key: _keyAccessToken, value: token);

  static Future<String?> getAccessToken() =>
      _storage.read(key: _keyAccessToken);

  static Future<void> clearAccessToken() =>
      _storage.delete(key: _keyAccessToken);

  // Refresh token — persisted so mobile clients can refresh without cookies
  static Future<void> saveRefreshToken(String token) =>
      _storage.write(key: _keyRefreshToken, value: token);

  static Future<String?> getRefreshToken() =>
      _storage.read(key: _keyRefreshToken);

  // User data cache
  static Future<void> saveUser(String json) =>
      _storage.write(key: _keyUserJson, value: json);

  static Future<String?> getUser() => _storage.read(key: _keyUserJson);

  static Future<void> clearUser() => _storage.delete(key: _keyUserJson);

  static Future<void> clearAll() => _storage.deleteAll();
}
