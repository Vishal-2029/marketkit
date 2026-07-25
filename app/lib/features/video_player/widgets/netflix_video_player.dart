import 'dart:async';

import 'package:better_player_plus/better_player_plus.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:screen_brightness/screen_brightness.dart';

import '../../../core/theme/app_colors.dart';
import 'player_gesture_overlay.dart';
import 'snappy_player_controls.dart';

/// A reusable, Netflix-style video player built on better_player_plus.
///
/// Playback sources are passed in already-resolved (the host app resolves
/// signed, entitlement-checked URLs server-side by [videoId] and hands them
/// here) — this widget never talks to the backend itself, so it stays a
/// self-contained, reusable component.
///
/// [SnappyPlayerControls] (a patched copy of better_player's material
/// controls that responds to single taps instantly) provides the dark
/// auto-hiding control bar, quality menu (from [mp4Resolutions] + Auto),
/// playback-speed menu, fullscreen with orientation handling and a visible
/// brightness button/slider, ±10s skip buttons, buffered seek bar, time
/// labels, and the loading spinner. On top
/// of that this widget adds a [PlayerGestureOverlay] (double-tap seek,
/// brightness/volume swipes — also present in fullscreen via
/// routePageBuilder), a resume banner, and an error/retry surface.
class NetflixVideoPlayer extends StatefulWidget {
  const NetflixVideoPlayer({
    super.key,
    required this.title,
    this.hlsUrl,
    this.mp4Resolutions,
    this.videoId,
    this.startPositionSeconds = 0,
    this.onProgress,
    this.autoPlay = true,
  });

  final String title;

  /// Adaptive (HLS) source used for the "Auto" quality option.
  final String? hlsUrl;

  /// Selectable quality label → URL. In this app these are signed HLS-variant
  /// playlist URLs (1080p/720p/360p); the key is what shows in the quality
  /// menu. Ignored when playing a single offline file.
  final Map<String, String>? mp4Resolutions;

  final String? videoId;

  /// Position to resume from; when > 0 a "resume / start over" banner shows.
  final int startPositionSeconds;

  /// Called ~every 15s and on pause/dispose with the current position, so the
  /// host can persist progress to its backend.
  final ValueChanged<int>? onProgress;

  final bool autoPlay;

  @override
  State<NetflixVideoPlayer> createState() => _NetflixVideoPlayerState();
}

class _NetflixVideoPlayerState extends State<NetflixVideoPlayer> {
  BetterPlayerController? _controller;
  bool _hasError = false;
  bool _showResumeBanner = false;
  // Live controls visibility, reported synchronously by SnappyPlayerControls.
  // The gesture overlay uses it to suppress double-tap seek while the
  // on-screen buttons are shown.
  bool _controlsVisible = false;
  // Kept as a field so the customControlsBuilder closure (which runs after
  // _setup finishes) can hand the config to SnappyPlayerControls.
  BetterPlayerControlsConfiguration? _controlsConfig;
  Timer? _resumeBannerTimer;
  Timer? _progressTimer;

  @override
  void initState() {
    super.initState();
    _setup();
  }

