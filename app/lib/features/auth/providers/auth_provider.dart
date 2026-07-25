import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:image_picker/image_picker.dart';
import '../../../core/network/dio_client.dart';
import '../../../core/storage/secure_storage.dart';
import '../models/user_model.dart';
import '../services/auth_service.dart';

class AuthState {
  final bool isLoading;
  final UserModel? user;
  final String? error;

  const AuthState({
    this.isLoading = false,
    this.user,
    this.error,
  });

  bool get isLoggedIn => user != null;
  bool get hasSubscription => user?.activeSubscriptions.isNotEmpty ?? false;

  AuthState copyWith({
    bool? isLoading,
    UserModel? user,
    String? error,
    bool clearUser = false,
    bool clearError = false,
  }) =>
      AuthState(
        isLoading: isLoading ?? this.isLoading,
        user: clearUser ? null : (user ?? this.user),
        error: clearError ? null : (error ?? this.error),
      );
}

class AuthNotifier extends StateNotifier<AuthState> {
  final AuthService _service;

  AuthNotifier(this._service) : super(const AuthState());

  Future<bool> tryRefresh() async {
    state = state.copyWith(isLoading: true, clearError: true);
    final user = await _service.tryRefresh();
    state = state.copyWith(isLoading: false, user: user, clearUser: user == null);
    return user != null;
  }

  Future<void> fetchMe() async {
    final user = await _service.fetchMe();
    if (user != null) state = state.copyWith(user: user);
  }

  Future<void> register({
    required String name,
    required String email,
    required String phone,
    required String password,
    required String mode,
  }) async {
    state = state.copyWith(isLoading: true, clearError: true);
    try {
      await _service.register(
        name: name, email: email, phone: phone, password: password, mode: mode,
      );
      state = state.copyWith(isLoading: false);
    } catch (e) {
      state = state.copyWith(isLoading: false, error: _parseError(e));
      rethrow;
    }
  }

  /// Switches the current user's app mode post-registration. Returns whether
  /// this was the account's first-ever entry into [mode].
  Future<bool> setAppMode(String mode) async {
    return _service.setAppMode(mode);
  }

  Future<void> sendOtp({required String email, required String password}) async {
    state = state.copyWith(isLoading: true, clearError: true);
    try {
      await _service.sendOtp(email: email, password: password);
      state = state.copyWith(isLoading: false);
    } catch (e) {
      state = state.copyWith(isLoading: false, error: _parseError(e));
      rethrow;
    }
  }

  Future<void> verifyOtp({required String email, required String otp}) async {
    state = state.copyWith(isLoading: true, clearError: true);
    try {
      final user = await _service.verifyOtp(email: email, otp: otp);
      state = state.copyWith(isLoading: false, user: user);
    } catch (e) {
      state = state.copyWith(isLoading: false, error: _parseError(e));
      rethrow;
    }
  }

  Future<void> loginWithGoogle({required String idToken}) async {
    state = state.copyWith(isLoading: true, clearError: true);
    try {
      final user = await _service.loginWithGoogle(idToken: idToken);
      state = state.copyWith(isLoading: false, user: user);
    } catch (e) {
      state = state.copyWith(isLoading: false, error: _parseError(e));
      rethrow;
    }
  }

  Future<void> uploadAvatar(XFile imageFile) async {
    await _service.uploadAvatar(imageFile);
    await fetchMe();
  }

  Future<void> removeAvatar() async {
    await _service.removeAvatar();
    await fetchMe();
  }

  Future<void> updateProfile({
    required String name,
    required String phone,
  }) async {
    final user = await _service.updateProfile(name: name, phone: phone);
    state = state.copyWith(user: user);
  }

  Future<void> changePassword({
    required String currentPassword,
    required String newPassword,
  }) async {
    await _service.changePassword(
      currentPassword: currentPassword,
      newPassword: newPassword,
    );
  }

  Future<void> deleteAccount() async {
    await _service.deleteAccount();
    DioClient().clearToken();
    await SecureStorage.clearAll();
    state = const AuthState();
  }

  Future<void> logout() async {
    await _service.logout();
    state = const AuthState();
  }

  String _parseError(Object e) {
    try {
      // ignore: avoid_dynamic_calls
      final data = (e as dynamic).response?.data;
      if (data is Map && data['error'] != null) return data['error'] as String;
    } catch (_) {}
    return e.toString();
  }
}

final authServiceProvider = Provider((ref) => AuthService());

final authProvider = StateNotifierProvider<AuthNotifier, AuthState>(
  (ref) => AuthNotifier(ref.read(authServiceProvider)),
);
