import 'dart:math' as math;
import 'package:flutter/material.dart';
import 'package:marketkit/core/theme/app_colors.dart';

/// Shared cream animated background used across splash, onboarding,
/// mode chooser, login, and register so the pre-auth funnel shares one look.
class AuthAnimatedBackground extends StatefulWidget {
  final Widget child;
  const AuthAnimatedBackground({super.key, required this.child});

  @override
  State<AuthAnimatedBackground> createState() => _AuthAnimatedBackgroundState();
}

class _AuthAnimatedBackgroundState extends State<AuthAnimatedBackground>
    with SingleTickerProviderStateMixin {
  late final AnimationController _ctrl;

  @override
  void initState() {
    super.initState();
    _ctrl = AnimationController(
      vsync: this,
      duration: const Duration(seconds: 8),
    )..repeat();
  }

  @override
  void dispose() {
    _ctrl.dispose();
    super.dispose();
  }

  double _cycle(double offset) => (_ctrl.value + offset) % 1.0;

  Widget _square({
    required double left,
    required double top,
    required double size,
    required Color color,
    required double offset,
  }) {
    return AnimatedBuilder(
      animation: _ctrl,
      builder: (_, __) {
        final t = _cycle(offset);
        final scale = 0.9 + 0.18 * math.sin(2 * math.pi * t);
        final opacity = 0.12 + 0.12 * (0.5 + 0.5 * math.sin(2 * math.pi * t));
        final dy = 8 * math.sin(2 * math.pi * (t + 0.25));
        return Positioned(
          left: left,
          top: top + dy,
          child: Opacity(
            opacity: opacity,
            child: Transform.scale(
              scale: scale,
              child: Container(
                width: size,
                height: size,
                decoration: BoxDecoration(
                  borderRadius: BorderRadius.circular(size * 0.2),
                  border: Border.all(color: color, width: 2),
                ),
              ),
            ),
          ),
        );
      },
    );
  }

  Widget _dot({
    required double left,
    required double startTop,
    required double size,
    required Color color,
    required double offset,
  }) {
    return AnimatedBuilder(
      animation: _ctrl,
      builder: (_, __) {
        final t = _cycle(offset);
        final y = startTop - 60 * t;
        final opacity =
            (0.25 + 0.35 * (0.5 + 0.5 * math.sin(2 * math.pi * t)))
                .clamp(0.0, 1.0);
        final scale = 0.85 + 0.3 * (0.5 + 0.5 * math.sin(2 * math.pi * t));
        return Positioned(
          left: left,
          top: y,
          child: Opacity(
            opacity: opacity,
            child: Transform.scale(
              scale: scale,
              child: Container(
                width: size,
                height: size,
                decoration: BoxDecoration(color: color, shape: BoxShape.circle),
              ),
            ),
          ),
        );
      },
    );
  }

  @override
  Widget build(BuildContext context) {
    final h = MediaQuery.of(context).size.height;
    return Stack(
      children: [
        Container(color: kBackground),
        _square(
            left: 24,
            top: 120,
            size: 80,
            color: kPrimary,
            offset: 0.0),
        _square(
            left: 260,
            top: 180,
            size: 100,
            color: kAccentStrong,
            offset: 0.2),
        _square(
            left: 48,
            top: h * 0.65,
            size: 70,
            color: kPrimaryLight,
            offset: 0.4),
        _square(
            left: 280,
            top: h * 0.65,
            size: 90,
            color: kPrimaryLight,
            offset: 0.6),
        _dot(
            left: 120,
            startTop: h - 40,
            size: 5,
            color: kAccentStrong,
            offset: 0.0),
        _dot(
            left: 260,
            startTop: h - 60,
            size: 4,
            color: kPrimary,
            offset: 0.25),
        _dot(
            left: 340,
            startTop: h - 40,
            size: 5,
            color: kPrimaryLight,
            offset: 0.45),
        _dot(
            left: 80,
            startTop: h - 100,
            size: 4,
            color: kPrimary,
            offset: 0.7),
        _dot(
            left: 320,
            startTop: h - 100,
            size: 4,
            color: kAccentStrong,
            offset: 0.55),
        Positioned.fill(child: widget.child),
      ],
    );
  }
}
