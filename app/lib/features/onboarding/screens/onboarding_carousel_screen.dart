import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:smooth_page_indicator/smooth_page_indicator.dart';
import '../../../core/services/app_mode_service.dart';
import '../../../core/theme/app_colors.dart';
import '../../../shared/widgets/auth_animated_background.dart';

class _OnboardingSlide {
  final IconData icon;
  final String title;
  final String subtitle;
  const _OnboardingSlide({required this.icon, required this.title, required this.subtitle});
}

// Placeholder icon-based slides — swap `icon` for real artwork/photos later.
const _slides = [
  _OnboardingSlide(
    icon: Icons.auto_awesome_rounded,
    title: 'Welcome to Design Express',
    subtitle: 'Everything embroidery — learning and selling, in one app.',
  ),
  _OnboardingSlide(
    icon: Icons.swap_horizontal_circle_rounded,
    title: 'Learn or sell — your choice',
    subtitle: 'Watch embroidery courses, or buy and sell designs in the marketplace.',
  ),
  _OnboardingSlide(
    icon: Icons.rocket_launch_rounded,
    title: 'Get started in seconds',
    subtitle: "Pick what brings you here, and we'll set things up for you.",
  ),
];

/// Shown once per device, before any account exists. Ends by sending the
/// user to the mode chooser (pre-registration context).
class OnboardingCarouselScreen extends StatefulWidget {
  const OnboardingCarouselScreen({super.key});

  @override
  State<OnboardingCarouselScreen> createState() => _OnboardingCarouselScreenState();
}

class _OnboardingCarouselScreenState extends State<OnboardingCarouselScreen> {
  final _controller = PageController();
  int _current = 0;

  Future<void> _finish() async {
    await AppModeService.setSeenOnboarding();
    if (!mounted) return;
    context.go('/choose-mode');
  }

  void _next() {
    if (_current == _slides.length - 1) {
      _finish();
      return;
    }
    _controller.nextPage(duration: const Duration(milliseconds: 350), curve: Curves.easeOutCubic);
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final isLast = _current == _slides.length - 1;
    return Scaffold(
      body: AuthAnimatedBackground(
        child: SafeArea(
          child: Column(
            children: [
              Align(
                alignment: Alignment.topRight,
                child: Padding(
                  padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
                  child: TextButton(
                    onPressed: _finish,
                    child: const Text('Skip', style: TextStyle(color: kMutedForeground, fontWeight: FontWeight.w600)),
                  ),
                ),
              ),
              Expanded(
                child: PageView.builder(
                  controller: _controller,
                  onPageChanged: (i) => setState(() => _current = i),
                  itemCount: _slides.length,
                  itemBuilder: (context, i) => _SlideView(slide: _slides[i]),
                ),
              ),
              SmoothPageIndicator(
                controller: _controller,
                count: _slides.length,
                effect: ExpandingDotsEffect(
                  activeDotColor: kGold,
                  dotColor: kBorder,
                  dotHeight: 8,
                  dotWidth: 8,
                  expansionFactor: 3,
                ),
              ),
            Padding(
              padding: const EdgeInsets.fromLTRB(24, 24, 24, 32),
              child: SizedBox(
                width: double.infinity,
                child: ElevatedButton(
                  style: ElevatedButton.styleFrom(
                    backgroundColor: kGold,
                    foregroundColor: Colors.white,
                    padding: const EdgeInsets.symmetric(vertical: 16),
                    shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(14)),
                  ),
                  onPressed: _next,
                  child: Text(
                    isLast ? 'Get Started' : 'Next',
                    style: const TextStyle(fontWeight: FontWeight.w700, fontSize: 15),
                  ),
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

class _SlideView extends StatelessWidget {
  final _OnboardingSlide slide;
  const _SlideView({required this.slide});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 32),
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Container(
            width: 160,
            height: 160,
            decoration: BoxDecoration(
              color: kGold.withValues(alpha: 0.12),
              shape: BoxShape.circle,
            ),
            child: Icon(slide.icon, size: 84, color: kGold),
          ),
          const SizedBox(height: 40),
          Text(
            slide.title,
            textAlign: TextAlign.center,
            style: const TextStyle(fontSize: 24, fontWeight: FontWeight.w700, color: kForeground),
          ),
          const SizedBox(height: 12),
          Text(
            slide.subtitle,
            textAlign: TextAlign.center,
            style: const TextStyle(fontSize: 14.5, color: kMutedForeground, height: 1.5),
          ),
        ],
      ),
    );
  }
}
