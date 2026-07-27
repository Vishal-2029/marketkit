import 'dart:math' as math;
import 'package:flutter/material.dart';
import 'package:marketkit/core/theme/app_colors.dart';

class AuthBackground extends StatefulWidget {
  const AuthBackground({super.key, required this.child});

  final Widget child;

  @override
  State<AuthBackground> createState() => _AuthBackgroundState();
}

class _AuthBackgroundState extends State<AuthBackground>
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

  double _cycle(double offset) => (_ctrl.value + offset) % 1;

  Widget _animatedSquare({
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
        final transform = 0.9 + 0.18 * math.sin(2 * math.pi * t);
        final opacity = 0.15 + 0.15 * (0.5 + 0.5 * math.sin(2 * math.pi * t));
        final dy = 8 * math.sin(2 * math.pi * (t + 0.25));

        return Positioned(
          left: left,
          top: top + dy,
          child: Opacity(
            opacity: opacity,
            child: Transform.scale(
              scale: transform,
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

  Widget _floatingDot({
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
        final opacity = 0.25 + 0.35 * (0.5 + 0.5 * math.sin(2 * math.pi * t));
        final scale = 0.85 + 0.3 * (0.5 + 0.5 * math.sin(2 * math.pi * t));
        return Positioned(
          left: left,
          top: y,
          child: Opacity(
            opacity: opacity.clamp(0.0, 1.0),
            child: Transform.scale(
              scale: scale,
              child: Container(
                width: size,
                height: size,
                decoration: BoxDecoration(
                  color: color,
                  shape: BoxShape.circle,
                ),
              ),
            ),
          ),
        );
      },
    );
  }

  @override
  Widget build(BuildContext context) {
    return SizedBox.expand(
      child: Stack(
        children: [
          Container(color: Colors.transparent),
          _animatedSquare(
            left: 24,
            top: 120,
            size: 80,
            color: kPrimary,
            offset: 0.0,
          ),
          _animatedSquare(
            left: 260,
            top: 180,
            size: 100,
            color: kAccentStrong,
            offset: 0.2,
          ),
          _animatedSquare(
            left: 48,
            top: 520,
            size: 70,
            color: kPrimaryLight,
            offset: 0.4,
          ),
          _animatedSquare(
            left: 280,
            top: 520,
            size: 90,
            color: kPrimaryLight,
            offset: 0.6,
          ),
          _floatingDot(
            left: 120,
            startTop: 760,
            size: 5,
            color: kAccentStrong,
            offset: 0.0,
          ),
          _floatingDot(
            left: 260,
            startTop: 740,
            size: 4,
            color: kPrimary,
            offset: 0.25,
          ),
          _floatingDot(
            left: 340,
            startTop: 760,
            size: 5,
            color: kPrimaryLight,
            offset: 0.45,
          ),
          _floatingDot(
            left: 80,
            startTop: 700,
            size: 4,
            color: kPrimary,
            offset: 0.7,
          ),
          _floatingDot(
            left: 320,
            startTop: 700,
            size: 4,
            color: kAccentStrong,
            offset: 0.55,
          ),
          Positioned.fill(
            child: SafeArea(
              child: widget.child,
            ),
          ),
        ],
      ),
    );
  }
}

