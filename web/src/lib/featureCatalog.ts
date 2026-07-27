/**
 * Single source of truth for content-category / plan-feature keys.
 *
 * A key is an opaque string shared with the backend: video categories, photo
 * categories, community post categories, and `Plan.features` all draw from this
 * set. A user can watch a video when the video's category key appears in their
 * subscription's feature list.
 *
 * To rebrand: change the keys and labels below to match your own taxonomy, then
 * update `models.VideoCategory` in the Go API (api/internal/models/video.go) and
 * `FeatureCatalog` in the Flutter app (app/lib/core/config/feature_catalog.dart)
 * so all three agree. Nothing else in the admin panel hardcodes these values.
 */

export const CONTENT_CATEGORIES = [
  "CATEGORY_A",
  "CATEGORY_B",
  "CATEGORY_C",
] as const;

export type ContentCategory = (typeof CONTENT_CATEGORIES)[number];

/** Content categories plus the general bucket, which needs no entitlement. */
export const POST_CATEGORIES = ["GENERAL", ...CONTENT_CATEGORIES] as const;

const LABELS: Record<string, string> = {
  GENERAL: "General",
  CATEGORY_A: "Category A",
  CATEGORY_B: "Category B",
  CATEGORY_C: "Category C",
};

/**
 * Human-readable name for a key. Falls back to the raw key so a category added
 * on the backend still renders sensibly before the panel catches up.
 */
export const categoryLabel = (key: string): string => LABELS[key] ?? key;

/** Badge variant per category, for list and detail views. */
const BADGE_VARIANTS: Record<string, "brand" | "info" | "purple"> = {
  CATEGORY_A: "brand",
  CATEGORY_B: "info",
  CATEGORY_C: "purple",
};

export const categoryBadgeVariant = (key: string) =>
  BADGE_VARIANTS[key] ?? ("info" as const);

/** Tailwind text colour per category, used for icon accents. */
export const CATEGORY_TEXT_COLORS: Record<string, string> = {
  GENERAL: "text-muted-foreground",
  CATEGORY_A: "text-amber-500",
  CATEGORY_B: "text-sky-500",
  CATEGORY_C: "text-purple-500",
};

/** The category selected by default in create/upload forms. */
export const DEFAULT_CATEGORY: ContentCategory = CONTENT_CATEGORIES[0];
