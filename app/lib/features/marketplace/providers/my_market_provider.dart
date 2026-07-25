import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../models/design_model.dart';
import '../models/earnings_model.dart';
import '../models/purchase_model.dart';
import 'designs_provider.dart';

/// The current user's own listings (including unlisted ones).
final myDesignsProvider =
    FutureProvider.autoDispose<List<DesignModel>>((ref) async {
  return ref.read(marketServiceProvider).fetchMyDesigns();
});

/// The current user's successful purchases.
final myPurchasesProvider =
    FutureProvider.autoDispose<List<PurchaseModel>>((ref) async {
  return ref.read(marketServiceProvider).fetchMyPurchases();
});

/// Seller earnings summary (informational — payouts are handled manually).
final earningsProvider = FutureProvider.autoDispose<EarningsModel>((ref) async {
  return ref.read(marketServiceProvider).fetchEarnings();
});