  void _setup() {
    final resume = widget.startPositionSeconds > 5
        ? Duration(seconds: widget.startPositionSeconds)
        : null;

    final controlsConfig = BetterPlayerControlsConfiguration(
      controlBarColor: Colors.black.withValues(alpha: 0.55),
      iconsColor: Colors.white,
      textColor: Colors.white,
      loadingColor: kGold,
      progressBarPlayedColor: kGold,
      progressBarHandleColor: kGold,
      progressBarBufferedColor: Colors.white54,
      progressBarBackgroundColor: Colors.white24,
      enableProgressBar: true,
      enableProgressText: true,
      enablePlayPause: true,
      enableSkips: true, // ±10s replay_10 / forward_10 buttons
      forwardSkipTimeInMilliseconds: 10000,
      backwardSkipTimeInMilliseconds: 10000,
      enableFullscreen: true,
      enablePlaybackSpeed: true, // 0.5x–2x menu
      enableQualities: true, // driven by the resolutions map
      enableSubtitles: false,
      enableAudioTracks: false,
      enablePip: false,
      enableRetry: true,
      enableOverflowMenu: true,
      overflowMenuIconsColor: Colors.white,
      // NOTE: despite the name this is the show/hide FADE duration, not the
      // auto-hide delay (that's hardcoded to 3s in the controls). It was
      // Duration(seconds: 3), which made controls take 3s to appear.
      controlsHideTime: const Duration(milliseconds: 200),
      showControlsOnInitialize: true,
      // Patched copy of the material controls without the double-tap
      // recognizer that delayed every tap/button press by ~300ms.
      playerTheme: BetterPlayerTheme.custom,
      customControlsBuilder: (controller, onPlayerVisibilityChanged) =>
          SnappyPlayerControls(
        controlsConfiguration: _controlsConfig!,
        onControlsVisibilityChanged: onPlayerVisibilityChanged,
        onVisibilityChanged: (visible) => _controlsVisible = visible,
      ),
    );
    _controlsConfig = controlsConfig;

    final config = BetterPlayerConfiguration(
      aspectRatio: 16 / 9,
      fit: BoxFit.contain,
      autoPlay: widget.autoPlay,
      startAt: resume, // auto-resume; banner offers "start over"
      handleLifecycle: true, // pause when app is backgrounded
      autoDispose: false, // we own disposal
      allowedScreenSleep: false, // keep screen awake during playback
      // Fullscreen rotates to landscape and returns to portrait on exit.
      deviceOrientationsOnFullScreen: const [
        DeviceOrientation.landscapeLeft,
        DeviceOrientation.landscapeRight,
      ],
      deviceOrientationsAfterFullScreen: const [DeviceOrientation.portraitUp],
      systemOverlaysAfterFullScreen: SystemUiOverlay.values,
      controlsConfiguration: controlsConfig,
      placeholder: const ColoredBox(color: Colors.black),
      // Fullscreen pushes its own route containing only the player, so the
      // gesture overlay from build() isn't there — recreate the default
      // fullscreen page with the overlay stacked on top, otherwise
      // brightness/volume swipes and double-tap seek don't work fullscreen.
      routePageBuilder: (context, animation, secondaryAnimation,
              controllerProvider) =>
          AnimatedBuilder(
        animation: animation,
        builder: (context, child) => Scaffold(
          resizeToAvoidBottomInset: false,
          backgroundColor: Colors.black,
          body: Stack(
            fit: StackFit.expand,
            children: [
              Container(
                alignment: Alignment.center,
                color: Colors.black,
                child: controllerProvider,
              ),
              _buildGestureOverlay(),
            ],
          ),
        ),
      ),
    );

    final controller = BetterPlayerController(config);
    controller.addEventsListener(_onEvent);
    controller.setupDataSource(_buildDataSource());
    _controller = controller;

    if (resume != null) {
      _showResumeBanner = true;
      _resumeBannerTimer = Timer(const Duration(seconds: 6), () {
        if (mounted) setState(() => _showResumeBanner = false);
      });
    }

    // Periodically report position so the host can persist progress.
    _progressTimer = Timer.periodic(const Duration(seconds: 15), (_) {
      final pos = _controller?.videoPlayerController?.value.position;
      if ((_controller?.isPlaying() ?? false) && pos != null) {
        widget.onProgress?.call(pos.inSeconds);
      }
    });
  }

  BetterPlayerDataSource _buildDataSource() {
    final resolutions = <String, String>{
      if (widget.hlsUrl != null) 'Auto': widget.hlsUrl!,
      ...?widget.mp4Resolutions,
    };
    // Primary URL: the adaptive HLS source if present, else the first variant.
    final primary = widget.hlsUrl ??
        (widget.mp4Resolutions?.values.isNotEmpty ?? false
            ? widget.mp4Resolutions!.values.first
            : '');
    // Detect HLS from the resolved URL itself — hlsUrl is set for every
    // online video, but for videos that never got an HLS transcode the
    // server redirects to a plain MP4/WebM file; forcing the HLS demuxer on
    // those makes playback fail outright.
    final isHls = Uri.parse(primary).path.toLowerCase().endsWith('.m3u8');

    return BetterPlayerDataSource(
      BetterPlayerDataSourceType.network,
      primary,
      videoFormat:
          isHls ? BetterPlayerVideoFormat.hls : BetterPlayerVideoFormat.other,
      // Only offer the quality menu when there's more than one option.
      resolutions: resolutions.length > 1 ? resolutions : null,
      notificationConfiguration: BetterPlayerNotificationConfiguration(
        showNotification: false,
        title: widget.title,
      ),
    );
  }

