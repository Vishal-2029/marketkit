import 'package:intl/intl.dart';

/// Currency formatting for the app.
///
/// Every monetary value the API returns is an integer in the currency's *minor
/// unit* — paise for INR, cents for USD, yen for JPY. Never divide by 100
/// inline: zero-decimal currencies (JPY, KRW) and three-decimal ones (KWD,
/// BHD) would render wrong. Use the helpers here.
///
/// [code] must match `PAYMENT_CURRENCY` in the API's `.env`. Override at build
/// time:
///
///   flutter run --dart-define=CURRENCY=USD
class Currency {
  const Currency._();

  static const String code = String.fromEnvironment('CURRENCY', defaultValue: 'INR');

  /// Currencies whose minor unit is not 1/100. Anything absent uses 2.
  static const Map<String, int> _exponents = {
    'BIF': 0, 'CLP': 0, 'DJF': 0, 'GNF': 0, 'JPY': 0, 'KMF': 0, 'KRW': 0,
    'MGA': 0, 'PYG': 0, 'RWF': 0, 'UGX': 0, 'VND': 0, 'VUV': 0, 'XAF': 0,
    'XOF': 0, 'XPF': 0,
    'BHD': 3, 'IQD': 3, 'JOD': 3, 'KWD': 3, 'LYD': 3, 'OMR': 3, 'TND': 3,
  };

  static int get decimals => _exponents[code.toUpperCase()] ?? 2;

  /// The currency's symbol, for input prefixes ("₹ ", "$ ").
  static String get symbol =>
      NumberFormat.simpleCurrency(name: code).currencySymbol;

  /// Formats a minor-unit amount: `format(499900)` is "₹4,999" under INR and
  /// "$4,999" under USD. Whole amounts drop the decimals; fractional amounts
  /// show them in full ("$4,999.50").
  static String format(int minor) {
    final d = decimals;
    final divisor = _pow10(d);
    final hasFraction = d > 0 && minor % divisor != 0;
    return NumberFormat.currency(
      name: code,
      symbol: symbol,
      decimalDigits: hasFraction ? d : 0,
    ).format(minor / divisor);
  }

  /// Formats without the symbol — for inputs and plain numeric display.
  static String formatPlain(int minor) {
    final d = decimals;
    final divisor = _pow10(d);
    final hasFraction = d > 0 && minor % divisor != 0;
    return NumberFormat.decimalPatternDigits(
      decimalDigits: hasFraction ? d : 0,
    ).format(minor / divisor);
  }

  /// Minor units → major. Display only; keep arithmetic in integer minor units.
  static double toMajor(int minor) => minor / _pow10(decimals);

  /// Major units → minor, for form inputs where the user types "499.50".
  static int toMinor(double major) => (major * _pow10(decimals)).round();

  static int _pow10(int n) {
    var out = 1;
    for (var i = 0; i < n; i++) {
      out *= 10;
    }
    return out;
  }
}
