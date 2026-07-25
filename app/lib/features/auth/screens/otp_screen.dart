import 'dart:async';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
// ignore_for_file: deprecated_member_use
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../core/services/app_mode_service.dart';
import '../../../core/theme/app_colors.dart';
import '../providers/auth_provider.dart';

class OtpScreen extends ConsumerStatefulWidget {
  final String email;
  // Held in memory only (passed via route `extra`, never persisted to disk)
  // so the resend-OTP flow can re-authenticate without storing the
  // plaintext password anywhere on the device.
  final String? password;
  // Mode chosen at the mode chooser before registration. Present only for
  // the new-signup flow — absent for a legacy login-only OTP flow.
  final String? mode;

  const OtpScreen({super.key, required this.email, this.password, this.mode});

  @override
  ConsumerState<OtpScreen> createState() => _OtpScreenState();
}

class _OtpScreenState extends ConsumerState<OtpScreen> {
  final List<TextEditingController> _ctrls =
      List.generate(6, (_) => TextEditingController());
  final List<FocusNode> _foci = List.generate(6, (_) => FocusNode());

  int _resendCooldown = 60;
  Timer? _timer;
  String? _error;

  @override
  void initState() {
    super.initState();
    _startTimer();
  }

  void _startTimer() {
    _timer?.cancel();
    setState(() => _resendCooldown = 60);
    _timer = Timer.periodic(const Duration(seconds: 1), (t) {
      if (_resendCooldown <= 1) {
        t.cancel();
        if (mounted) setState(() => _resendCooldown = 0);
      } else {
        if (mounted) setState(() => _resendCooldown--);
      }
    });
  }

  @override
  void dispose() {
    for (final c in _ctrls) {
      c.dispose();
    }
    for (final f in _foci) {
      f.dispose();
    }
    _timer?.cancel();
    super.dispose();
  }

  String get _otp => _ctrls.map((c) => c.text.trim()).join();

