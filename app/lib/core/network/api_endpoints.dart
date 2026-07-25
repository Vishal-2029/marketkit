class ApiEndpoints {
  // Set your API host at build/run time — this is the one value you must
  // provide to run the app against a backend:
  //
  //   Android emulator : flutter run --dart-define=API_BASE_URL=http://10.0.2.2:3000/api/v1
  //   iOS simulator    : flutter run --dart-define=API_BASE_URL=http://localhost:3000/api/v1
  //   Physical device  : flutter run --dart-define=API_BASE_URL=http://<your-lan-ip>:3000/api/v1
  //                      (find it with `ip -4 addr` on Linux, `ipconfig` on Windows)
  //   Production       : flutter build apk --dart-define=API_BASE_URL=https://api.example.com/api/v1
  //
  // Without the flag it falls back to localhost, which works for Flutter web
  // and the iOS simulator but NOT for the Android emulator or a real device.
  static final String baseUrl =
      const String.fromEnvironment('API_BASE_URL').isNotEmpty
      ? const String.fromEnvironment('API_BASE_URL')
      : 'http://localhost:3000/api/v1';

  // Static file base (same host, without /api/v1)
  static String get staticBase {
    final uri = Uri.parse(baseUrl);
    final isDefaultPort =
        (uri.scheme == 'https' && uri.port == 443) ||
        (uri.scheme == 'http' && uri.port == 80);
    if (isDefaultPort) return '${uri.scheme}://${uri.host}';
    return '${uri.scheme}://${uri.host}:${uri.port}';
  }

  // User auth
  static const String register = '/user/auth/register';
  static const String sendOtp = '/user/auth/send-otp';
  static const String verifyOtp = '/user/auth/verify-otp';
  static const String googleLogin = '/user/auth/google';
  static const String refresh = '/user/auth/refresh';
  static const String logout = '/user/auth/logout';
  static const String me = '/user/auth/me';
  static const String avatarUpload = '/user/auth/avatar';

  // Videos
  static const String videos = '/user/videos';
  static const String introVideo = '/user/videos/intro';
  static const String latestVideos = '/user/videos/latest';
  static String videoStream(String id) => '/user/videos/$id/stream';
  static String videoStreamUrl(String id) => '/user/videos/$id/stream-url';
  static String videoDownloadUrl(String id, String quality) =>
      '/user/videos/$id/download-url?quality=$quality';
  static String videoQualities(String id) => '/user/videos/$id/qualities';
  static String videoPhotos(String id) => '/user/videos/$id/photos';

  // Notifications
  static const String notifications = '/user/notifications';
  static const String notificationsMarkRead = '/user/notifications/read';
  static const String notificationsClear = '/user/notifications';

  // Plans (public)
  static const String plans = '/plans/public';

  // Photos (public)
  static const String photosPublic = '/photos/public';

  // User payments
  static const String userPaymentOrder = '/user/payments/order';
  static const String userPaymentVerify = '/user/payments/verify';

  // Password reset
  static const String forgotPassword = '/user/auth/forgot-password';
  static const String resetPassword = '/user/auth/reset-password';

  // Profile
  static const String updateProfile = '/user/auth/me';
  static const String deleteAccount = '/user/auth/me';
  static const String changePassword = '/user/auth/change-password';
  static const String setAppMode = '/user/auth/me/app-mode';

  // Push notifications
  static const String deviceToken = '/user/auth/device-token';

  // App version check (public — no auth)
  static const String appVersion = '/app/version';

  // Video progress
  static String videoProgress(String id) => '/user/videos/$id/progress';

  // Playlists
  static const String playlists = '/user/playlists';
  static String playlistDetail(String id) => '/user/playlists/$id';

  // Community
  static const String communityPosts = '/user/community/posts';
  static String communityPost(String id) => '/user/community/posts/$id';
  static String communityReplies(String id) =>
      '/user/community/posts/$id/replies';

  // Wallet
  static const String walletSummary = '/user/wallet/';
  static const String walletTransactions = '/user/wallet/transactions';
  static const String walletTopupOrder = '/user/wallet/topup/order';
  static const String walletTopupVerify = '/user/wallet/topup/verify';
  static const String walletPayoutDetails = '/user/wallet/payout-details';
  static const String walletWithdrawals = '/user/wallet/withdrawals';

  // Product market
  static const String marketFee = '/user/market/fee';
  static const String marketCategories = '/user/market/categories';
  static const String marketWalletPurchase = '/user/market/purchases/wallet';
  static const String marketProducts = '/user/market/products';
  static String marketProduct(String id) => '/user/market/products/$id';
  static String marketDownloadUrl(String id) =>
      '/user/market/products/$id/download-url';
  static const String marketMyProducts = '/user/market/my/products';
  static String marketMyProductStats(String id) =>
      '/user/market/my/products/$id/stats';
  static const String marketMyPurchases = '/user/market/my/purchases';
  static String marketInvoice(String purchaseId) =>
      '/user/market/purchases/$purchaseId/invoice';
  static String marketProductMessages(String productId) =>
      '/user/market/products/$productId/messages';
  static const String marketEarnings = '/user/market/my/earnings';
  static const String marketOrder = '/user/market/purchases/order';
  static const String marketVerify = '/user/market/purchases/verify';

  // Product Market Plans (separate from learning /plans)
  static const String marketPlans = '/user/market/plans';
  static const String marketPlansMy = '/user/market/plans/my';
  static String marketPlanOrder(String id) => '/user/market/plans/$id/order';
  static String marketPlanWallet(String id) => '/user/market/plans/$id/wallet';
  static const String marketPlanVerify = '/user/market/plans/verify';
  static const String marketPlanCancel = '/user/market/plans/my';

  // Video engagement
  static String videoReactions(String id) => '/user/videos/$id/reactions';
  static String videoComments(String id) => '/user/videos/$id/comments';
  static String videoComment(String videoId, String commentId) =>
      '/user/videos/$videoId/comments/$commentId';
}
