import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../models/wallet_model.dart';
import '../services/wallet_service.dart';

final walletServiceProvider = Provider<WalletService>((ref) => WalletService());

final walletSummaryProvider =
    FutureProvider.autoDispose<WalletSummary>((ref) async {
  return ref.read(walletServiceProvider).fetchSummary();
});

final walletTransactionsProvider =
    FutureProvider.autoDispose<List<WalletTransactionModel>>((ref) async {
  return ref.read(walletServiceProvider).fetchTransactions();
});

final payoutDetailsProvider =
    FutureProvider.autoDispose<PayoutDetailsModel>((ref) async {
  return ref.read(walletServiceProvider).fetchPayoutDetails();
});

final myWithdrawalsProvider =
    FutureProvider.autoDispose<List<WithdrawalModel>>((ref) async {
  return ref.read(walletServiceProvider).fetchMyWithdrawals();
});

/// Platform fee percent for the upload form's live earnings breakdown.
final marketFeeProvider = FutureProvider.autoDispose<int>((ref) async {
  return ref.read(walletServiceProvider).fetchFeePercent();
});
