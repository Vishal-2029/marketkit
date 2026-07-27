/// Single place to rebrand the mobile app's user-facing identity.
///
/// Change these values and the app name in `pubspec.yaml`,
/// `android/app/src/main/AndroidManifest.xml` (`android:label`) and
/// `ios/Runner/Info.plist` (`CFBundleDisplayName`). Colours live in
/// `core/theme/app_colors.dart`; content categories in
/// `core/config/feature_catalog.dart`.
class Brand {
  const Brand._();

  /// Shown in the app bar, the onboarding carousel, and auth screens.
  static const String name = 'MarketKit';

  /// Merchant name shown inside the payment gateway's checkout sheet.
  /// Keep it recognisable — buyers see this at the moment they pay.
  static const String checkoutName = name;

  /// Support contact surfaced on the profile screen.
  static const String supportEmail = 'support@example.com';
  static const String privacyPolicyUrl = 'https://example.com/privacy';
}