  void _onEvent(BetterPlayerEvent event) {
    switch (event.betterPlayerEventType) {
      case BetterPlayerEventType.exception:
        if (mounted && !_hasError) setState(() => _hasError = true);
        break;
      case BetterPlayerEventType.pause:
      case BetterPlayerEventType.finished:
        final pos = _controller?.videoPlayerController?.value.position;
        if (pos != null) widget.onProgress?.call(pos.inSeconds);
        break;
      default:
        break;
    }
  }

  void _retry() {
    setState(() => _hasError = false);
    _controller?.retryDataSource();
  }

  void _startOver() {
    _controller?.seekTo(Duration.zero);
    setState(() => _showResumeBanner = false);
  }

  double _currentVolume() =>
      _controller?.videoPlayerController?.value.volume ?? 1.0;

  /// Gesture layer (double-tap seek + brightness/volume swipe), used both in
  /// the embedded player and inside the fullscreen route.
  Widget _buildGestureOverlay() {
    return PlayerGestureOverlay(
      onSeekBy: (offset) {
        final controller = _controller;
        if (controller == null) return;
        final pos =
            controller.videoPlayerController?.value.position ?? Duration.zero;
        final dur =
            controller.videoPlayerController?.value.duration ?? Duration.zero;
        var target = pos + offset;
        if (target < Duration.zero) target = Duration.zero;
        if (dur > Duration.zero && target > dur) target = dur;
        controller.seekTo(target);
      },
      currentVolume: _currentVolume,
      onSetVolume: (v) => _controller?.setVolume(v),
      areControlsVisible: () => _controlsVisible,
    );
  }

  @override
  void dispose() {
    _progressTimer?.cancel();
    _resumeBannerTimer?.cancel();
    final pos = _controller?.videoPlayerController?.value.position;
    if (pos != null) widget.onProgress?.call(pos.inSeconds);
    _controller?.removeEventsListener(_onEvent);
    _controller?.dispose();
    // Undo any in-player brightness override so the rest of the app returns
    // to the system brightness.
    ScreenBrightness().resetApplicationScreenBrightness().catchError((_) {});
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final controller = _controller;
    if (controller == null) {
      return const AspectRatio(
        aspectRatio: 16 / 9,
        child: ColoredBox(color: Colors.black),
      );
    }

    return AspectRatio(
      aspectRatio: 16 / 9,
      child: ColoredBox(
        color: Colors.black,
        child: Stack(
          fit: StackFit.expand,
          children: [
            BetterPlayer(controller: controller),

            if (!_hasError) _buildGestureOverlay(),

            if (_showResumeBanner) _ResumeBanner(onStartOver: _startOver),

            if (_hasError) _ErrorOverlay(onRetry: _retry),
          ],
        ),
      ),
    );
  }
}

class _ResumeBanner extends StatelessWidget {
  const _ResumeBanner({required this.onStartOver});

  final VoidCallback onStartOver;

  @override
  Widget build(BuildContext context) {
    return Positioned(
      left: 12,
      bottom: 12,
      child: Material(
        color: Colors.black.withValues(alpha: 0.75),
        borderRadius: BorderRadius.circular(8),
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              const Icon(Icons.history_rounded, color: Colors.white, size: 16),
              const SizedBox(width: 8),
              const Text('Resuming where you left off',
                  style: TextStyle(color: Colors.white, fontSize: 12)),
              const SizedBox(width: 12),
              GestureDetector(
                onTap: onStartOver,
                child: const Text(
                  'Start over',
                  style: TextStyle(
                    color: kGold,
                    fontSize: 12,
                    fontWeight: FontWeight.w700,
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _ErrorOverlay extends StatelessWidget {
  const _ErrorOverlay({required this.onRetry});

  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    return Container(
      color: Colors.black,
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          const Icon(Icons.error_outline_rounded,
              color: Colors.white54, size: 44),
          const SizedBox(height: 10),
          const Text(
            "Couldn't play this video.\nCheck your connection and try again.",
            textAlign: TextAlign.center,
            style: TextStyle(color: Colors.white70, fontSize: 13),
          ),
          const SizedBox(height: 14),
          OutlinedButton.icon(
            onPressed: onRetry,
            icon: const Icon(Icons.refresh_rounded, color: kGold, size: 18),
            label: const Text('Retry', style: TextStyle(color: kGold)),
            style: OutlinedButton.styleFrom(
              side: const BorderSide(color: kGold),
            ),
          ),
        ],
      ),
    );
  }
}
