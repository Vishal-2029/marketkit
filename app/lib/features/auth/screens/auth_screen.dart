import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:google_sign_in/google_sign_in.dart';
import '../../../core/config/google_auth_config.dart';
import '../../../core/services/app_mode_service.dart';
import '../../../core/theme/app_colors.dart';
import '../providers/auth_provider.dart';
import '../services/auth_service.dart';
import '../widgets/auth_widgets.dart';

/// Combined Login/Sign up screen. Which side is shown is local widget state
/// (`_isLogin`) toggled in place via [AuthToggle] or the bottom link — no
/// route push, no slide transition between them, just the form body
/// swapping under the same card.
///
/// Reached two ways:
///  - `/login`: plain entry point, opens on the login side, no mode.
///  - `/register`: opens on the signup side, carrying the `mode`
///    (`learning`/`market`) chosen on the mode chooser before an account
///    exists yet — signup can't proceed without one.
class AuthScreen extends ConsumerStatefulWidget {
  final String? mode;
  final bool startOnSignup;

  const AuthScreen({super.key, this.mode, this.startOnSignup = false});

  @override
  ConsumerState<AuthScreen> createState() => _AuthScreenState();
}

class _AuthScreenState extends ConsumerState<AuthScreen> {
  late bool _isLogin = !widget.startOnSignup;
  String? _pendingMode;

  // Login fields
  final _loginEmailCtrl = TextEditingController();
  final _loginPasswordCtrl = TextEditingController();
  final _obscureLogin = ValueNotifier<bool>(true);
  final _loading = ValueNotifier<bool>(false);
  final _googleLoading = ValueNotifier<bool>(false);

  // Signup fields
  final _nameCtrl = TextEditingController();
  final _phoneCtrl = TextEditingController();
  final _signupEmailCtrl = TextEditingController();
  final _signupPasswordCtrl = TextEditingController();
  bool _obscureSignup = true;
  String? _signupError;

  static final _googleSignIn = GoogleSignIn(
    scopes: const ['email', 'profile'],
    serverClientId: GoogleAuthConfig.webClientId,
  );

  @override
  void initState() {
    super.initState();
    _pendingMode = widget.mode;
    if (!_isLogin && _pendingMode == null) {
      // Landed on the signup side with no mode (e.g. a stale deep link) —
      // fall back to whatever mode was last picked, or bounce to the
      // chooser since signup can't proceed without one.
      WidgetsBinding.instance.addPostFrameCallback((_) async {
        final pending = await AppModeService.getPendingMode();
        if (!mounted) return;
        if (pending != null) {
          setState(() => _pendingMode = pending);
        } else {
          context.go('/choose-mode');
        }
      });
    }
  }

  @override
  void dispose() {
    _loginEmailCtrl.dispose();
    _loginPasswordCtrl.dispose();
    _obscureLogin.dispose();
    _loading.dispose();
    _googleLoading.dispose();
    _nameCtrl.dispose();
    _phoneCtrl.dispose();
    _signupEmailCtrl.dispose();
    _signupPasswordCtrl.dispose();
    super.dispose();
  }

  static bool _isValidEmail(String email) =>
      RegExp(r'^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$')
          .hasMatch(email.trim().toLowerCase());

  static bool _isValidPhone(String p) =>
      RegExp(r'^\+?[0-9]{7,15}$').hasMatch(p);

  int _strength(String p) {
    if (p.isEmpty) return 0;
    if (p.length < 6) return 1;
    final s = (p.contains(RegExp(r'[a-zA-Z]')) ? 1 : 0) +
        (p.contains(RegExp(r'[0-9]')) ? 1 : 0) +
        (p.contains(RegExp(r'[^a-zA-Z0-9]')) ? 1 : 0);
    if (p.length >= 10 && s >= 2) return 3;
    if (p.length >= 6 && s >= 1) return 2;
    return 1;
  }

  Color _strengthColor(int s) => s == 1
      ? kDanger
      : s == 2
          ? kPrimary
          : kSuccess;

  String _strengthLabel(int s) =>
      s == 1 ? 'Weak' : s == 2 ? 'Medium' : 'Strong';

  /// Switches to the signup side in place. Signup needs a mode, so if none
  /// is known yet this defers to the mode chooser instead of toggling.
  Future<void> _switchToSignup() async {
    if (_pendingMode != null) {
      setState(() => _isLogin = false);
      return;
    }
    final pending = await AppModeService.getPendingMode();
    if (!mounted) return;
    if (pending != null) {
      setState(() {
        _pendingMode = pending;
        _isLogin = false;
      });
    } else {
      context.push('/choose-mode');
    }
  }

