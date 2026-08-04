/**
 * Chart colours, in one place.
 *
 * Recharts needs concrete values (it measures and interpolates them), so these
 * cannot be plain `hsl(var(--primary))` references. They are the same palette as
 * `index.css` — if you change `--primary` there, change `SERIES[0]` here too.
 *
 * Rebranding: replace SERIES with your own ramp. Keep it ordered light-to-dark
 * within a hue so stacked bars and donut slices stay readable next to each other.
 */

/** Categorical ramp for donut slices and multi-series charts. */
export const SERIES = [
  "#4F46E5", // primary
  "#818CF8", // primary, lighter
  "#6366F1",
  "#3730A3", // primary, darker
  "#A5B4FC",
  "#312E81",
] as const;

/** Single-series charts (line, bar) use the brand colour. */
export const PRIMARY = SERIES[0];
/** Second series in a stacked chart. */
export const SECONDARY = SERIES[3];
/** Hover/active point. */
export const ACTIVE = SERIES[3];

/** Grid lines and borders — neutral, matching `--border`. */
export const GRID = "hsl(240 6% 90%)";

/** Shared Recharts tooltip styling so every chart's tooltip matches. */
export const TOOLTIP_CONTENT_STYLE = {
  background: "hsl(0 0% 100%)",
  border: `1px solid ${GRID}`,
  borderRadius: "10px",
  fontSize: "12px",
  color: "hsl(240 6% 12%)",
} as const;
