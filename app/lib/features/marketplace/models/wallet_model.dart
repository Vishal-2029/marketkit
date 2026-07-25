String formatPaise(int paise) {
  final rupees = paise / 100;
  if (rupees == rupees.roundToDouble()) return '₹${rupees.toInt()}';
  return '₹${rupees.toStringAsFixed(2)}';
}

class WalletSummary {
  final int balanceInPaise;
  final bool hasUpi;
  final bool hasBank;
  final int minWithdrawalInPaise;

  const WalletSummary({
    required this.balanceInPaise,
    required this.hasUpi,
    required this.hasBank,
    required this.minWithdrawalInPaise,
  });

  factory WalletSummary.fromJson(Map<String, dynamic> json) => WalletSummary(
        balanceInPaise: (json['balance_in_paise'] as num?)?.toInt() ?? 0,
        hasUpi: json['has_upi'] as bool? ?? false,
        hasBank: json['has_bank'] as bool? ?? false,
        minWithdrawalInPaise:
            (json['min_withdrawal_in_paise'] as num?)?.toInt() ?? 10000,
      );

  String get formattedBalance => formatPaise(balanceInPaise);
}

class WalletTransactionModel {
  final String id;
  final String type; // TOPUP | PURCHASE_DEBIT | SALE_CREDIT | WITHDRAWAL
  final int amountInPaise; // signed: credits positive, debits negative
  final int balanceAfterInPaise;
  final DateTime createdAt;

  const WalletTransactionModel({
    required this.id,
    required this.type,
    required this.amountInPaise,
    required this.balanceAfterInPaise,
    required this.createdAt,
  });

  factory WalletTransactionModel.fromJson(Map<String, dynamic> json) =>
      WalletTransactionModel(
        id: json['id'] as String,
        type: json['type'] as String? ?? '',
        amountInPaise: (json['amount_in_paise'] as num?)?.toInt() ?? 0,
        balanceAfterInPaise:
            (json['balance_after_in_paise'] as num?)?.toInt() ?? 0,
        createdAt:
            DateTime.tryParse(json['created_at'] as String? ?? '')?.toLocal() ??
                DateTime.now(),
      );

  bool get isCredit => amountInPaise >= 0;

  String get label => switch (type) {
        'TOPUP' => 'Money added',
        'PURCHASE_DEBIT' => 'Design purchased',
        'SALE_CREDIT' => 'Design sold',
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
  final int amountInPaise;
  final String method; // UPI | BANK
  final String status; // APPROVED | SETTLED
  final DateTime createdAt;

  const WithdrawalModel({
    required this.id,
    required this.amountInPaise,
    required this.method,
    required this.status,
    required this.createdAt,
  });

  factory WithdrawalModel.fromJson(Map<String, dynamic> json) =>
      WithdrawalModel(
        id: json['id'] as String,
        amountInPaise: (json['amount_in_paise'] as num?)?.toInt() ?? 0,
        method: json['method'] as String? ?? '',
        status: json['status'] as String? ?? '',
        createdAt:
            DateTime.tryParse(json['created_at'] as String? ?? '')?.toLocal() ??
                DateTime.now(),
      );
}