  void _switchToLogin() => setState(() => _isLogin = true);

  Future<void> _goBackToChooser() async {
    await AppModeService.clearPendingMode();
    if (!mounted) return;
    context.go('/choose-mode');
  }

  // ---- Login ----

  Future<void> _continue() async {
    final email = _loginEmailCtrl.text.trim();
    final password = _loginPasswordCtrl.text;
    if (email.isEmpty || password.isEmpty) {
      _showErrorDialog('Email and password are required.');
      return;
    }
    if (!_isValidEmail(email)) {
      _showErrorDialog('Please enter a valid email address.');
      return;
    }

    _loading.value = true;
    try {
      await AuthService().sendOtp(email: email, password: password);
      if (!mounted) return;
      // Passed in-memory via route `extra` (never persisted to disk) so the
      // resend-OTP flow can re-authenticate without storing the plaintext
      // password anywhere.
      context.push('/otp', extra: {
        'email': email.trim().toLowerCase(),
        'password': password,
      });
    } on DioException catch (e) {
      if (!mounted) return;
      _loading.value = false;
      _loginEmailCtrl.text = email;
      _loginPasswordCtrl.text = password;
      String msg;
      if (e.response?.statusCode == 401) {
        msg = 'Invalid email or password. Please try again.';
      } else if (e.type == DioExceptionType.connectionTimeout ||
          e.type == DioExceptionType.receiveTimeout ||
          e.type == DioExceptionType.sendTimeout) {
        msg = 'Server is waking up. Please wait a moment and try again.';
      } else if (e.type == DioExceptionType.connectionError) {
        msg = 'No internet connection. Please check your network.';
      } else {
        msg = 'Something went wrong (${e.response?.statusCode ?? 'no response'}). Please try again.';
      }
      _showErrorDialog(msg);
    } catch (e) {
      if (!mounted) return;
      _loading.value = false;
      _loginEmailCtrl.text = email;
      _loginPasswordCtrl.text = password;
      _showErrorDialog('Unexpected error. Please try again.');
    }
  }

  Future<void> _continueWithGoogle() async {
    if (_googleLoading.value || _loading.value) return;
    _googleLoading.value = true;
    try {
      // Force account picker each time so switching accounts is easy.
      await _googleSignIn.signOut();
      final account = await _googleSignIn.signIn();
      if (account == null) {
        // User cancelled the picker — not an error.
        return;
      }
      final auth = await account.authentication;
      final idToken = auth.idToken;
      if (idToken == null || idToken.isEmpty) {
        _showErrorDialog(
          'Google Sign-In did not return an ID token. Check that the Web client ID is configured.',
        );
        return;
      }

      await ref.read(authProvider.notifier).loginWithGoogle(idToken: idToken);
      if (!mounted) return;

      final user = ref.read(authProvider).user;
      final mode = user == null
          ? null
          : await AppModeService.resolveMode(user.id, user.currentAppMode);
      if (!mounted) return;
      context.go(AppModeService.landingRoute(mode));
    } on DioException catch (e) {
      if (!mounted) return;
      final msg = e.response?.data is Map
          ? (e.response!.data['error'] as String?)
          : null;
      if (e.response?.statusCode == 401) {
        _showErrorDialog(msg ?? 'Google sign-in was rejected. Please try again.');
      } else if (e.type == DioExceptionType.connectionError) {
        _showErrorDialog('No internet connection. Please check your network.');
      } else {
        _showErrorDialog(msg ?? 'Google sign-in failed. Please try again.');
      }
    } catch (e) {
      if (!mounted) return;
      _showErrorDialog('Google sign-in failed. Please try again.');
    } finally {
      if (mounted) _googleLoading.value = false;
    }
  }

