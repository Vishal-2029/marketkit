import 'package:marketkit/core/config/currency.dart';

String formatMinor(int minor) => Currency.format(minor);

class WalletSummary {
  final int balanceMinor;
  final bool hasUpi;
  final bool hasBank;
  final int minWithdrawalMinor;

  const WalletSummary({
    required this.balanceMinor,
    required this.hasUpi,
    required this.hasBank,
    required this.minWithdrawalMinor,
  });

  factory WalletSummary.fromJson(Map<String, dynamic> json) => WalletSummary(
        balanceMinor: (json['balance_minor'] as num?)?.toInt() ?? 0,
        hasUpi: json['has_upi'] as bool? ?? false,
        hasBank: json['has_bank'] as bool? ?? false,
        minWithdrawalMinor:
            (json['min_withdrawal_minor'] as num?)?.toInt() ?? 10000,
      );

  String get formattedBalance => formatMinor(balanceMinor);
}

class WalletTransactionModel {
  final String id;
  final String type; // TOPUP | PURCHASE_DEBIT | SALE_CREDIT | WITHDRAWAL
  final int amountMinor; // signed: credits positive, debits negative
  final int balanceAfterMinor;
  final DateTime createdAt;

  const WalletTransactionModel({
    required this.id,
    required this.type,
    required this.amountMinor,
    required this.balanceAfterMinor,
    required this.createdAt,
  });

  factory WalletTransactionModel.fromJson(Map<String, dynamic> json) =>
      WalletTransactionModel(
        id: json['id'] as String,
        type: json['type'] as String? ?? '',
        amountMinor: (json['amount_minor'] as num?)?.toInt() ?? 0,
        balanceAfterMinor:
            (json['balance_after_minor'] as num?)?.toInt() ?? 0,
        createdAt:
            DateTime.tryParse(json['created_at'] as String? ?? '')?.toLocal() ??
                DateTime.now(),
      );

  bool get isCredit => amountMinor >= 0;

  String get label => switch (type) {
        'TOPUP' => 'Money added',
        'PURCHASE_DEBIT' => 'Product purchased',
        'SALE_CREDIT' => 'Product sold',
        'WITHDRAWAL' => 'Withdrawal',
        'PLAN_DEBIT' => 'Plan subscription',
        _ => type,
      };
}

class PayoutDetailsModel {
  final String upiId;
  final String bankAccountNumber;
  final String bankIfsc;
  final String bankHolderName;

  const PayoutDetailsModel({
    this.upiId = '',
    this.bankAccountNumber = '',
    this.bankIfsc = '',
    this.bankHolderName = '',
  });

  factory PayoutDetailsModel.fromJson(Map<String, dynamic> json) =>
      PayoutDetailsModel(
        upiId: json['upi_id'] as String? ?? '',
        bankAccountNumber: json['bank_account_number'] as String? ?? '',
        bankIfsc: json['bank_ifsc'] as String? ?? '',
        bankHolderName: json['bank_holder_name'] as String? ?? '',
      );
}

class WithdrawalModel {
  final String id;
  final int amountMinor;
  final String method; // UPI | BANK
  final String status; // APPROVED | SETTLED
  final DateTime createdAt;

  const WithdrawalModel({
    required this.id,
    required this.amountMinor,
    required this.method,
    required this.status,
    required this.createdAt,
  });

  factory WithdrawalModel.fromJson(Map<String, dynamic> json) =>
      WithdrawalModel(
        id: json['id'] as String,
        amountMinor: (json['amount_minor'] as num?)?.toInt() ?? 0,
        method: json['method'] as String? ?? '',
        status: json['status'] as String? ?? '',
        createdAt:
            DateTime.tryParse(json['created_at'] as String? ?? '')?.toLocal() ??
                DateTime.now(),
      );
}
