/// Google Sign-In config. The Web client ID must match the backend
/// `GOOGLE_WEB_CLIENT_ID` / ID-token `aud` claim.
///
/// No default is provided on purpose — pass your own at build time:
///   flutter run --dart-define=GOOGLE_WEB_CLIENT_ID=xxxx.apps.googleusercontent.com
/// Get the value from Google Cloud Console → Credentials → OAuth 2.0 Client IDs
/// (Web application). Leave it empty to disable Google Sign-In.
class GoogleAuthConfig {
  static const String webClientId = String.fromEnvironment(
    'GOOGLE_WEB_CLIENT_ID',
  );

  /// Whether Google Sign-In is configured for this build.
  static bool get isConfigured => webClientId.isNotEmpty;
}
