import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../core/theme/app_colors.dart';
import '../models/wallet_model.dart';
import '../providers/wallet_provider.dart';

/// Bottom sheet: pick UPI or bank, enter an amount, and withdraw. The request
/// is approved instantly (balance deducted); the payout itself is transferred
/// by the admin.
class WithdrawSheet extends ConsumerStatefulWidget {
  final WalletSummary summary;
  const WithdrawSheet({super.key, required this.summary});

  static Future<void> show(BuildContext context, WalletSummary summary) {
    return showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      backgroundColor: kCard,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      builder: (_) => WithdrawSheet(summary: summary),
    );
  }

  @override
  ConsumerState<WithdrawSheet> createState() => _WithdrawSheetState();
}

class _WithdrawSheetState extends ConsumerState<WithdrawSheet> {
  final _amountCtrl = TextEditingController();
  String? _method;
  bool _isSubmitting = false;

  @override
  void initState() {
    super.initState();
    if (widget.summary.hasUpi) {
      _method = 'UPI';
    } else if (widget.summary.hasBank) {
      _method = 'BANK';
    }
  }

  @override
  void dispose() {
    _amountCtrl.dispose();
    super.dispose();
  }

  int? get _amountInPaise {
    final rupees = int.tryParse(_amountCtrl.text.trim());
    if (rupees == null || rupees <= 0) return null;
    return rupees * 100;
  }

  bool get _canSubmit {
    final amount = _amountInPaise;
    return _method != null &&
        amount != null &&
        amount >= widget.summary.minWithdrawalInPaise &&
        amount <= widget.summary.balanceInPaise &&
        !_isSubmitting;
  }

  Future<void> _submit() async {
    final amount = _amountInPaise;
    final method = _method;
    if (amount == null || method == null) return;
    setState(() => _isSubmitting = true);
    try {
      await ref.read(walletServiceProvider).createWithdrawal(
            amountInPaise: amount,
            method: method,
          );
      if (!mounted) return;
      ref.invalidate(walletSummaryProvider);
      ref.invalidate(walletTransactionsProvider);
      ref.invalidate(myWithdrawalsProvider);
      Navigator.of(context).pop();
      ScaffoldMessenger.of(context).showSnackBar(const SnackBar(
        content:
            Text('Withdrawal approved — the payout will be transferred soon.'),
        backgroundColor: kSage,
      ));
    } on DioException catch (e) {
      final msg = e.response?.data?['error'] as String?;
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(
          content: Text(msg ?? 'Withdrawal failed. Please try again.'),
          backgroundColor: kTerracotta,
        ));
      }
    } catch (_) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(const SnackBar(
          content: Text('Withdrawal failed. Please try again.'),
          backgroundColor: kTerracotta,
        ));
      }
    } finally {
      if (mounted) setState(() => _isSubmitting = false);
    }
  }

  Widget _methodTile({
    required String value,
    required IconData icon,
    required String title,
    required bool available,
  }) {
    final selected = _method == value;
    return Expanded(
      child: InkWell(
        borderRadius: BorderRadius.circular(12),
        onTap: available
            ? () => setState(() => _method = value)
            : () async {
                Navigator.of(context).pop();
                context.push('/market/wallet/payout-details');
              },
        child: Container(
          padding: const EdgeInsets.symmetric(vertical: 14),
          decoration: BoxDecoration(
            color: selected ? kGold.withValues(alpha: 0.1) : kMuted,
            borderRadius: BorderRadius.circular(12),
            border: Border.all(color: selected ? kGold : kBorder),
          ),
          child: Column(
            children: [
              Icon(icon,
                  size: 22, color: selected ? kGold : kMutedForeground),
              const SizedBox(height: 6),
              Text(
                title,
                style: TextStyle(
                  fontSize: 13,
                  fontWeight: FontWeight.w600,
                  color: selected ? kGold : kForeground,
                ),
              ),
              if (!available)
                const Text('Add details',
                    style: TextStyle(fontSize: 10, color: kTerracotta)),
            ],
          ),
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final s = widget.summary;
    return Padding(
      padding: EdgeInsets.fromLTRB(
          20, 20, 20, 20 + MediaQuery.of(context).viewInsets.bottom),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Text(
            'Withdraw',
            style: TextStyle(
                fontSize: 18, fontWeight: FontWeight.w700, color: kForeground),
          ),
          const SizedBox(height: 4),
          Text(
            'Available: ${s.formattedBalance} · minimum ${formatPaise(s.minWithdrawalInPaise)}',
            style: const TextStyle(fontSize: 12, color: kMutedForeground),
          ),
          const SizedBox(height: 16),
          Row(
            children: [
              _methodTile(
                value: 'UPI',
                icon: Icons.qr_code_rounded,
                title: 'UPI',
                available: s.hasUpi,
              ),
              const SizedBox(width: 10),
              _methodTile(
                value: 'BANK',
                icon: Icons.account_balance_outlined,
                title: 'Bank',
                available: s.hasBank,
              ),
            ],
          ),
          const SizedBox(height: 14),
          TextField(
            controller: _amountCtrl,
            keyboardType: TextInputType.number,
            inputFormatters: [FilteringTextInputFormatter.digitsOnly],
            decoration: InputDecoration(
              prefixText: '₹ ',
              hintText: 'Amount',
              filled: true,
              fillColor: kInput,
              border: OutlineInputBorder(
                borderRadius: BorderRadius.circular(12),
                borderSide: BorderSide.none,
              ),
            ),
            onChanged: (_) => setState(() {}),
          ),
          const SizedBox(height: 20),
          SizedBox(
            width: double.infinity,
            child: ElevatedButton(
              onPressed: _canSubmit ? _submit : null,
              style: ElevatedButton.styleFrom(
                backgroundColor: kGold,
                foregroundColor: Colors.white,
                padding: const EdgeInsets.symmetric(vertical: 14),
                shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(10)),
              ),
              child: _isSubmitting
                  ? const SizedBox(
                      width: 20,
                      height: 20,
                      child: CircularProgressIndicator(
                          color: Colors.white, strokeWidth: 2),
                    )
                  : const Text('Withdraw',
                      style: TextStyle(fontWeight: FontWeight.w600)),
            ),
          ),
        ],
      ),
    );
  }
}