  void _scheduleVerifyIfComplete() {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted) return;
      final otp = _otp.replaceAll(RegExp(r'\D'), '');
      if (otp.length == 6) {
        _verify();
      }
    });
  }

  void _onDigitInput(int index, String val) {
    if (val.isEmpty) {
      if (index > 0) _foci[index - 1].requestFocus();
      return;
    }
    // Take only last character if pasted into a field
    if (val.length > 1) {
      // Autofill / paste — distribute across all fields
      final digits = val.replaceAll(RegExp(r'\D'), '');
      for (var i = 0; i < digits.length && i < 6; i++) {
        _ctrls[i].text = digits[i];
      }
      _foci[digits.length >= 6 ? 5 : digits.length - 1].requestFocus();
      setState(() {});
      _scheduleVerifyIfComplete();
      return;
    }
    if (index < 5) _foci[index + 1].requestFocus();
    _scheduleVerifyIfComplete();
  }

  Future<void> _verify() async {
    final otp = _otp.replaceAll(RegExp(r'\D'), '');
    if (otp.length < 6) {
      setState(() => _error = 'Enter all 6 digits.');
      return;
    }
    if (ref.read(authProvider).isLoading) return;
    setState(() => _error = null);

    try {
      await ref.read(authProvider.notifier).verifyOtp(
            email: widget.email.trim().toLowerCase(),
            otp: otp,
          );
      if (!mounted) return;
      final userId = ref.read(authProvider).user?.id;

      if (widget.mode != null && userId != null) {
        // New-signup flow: the account's mode was already set server-side at
        // registration — just seed the local cache and land there directly,
        // no detour through the mode chooser.
        await AppModeService.setMode(userId, widget.mode!);
        await AppModeService.clearPendingMode();
        if (!mounted) return;
        context.go(AppModeService.landingRoute(widget.mode));
        return;
      }

      // Login flow: resolve the account's mode (local → device → server) so
      // an existing account lands where it left off instead of the default.
      final user = ref.read(authProvider).user;
      final mode = user == null
          ? null
          : await AppModeService.resolveMode(user.id, user.currentAppMode);
      if (!mounted) return;
      context.go(AppModeService.landingRoute(mode));
    } on DioException catch (e) {
      if (!mounted) return;
      final msg = e.response?.data?['error'] as String?;
      if (e.response?.statusCode == 401) {
        setState(() => _error = msg ?? 'Invalid or expired OTP. Please try again.');
      } else {
        setState(() => _error = 'Connection error. Please check your internet.');
      }
      for (final c in _ctrls) {
        c.clear();
      }
      _foci[0].requestFocus();
    } catch (e) {
      if (!mounted) return;
      final msg = e is DioException
          ? null
          : (e is Exception ? e.toString().replaceFirst('Exception: ', '') : null);
      setState(() => _error = msg ?? 'Something went wrong. Please try again.');
      for (final c in _ctrls) {
        c.clear();
      }
      _foci[0].requestFocus();
    }
  }

  Future<void> _resend() async {
    if (_resendCooldown > 0) return;
    final password = widget.password;
    if (password == null || password.isEmpty) {
      setState(() => _error = 'Unable to resend OTP. Please return and retry.');
      return;
    }
    setState(() => _error = null);
    try {
      await ref.read(authProvider.notifier).sendOtp(
            email: widget.email.trim().toLowerCase(),
            password: password,
          );
      _startTimer();
    } on DioException catch (e) {
      if (!mounted) return;
      final msg = e.response?.statusCode == 401
          ? 'Session expired. Please go back and log in again.'
          : 'Failed to resend OTP. Check your connection.';
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text(msg)));
      _startTimer();
    } catch (_) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
            content: Text('Failed to resend OTP. Check your connection.')),
      );
      _startTimer();
    }
  }

  @override
  Widget build(BuildContext context) {
    final isLoading = ref.watch(authProvider).isLoading;

    return Scaffold(
      backgroundColor: kBackground,
      appBar: AppBar(
        backgroundColor: Colors.transparent,
        elevation: 0,
        leading: BackButton(color: kForeground),
        title: const Text(
          'Verify Email',
          style: TextStyle(color: kForeground, fontWeight: FontWeight.w600),
        ),
      ),
      body: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const SizedBox(height: 16),
            const Text(
              'Enter verification code',
              style: TextStyle(
                fontSize: 22,
                fontWeight: FontWeight.w700,
                color: kForeground,
              ),
            ),
            const SizedBox(height: 8),
            Text(
              'We sent a 6-digit OTP to ${widget.email}',
              style: const TextStyle(fontSize: 14, color: kMutedForeground),
            ),
            const SizedBox(height: 40),

            // OTP boxes
            Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: List.generate(6, (i) => _OtpBox(
                controller: _ctrls[i],
                focusNode: _foci[i],
                onChanged: (val) => _onDigitInput(i, val),
                onBackspace: () {
                  if (_ctrls[i].text.isEmpty && i > 0) {
                    _ctrls[i - 1].clear();
                    _foci[i - 1].requestFocus();
                  }
                },
              )),
            ),

            if (_error != null) ...[
              const SizedBox(height: 16),
              Text(
                _error!,
                style: const TextStyle(color: kTerracotta, fontSize: 13),
              ),
            ],

            const SizedBox(height: 40),

            // Verify button
            SizedBox(
              width: double.infinity,
              height: 52,
              child: ElevatedButton(
                onPressed: isLoading ? null : _verify,
                child: isLoading
                    ? const SizedBox(
                        width: 22,
                        height: 22,
                        child: CircularProgressIndicator(color: Colors.white, strokeWidth: 2.5),
                      )
                    : const Text('Verify'),
              ),
            ),

            const SizedBox(height: 24),

            // Resend
            Row(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                const Text("Didn't receive it? ",
                    style: TextStyle(color: kMutedForeground, fontSize: 14)),
                GestureDetector(
                  onTap: _resendCooldown == 0 ? _resend : null,
                  child: Text(
                    _resendCooldown > 0
                        ? 'Resend in ${_resendCooldown}s'
                        : 'Resend OTP',
                    style: TextStyle(
                      color: _resendCooldown == 0 ? kGold : kMutedForeground,
                      fontWeight: FontWeight.w600,
                      fontSize: 14,
                    ),
                  ),
                ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}

class _OtpBox extends StatelessWidget {
  final TextEditingController controller;
  final FocusNode focusNode;
  final ValueChanged<String> onChanged;
  final VoidCallback onBackspace;

  const _OtpBox({
    required this.controller,
    required this.focusNode,
    required this.onChanged,
    required this.onBackspace,
  });

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      width: 46,
      height: 56,
      child: RawKeyboardListener(
        focusNode: FocusNode(),
        onKey: (event) {
          if (event is RawKeyDownEvent &&
              event.logicalKey == LogicalKeyboardKey.backspace &&
              controller.text.isEmpty) {
            onBackspace();
          }
        },
        child: TextField(
          controller: controller,
          focusNode: focusNode,
          textAlign: TextAlign.center,
          keyboardType: TextInputType.number,
          maxLength: 1,
          inputFormatters: [FilteringTextInputFormatter.digitsOnly],
          style: const TextStyle(
            fontSize: 22,
            fontWeight: FontWeight.w700,
            color: kForeground,
          ),
          decoration: InputDecoration(
            counterText: '',
            contentPadding: EdgeInsets.zero,
            border: OutlineInputBorder(
              borderRadius: BorderRadius.circular(10),
              borderSide: const BorderSide(color: kBorder),
            ),
            enabledBorder: OutlineInputBorder(
              borderRadius: BorderRadius.circular(10),
              borderSide: const BorderSide(color: kBorder),
            ),
            focusedBorder: OutlineInputBorder(
              borderRadius: BorderRadius.circular(10),
              borderSide: const BorderSide(color: kGold, width: 2),
            ),
            filled: true,
            fillColor: kCard,
          ),
          onChanged: onChanged,
        ),
      ),
    );
  }
}
