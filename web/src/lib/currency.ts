/**
 * Currency formatting for the admin panel.
 *
 * Every monetary value the API returns is an integer in the currency's *minor
 * unit* — paise for INR, cents for USD, yen for JPY. Never divide by 100
 * inline: zero-decimal currencies (JPY, KRW) and three-decimal ones (KWD, BHD)
 * would render wrong. Use the helpers here.
 *
 * CURRENCY must match PAYMENT_CURRENCY in the API's .env. It is a build-time
 * constant rather than an API call so the first paint has no loading state;
 * override it at build time with VITE_CURRENCY.
 */

export const CURRENCY = (import.meta.env.VITE_CURRENCY as string | undefined) ?? "INR";

/** Currencies whose minor unit is not 1/100. Anything absent uses 2. */
const EXPONENTS: Record<string, number> = {
  BIF: 0, CLP: 0, DJF: 0, GNF: 0, JPY: 0, KMF: 0, KRW: 0, MGA: 0,
  PYG: 0, RWF: 0, UGX: 0, VND: 0, VUV: 0, XAF: 0, XOF: 0, XPF: 0,
  BHD: 3, IQD: 3, JOD: 3, KWD: 3, LYD: 3, OMR: 3, TND: 3,
};

export const decimals = (currency: string = CURRENCY): number =>
  EXPONENTS[currency.toUpperCase()] ?? 2;

/**
 * Format a minor-unit amount for display: `formatMoney(499900)` is "₹4,999"
 * under INR and "$4,999" under USD. Whole amounts drop the decimals;
 * fractional amounts show them in full ("$4,999.50").
 *
 * Uses Intl, so grouping and symbol placement follow the currency's own
 * conventions rather than being hardcoded to one locale.
 */
export function formatMoney(minor: number, currency: string = CURRENCY): string {
  const d = decimals(currency);
  const major = minor / 10 ** d;
  const hasFraction = minor % 10 ** d !== 0;
  return new Intl.NumberFormat(undefined, {
    style: "currency",
    currency,
    minimumFractionDigits: hasFraction ? d : 0,
    maximumFractionDigits: hasFraction ? d : 0,
  }).format(major);
}

/** Format without the currency symbol — for inputs and CSV columns. */
export function formatMoneyPlain(minor: number, currency: string = CURRENCY): string {
  const d = decimals(currency);
  const major = minor / 10 ** d;
  const hasFraction = minor % 10 ** d !== 0;
  return new Intl.NumberFormat(undefined, {
    minimumFractionDigits: hasFraction ? d : 0,
    maximumFractionDigits: hasFraction ? d : 0,
  }).format(major);
}

/** The currency's symbol, for input prefixes and axis labels. */
export function currencySymbol(currency: string = CURRENCY): string {
  const parts = new Intl.NumberFormat(undefined, { style: "currency", currency })
    .formatToParts(0);
  return parts.find((p) => p.type === "currency")?.value ?? currency;
}

/** Minor units → major, for chart axes that need a number, not a string. */
export const toMajor = (minor: number, currency: string = CURRENCY): number =>
  minor / 10 ** decimals(currency);

/** Major units → minor, for form inputs where the user types "499.50". */
export const toMinor = (major: number, currency: string = CURRENCY): number =>
  Math.round(major * 10 ** decimals(currency));
