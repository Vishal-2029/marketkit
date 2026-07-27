/// Single source of truth for content-category / plan-feature keys.
///
/// A key is an opaque string shared with the backend: video categories, photo
/// categories, community post categories, and `Plan.features` all draw from
/// this set. A user can watch a video when the video's category key appears in
/// their subscription's feature list.
///
/// To rebrand: change the labels in [_labels] and the keys in [contentKeys] to
/// match your own taxonomy, then update `models.VideoCategory` in the Go API
/// (api/internal/models/video.go) so both sides agree. Nothing else in the app
/// hardcodes these values.
class FeatureCatalog {
  const FeatureCatalog._();

  /// Category keys users can hold entitlements for, in display order.
  static const List<String> contentKeys = [
    'CATEGORY_A',
    'CATEGORY_B',
    'CATEGORY_C',
  ];

  /// Keys valid for photo galleries and community posts — the content
  /// categories plus a general bucket that needs no entitlement.
  static const List<String> generalKey = ['GENERAL'];
  static List<String> get postCategoryKeys => [...generalKey, ...contentKeys];

  static const Map<String, String> _labels = {
    'GENERAL': 'General',
    'CATEGORY_A': 'Category A',
    'CATEGORY_B': 'Category B',
    'CATEGORY_C': 'Category C',
  };

  /// Human-readable name for [key]. Falls back to the raw key so a category
  /// added on the backend still renders sensibly before the app catches up.
  static String label(String key) => _labels[key] ?? key;
}
