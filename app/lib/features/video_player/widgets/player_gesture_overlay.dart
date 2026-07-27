import 'dart:async';

import 'package:flutter/material.dart';
import 'package:screen_brightness/screen_brightness.dart';

import '../../../core/theme/app_colors.dart';

/// A transparent gesture layer that sits above the video player. It adds
/// double-tap seek (±10s on the right/left half) and vertical-drag
/// brightness/volume control without ever delaying or stealing single taps.
///
/// Double taps are detected manually from raw pointer events (a [Listener]),
/// NOT with a GestureDetector double-tap recognizer: a registered double-tap
/// recognizer makes every single tap underneath wait ~300ms for the
/// double-tap deadline, which made the player controls and buttons feel like
/// they needed two clicks. A Listener never joins the gesture arena, so taps
/// pass through to the controls instantly.
///
/// Seeking only triggers while the player controls are hidden (checked via
/// [areControlsVisible] at the first tap of a sequence) — when controls are
/// visible the on-screen ±10s buttons are available, and rapid presses of
/// those buttons must not also trigger a gesture seek.
///
/// It is intentionally decoupled from the player package: the parent passes in
/// callbacks to read/seek position and set volume, so this widget never imports
/// better_player and stays reusable/testable on its own.
class PlayerGestureOverlay extends StatefulWidget {
  const PlayerGestureOverlay({
    super.key,
    required this.onSeekBy,
    required this.currentVolume,
    required this.onSetVolume,
    this.areControlsVisible,
    this.enabled = true,
  });

  /// Seek relative to the current position by [offset] (positive = forward).
  final void Function(Duration offset) onSeekBy;

  /// Reads the player's current volume (0.0–1.0) at drag start.
  final double Function() currentVolume;

  /// Sets the player's volume (0.0–1.0).
  final void Function(double volume) onSetVolume;

  /// Reads whether the player controls are currently shown. When they are,
  /// double-tap seek is suppressed (null = treat as always hidden).
  final bool Function()? areControlsVisible;

  /// When false (e.g. while loading or errored) all gestures are ignored.
  final bool enabled;

  @override
  State<PlayerGestureOverlay> createState() => _PlayerGestureOverlayState();
}

class _PlayerGestureOverlayState extends State<PlayerGestureOverlay> {
  static const _seekStep = Duration(seconds: 10);

  // Manual tap tracking (see class comment for why this isn't a
  // GestureDetector double-tap).
  static const _doubleTapWindow = Duration(milliseconds: 300);
  static const _maxTapDuration = Duration(milliseconds: 250);
  static const _tapMoveSlop = 24.0;
  static const _doubleTapSlop = 80.0;

  int? _activePointer;
  Offset? _downPosition;
  DateTime? _downTime;
  DateTime? _lastTapTime; // completed-tap (pointer up) timestamp
  Offset? _lastTapPosition;
  // Controls visibility captured at the first tap of a tap sequence — a
  // double tap's own first tap shows the controls, so checking at the second
  // tap would always see them visible and never seek.
  bool _sequenceStartedWithControlsVisible = false;

  // Double-tap seek ripple state.
  bool _showSeekIndicator = false;
  bool _seekForward = true;
  Timer? _seekIndicatorTimer;

  // Vertical-drag (brightness/volume) state.
  bool _draggingLeft = false; // true = brightness, false = volume
  double _dragValue = 0; // live value during the drag
  bool _showVerticalIndicator = false;
  Timer? _verticalIndicatorTimer;

  @override
  void dispose() {
    _seekIndicatorTimer?.cancel();
    _verticalIndicatorTimer?.cancel();
    super.dispose();
  }

  void _handlePointerDown(PointerDownEvent event) {
    if (!widget.enabled || _activePointer != null) {
      // Second simultaneous finger — abandon tap tracking for this touch.
      _downPosition = null;
      return;
    }
    _activePointer = event.pointer;
    _downPosition = event.localPosition;
    _downTime = DateTime.now();
  }

  void _handlePointerUp(PointerUpEvent event, double width) {
    if (event.pointer != _activePointer) return;
    _activePointer = null;

    final downPosition = _downPosition;
    final downTime = _downTime;
    _downPosition = null;
    _downTime = null;
    if (!widget.enabled || downPosition == null || downTime == null) return;

    final now = DateTime.now();
    final moved = (event.localPosition - downPosition).distance;
    if (moved > _tapMoveSlop || now.difference(downTime) > _maxTapDuration) {
      _lastTapTime = null; // a drag/long press breaks any tap sequence
      return;
    }

    final continuesSequence = _lastTapTime != null &&
        _lastTapPosition != null &&
        now.difference(_lastTapTime!) < _doubleTapWindow &&
        (event.localPosition - _lastTapPosition!).distance < _doubleTapSlop;

    if (!continuesSequence) {
      _sequenceStartedWithControlsVisible =
          widget.areControlsVisible?.call() ?? false;
    } else if (!_sequenceStartedWithControlsVisible) {
      // Second (or third, fourth… — repeated taps keep seeking, like
      // YouTube) tap of a sequence that began with controls hidden: seek.
      _seek(event.localPosition.dx > width / 2);
    }

    _lastTapTime = now;
    _lastTapPosition = event.localPosition;
  }