  void _showErrorDialog(String message) {
    showDialog(
      context: context,
      builder: (_) => AlertDialog(
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
        title: const Row(
          children: [
            Icon(Icons.error_outline_rounded, color: kDanger, size: 22),
            SizedBox(width: 8),
            Text('Error', style: TextStyle(fontSize: 17, fontWeight: FontWeight.w600)),
          ],
        ),
        content: Text(message, style: const TextStyle(fontSize: 14, color: Color(0xFF555555))),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('OK', style: TextStyle(color: kPrimary, fontWeight: FontWeight.w600)),
          ),
        ],
      ),
    );
  }

  // ---- Signup ----

  Future<void> _createAccount() async {
    final name = _nameCtrl.text.trim();
    final phone = _phoneCtrl.text.trim();
    final email = _signupEmailCtrl.text.trim();
    final password = _signupPasswordCtrl.text;

    if (name.isEmpty || phone.isEmpty || email.isEmpty || password.isEmpty) {
      setState(() => _signupError = 'All fields are required.');
      return;
    }
    if (!_isValidEmail(email)) {
      setState(() => _signupError = 'Please enter a valid email address.');
      return;
    }
    if (!_isValidPhone(phone)) {
      setState(() => _signupError = 'Phone number must be 7–15 digits.');
      return;
    }
    if (password.length < 6) {
      setState(() => _signupError = 'Password must be at least 6 characters.');
      return;
    }
    final mode = _pendingMode;
    if (mode == null) {
      setState(() => _signupError = 'Please choose a mode first.');
      return;
    }
    setState(() => _signupError = null);

    try {
      await ref.read(authProvider.notifier).register(
            name: name,
            email: email.trim().toLowerCase(),
            phone: phone,
            password: password,
            mode: mode,
          );
      if (!mounted) return;
      // Passed in-memory via route `extra` (never persisted to disk) so the
      // resend-OTP flow can re-authenticate without storing the plaintext
      // password anywhere.
      context.pushReplacement('/otp', extra: {
        'email': email.trim().toLowerCase(),
        'password': password,
        'mode': mode,
      });
    } catch (_) {
      if (!mounted) return;
      setState(() => _signupError =
          ref.read(authProvider).error ?? 'Registration failed. Please try again.');
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.transparent,
      body: AuthAnimatedBackground(
        child: SafeArea(
          child: SingleChildScrollView(
            padding: const EdgeInsets.symmetric(horizontal: 24),
            child: Column(
              children: [
                Align(
                  alignment: Alignment.centerLeft,
                  child: IconButton(
                    padding: EdgeInsets.zero,
                    constraints: const BoxConstraints(),
                    icon: const Icon(Icons.arrow_back_ios_new_rounded, size: 20),
                    color: const Color(0xFF1A1A1A),
                    onPressed: _goBackToChooser,
                  ),
                ),
                const SizedBox(height: 24),
                const AuthLogo(),
                const SizedBox(height: 40),
                AuthCard(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      AuthToggle(
                        selected: _isLogin ? 0 : 1,
                        onTap: (i) => i == 0 ? _switchToLogin() : _switchToSignup(),
                      ),
                      const SizedBox(height: 20),
                      AnimatedSwitcher(
                        duration: const Duration(milliseconds: 180),
                        child: _isLogin ? _loginForm() : _signupForm(),
                      ),
                    ],
                  ),
                ),
                const SizedBox(height: 32),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget _loginForm() {
    return Column(
      key: const ValueKey('login'),
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        const Text(
          'Welcome back',
          style: TextStyle(fontSize: 24, fontWeight: FontWeight.w700, color: Color(0xFF1A1A1A)),
        ),
        const SizedBox(height: 4),
        const Text(
          'Sign in to continue learning',
          style: TextStyle(fontSize: 13, color: Color(0xFF888888)),
        ),
        const SizedBox(height: 24),
        AuthField(
          controller: _loginEmailCtrl,
          hint: 'Email address',
          icon: Icons.mail_outline_rounded,
          keyboardType: TextInputType.emailAddress,
        ),
        const SizedBox(height: 12),
        ValueListenableBuilder<bool>(
          valueListenable: _obscureLogin,
          builder: (_, obscure, __) => AuthField(
            controller: _loginPasswordCtrl,
            hint: 'Password',
            icon: Icons.lock_outline_rounded,
            obscure: obscure,
            onToggleObscure: () => _obscureLogin.value = !_obscureLogin.value,
            onSubmitted: (_) => _continue(),
          ),
        ),
        Align(
          alignment: Alignment.centerRight,
          child: TextButton(
            onPressed: () => context.push('/forgot-password'),
            style: TextButton.styleFrom(
              padding: const EdgeInsets.symmetric(vertical: 4),
              minimumSize: Size.zero,
              tapTargetSize: MaterialTapTargetSize.shrinkWrap,
            ),
            child: const Text(
              'Forgot Password?',
              style: TextStyle(color: kPrimary, fontSize: 13, fontWeight: FontWeight.w500),
            ),
          ),
        ),
        const SizedBox(height: 8),
        ValueListenableBuilder<bool>(
          valueListenable: _loading,
          builder: (_, loading, __) => GoldButton(
            label: 'Continue',
            loading: loading,
            onTap: loading ? null : _continue,
          ),
        ),
        const SizedBox(height: 16),
        const AuthOrDivider(),
        const SizedBox(height: 16),
        ValueListenableBuilder<bool>(
          valueListenable: _googleLoading,
          builder: (_, googleLoading, __) => GoogleSignInButton(
            loading: googleLoading,
            onTap: googleLoading ? null : _continueWithGoogle,
          ),
        ),
        const SizedBox(height: 24),
        Center(
          child: GestureDetector(
            onTap: _switchToSignup,
            child: RichText(
              text: const TextSpan(
                text: 'New here? ',
                style: TextStyle(color: Color(0xFF888888), fontSize: 13),
                children: [
                  TextSpan(
                    text: 'Sign up',
                    style: TextStyle(color: kPrimary, fontWeight: FontWeight.w600),
                  ),
                ],
              ),
            ),
          ),
        ),
      ],
    );
  }

  Widget _signupForm() {
    final isLoading = ref.watch(authProvider).isLoading;
    final password = _signupPasswordCtrl.text;
    final strength = _strength(password);

    return Column(
      key: const ValueKey('signup'),
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        const Text(
          'Create account',
          style: TextStyle(fontSize: 24, fontWeight: FontWeight.w700, color: Color(0xFF1A1A1A)),
        ),
        const SizedBox(height: 4),
        const Text(
          'Start your learning journey today',
          style: TextStyle(fontSize: 13, color: Color(0xFF888888)),
        ),
        const SizedBox(height: 24),
        AuthField(controller: _nameCtrl, hint: 'Full name', icon: Icons.person_outline_rounded),
        const SizedBox(height: 12),
        AuthField(
          controller: _phoneCtrl,
          hint: 'Phone number',
          icon: Icons.phone_outlined,
          keyboardType: TextInputType.phone,
        ),
        const SizedBox(height: 12),
        AuthField(
          controller: _signupEmailCtrl,
          hint: 'Email address',
          icon: Icons.mail_outline_rounded,
          keyboardType: TextInputType.emailAddress,
        ),
        const SizedBox(height: 12),
        AuthField(
          controller: _signupPasswordCtrl,
          hint: 'Password',
          icon: Icons.lock_outline_rounded,
          obscure: _obscureSignup,
          onToggleObscure: () => setState(() => _obscureSignup = !_obscureSignup),
          onSubmitted: (_) => _createAccount(),
        ),
        if (password.isNotEmpty) ...[
          const SizedBox(height: 10),
          Row(
            children: List.generate(
              3,
              (i) => Expanded(
                child: Container(
                  margin: EdgeInsets.only(right: i < 2 ? 4 : 0),
                  height: 3,
                  decoration: BoxDecoration(
                    borderRadius: BorderRadius.circular(2),
                    color: i < strength ? _strengthColor(strength) : const Color(0xFFEEEEEE),
                  ),
                ),
              ),
            ),
          ),
          const SizedBox(height: 4),
          Align(
            alignment: Alignment.centerRight,
            child: Text(
              _strengthLabel(strength),
              style: TextStyle(
                fontSize: 11,
                color: _strengthColor(strength),
                fontWeight: FontWeight.w500,
              ),
            ),
          ),
        ],
        if (_signupError != null) ...[
          const SizedBox(height: 10),
          Text(_signupError!, style: const TextStyle(color: kDanger, fontSize: 12)),
        ],
        const SizedBox(height: 24),
        GoldButton(
          label: 'Create Account',
          loading: isLoading,
          onTap: isLoading ? null : _createAccount,
        ),
        const SizedBox(height: 24),
        Center(
          child: GestureDetector(
            onTap: _switchToLogin,
            child: RichText(
              text: const TextSpan(
                text: 'Already have an account? ',
                style: TextStyle(color: Color(0xFF888888), fontSize: 13),
                children: [
                  TextSpan(
                    text: 'Sign in',
                    style: TextStyle(color: kPrimary, fontWeight: FontWeight.w600),
                  ),
                ],
              ),
            ),
          ),
        ),
      ],
    );
  }
}