  void _handlePointerCancel(PointerCancelEvent event) {
    if (event.pointer != _activePointer) return;
    _activePointer = null;
    _downPosition = null;
    _downTime = null;
  }

  void _seek(bool forward) {
    widget.onSeekBy(forward ? _seekStep : -_seekStep);
    setState(() {
      _seekForward = forward;
      _showSeekIndicator = true;
    });
    _seekIndicatorTimer?.cancel();
    _seekIndicatorTimer = Timer(const Duration(milliseconds: 700), () {
      if (mounted) setState(() => _showSeekIndicator = false);
    });
  }

  Future<void> _handleVerticalDragStart(
      DragStartDetails details, double width) async {
    if (!widget.enabled) return;
    _draggingLeft = details.localPosition.dx < width / 2;
    if (_draggingLeft) {
      double start;
      try {
        start = await ScreenBrightness().application;
      } catch (_) {
        start = 0.5;
      }
      _dragValue = start;
    } else {
      _dragValue = widget.currentVolume().clamp(0.0, 1.0);
    }
    setState(() => _showVerticalIndicator = true);
    _verticalIndicatorTimer?.cancel();
  }

  void _handleVerticalDragUpdate(DragUpdateDetails details, double height) {
    if (!widget.enabled || height <= 0) return;
    // Full-height swipe = full 0→1 range; dragging up increases the value.
    final delta = -details.primaryDelta! / height;
    _dragValue = (_dragValue + delta).clamp(0.0, 1.0);
    if (_draggingLeft) {
      ScreenBrightness().setApplicationScreenBrightness(_dragValue);
    } else {
      widget.onSetVolume(_dragValue);
    }
    setState(() {});
  }

  void _handleVerticalDragEnd(DragEndDetails details) {
    _verticalIndicatorTimer?.cancel();
    _verticalIndicatorTimer = Timer(const Duration(milliseconds: 600), () {
      if (mounted) setState(() => _showVerticalIndicator = false);
    });
  }

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(
      builder: (context, constraints) {
        final width = constraints.maxWidth;
        final height = constraints.maxHeight;
        return Stack(
          children: [
            // Raw pointer observation for double-tap seek (never claims the
            // events) + a drag-only GestureDetector for brightness/volume.
            // Neither delays taps headed for the player controls beneath.
            Positioned.fill(
              child: Listener(
                behavior: HitTestBehavior.translucent,
                onPointerDown: _handlePointerDown,
                onPointerUp: (e) => _handlePointerUp(e, width),
                onPointerCancel: _handlePointerCancel,
                child: GestureDetector(
                  behavior: HitTestBehavior.translucent,
                  onVerticalDragStart: (d) =>
                      _handleVerticalDragStart(d, width),
                  onVerticalDragUpdate: (d) =>
                      _handleVerticalDragUpdate(d, height),
                  onVerticalDragEnd: _handleVerticalDragEnd,
                ),
              ),
            ),
            if (_showSeekIndicator)
              Align(
                alignment:
                    _seekForward ? Alignment.centerRight : Alignment.centerLeft,
                child: FractionallySizedBox(
                  widthFactor: 0.5,
                  child: _SeekIndicator(forward: _seekForward),
                ),
              ),
            if (_showVerticalIndicator)
              Align(
                alignment: Alignment.center,
                child: _VerticalIndicator(
                  isBrightness: _draggingLeft,
                  value: _dragValue,
                ),
              ),
          ],
        );
      },
    );
  }
}

class _SeekIndicator extends StatelessWidget {
  const _SeekIndicator({required this.forward});

  final bool forward;

  @override
  Widget build(BuildContext context) {
    return IgnorePointer(
      child: Container(
        color: Colors.black26,
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(
              forward ? Icons.forward_10_rounded : Icons.replay_10_rounded,
              color: Colors.white,
              size: 40,
            ),
            const SizedBox(height: 6),
            Text(
              forward ? '+10s' : '-10s',
              style: const TextStyle(
                color: Colors.white,
                fontSize: 13,
                fontWeight: FontWeight.w700,
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _VerticalIndicator extends StatelessWidget {
  const _VerticalIndicator({required this.isBrightness, required this.value});

  final bool isBrightness;
  final double value;

  @override
  Widget build(BuildContext context) {
    return IgnorePointer(
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 14),
        decoration: BoxDecoration(
          color: Colors.black.withValues(alpha: 0.6),
          borderRadius: BorderRadius.circular(12),
        ),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              isBrightness
                  ? (value > 0.5
                      ? Icons.brightness_high_rounded
                      : Icons.brightness_low_rounded)
                  : (value == 0
                      ? Icons.volume_off_rounded
                      : value > 0.5
                          ? Icons.volume_up_rounded
                          : Icons.volume_down_rounded),
              color: Colors.white,
              size: 26,
            ),
            const SizedBox(height: 10),
            SizedBox(
              width: 6,
              height: 90,
              child: RotatedBox(
                quarterTurns: 3,
                child: LinearProgressIndicator(
                  value: value,
                  backgroundColor: Colors.white24,
                  valueColor: const AlwaysStoppedAnimation(kPrimary),
                ),
              ),
            ),
            const SizedBox(height: 8),
            Text(
              '${(value * 100).round()}%',
              style: const TextStyle(
                color: Colors.white,
                fontSize: 11,
                fontWeight: FontWeight.w600,
              ),
            ),
          ],
        ),
      ),
    );
  }
}
