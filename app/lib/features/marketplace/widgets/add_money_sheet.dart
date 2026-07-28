import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../core/theme/app_colors.dart';
import '../../auth/providers/auth_provider.dart';
import '../providers/wallet_provider.dart';
import 'package:marketkit/core/config/currency.dart';
import 'package:marketkit/core/payments/checkout.dart';

/// Bottom sheet: enter an amount, pay via Razorpay, wallet gets credited on
/// verify (or by the server webhook if the app-side verify fails).
class AddMoneySheet extends ConsumerStatefulWidget {
  const AddMoneySheet({super.key});

  static Future<void> show(BuildContext context) {
    return showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      backgroundColor: kCard,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      builder: (_) => const AddMoneySheet(),
    );
  }

  @override
  ConsumerState<AddMoneySheet> createState() => _AddMoneySheetState();
}

class _AddMoneySheetState extends ConsumerState<AddMoneySheet> {
  // Minor units — must match minTopupMinor in api/internal/modules/wallet.
  static const int _minTopupMinor = 1000;

  final _amountCtrl = TextEditingController();
  bool _isPaying = false;

  @override
  void dispose() {
    _amountCtrl.dispose();
    super.dispose();
  }

  void _snack(String msg, {Color? color}) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(msg), backgroundColor: color),
    );
  }

  int? get _amountMinor {
    final major = double.tryParse(_amountCtrl.text.trim());
    if (major == null || major <= 0) return null;
    return Currency.toMinor(major);
  }

  Future<void> _pay() async {
    final amount = _amountMinor;
    if (amount == null) {
      _snack('Enter a valid amount.', color: kDanger);
      return;
    }
    if (amount < _minTopupMinor) {
      _snack('Minimum top-up is ${Currency.format(_minTopupMinor)}.', color: kDanger);
      return;
    }
    final auth = ref.read(authProvider);
    setState(() => _isPaying = true);
    try {
      final raw =
          await ref.read(walletServiceProvider).createTopupOrder(amount);
      final result = await CheckoutService.pay(
        order: CheckoutOrder.fromJson(raw),
        description: 'Wallet top-up',
        email: auth.user?.email,
        phone: auth.user?.phone,
      );

      try {
        await ref.read(walletServiceProvider).verifyTopup(
              razorpayOrderId: result.orderId,
              razorpayPaymentId: result.paymentId,
              razorpaySignature: result.signature,
            );
        _snack('Money added to your wallet!', color: kSuccess);
      } catch (_) {
        // The server webhook still credits the top-up if this call fails.
        _snack('Payment received — your wallet will be credited shortly.');
      }

      if (!mounted) return;
      ref.invalidate(walletSummaryProvider);
      ref.invalidate(walletTransactionsProvider);
      Navigator.of(context).pop();
    } on CheckoutCancelled {
      // User backed out — nothing to report.
    } on CheckoutFailure catch (e) {
      _snack(e.message, color: kDanger);
    } catch (e) {
      _snack('Could not initiate payment: $e', color: kDanger);
    } finally {
      if (mounted) setState(() => _isPaying = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: EdgeInsets.fromLTRB(
          20, 20, 20, 20 + MediaQuery.of(context).viewInsets.bottom),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Text(
            'Add Money',
            style: TextStyle(
                fontSize: 18, fontWeight: FontWeight.w700, color: kForeground),
          ),
          const SizedBox(height: 16),
          TextField(
            controller: _amountCtrl,
            keyboardType: TextInputType.number,
            inputFormatters: [FilteringTextInputFormatter.digitsOnly],
            autofocus: true,
            decoration: InputDecoration(
              prefixText: '${Currency.symbol} ',
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
          const SizedBox(height: 12),
          Wrap(
            spacing: 8,
            children: [
              for (final amount in [100, 500, 1000, 2000])
                ActionChip(
                  label: Text(Currency.format(Currency.toMinor(amount.toDouble()))),
                  backgroundColor: kMuted,
                  side: BorderSide.none,
                  onPressed: () =>
                      setState(() => _amountCtrl.text = amount.toString()),
                ),
            ],
          ),
          const SizedBox(height: 20),
          SizedBox(
            width: double.infinity,
            child: ElevatedButton(
              onPressed: _isPaying || _amountMinor == null ? null : _pay,
              style: ElevatedButton.styleFrom(
                backgroundColor: kPrimary,
                foregroundColor: Colors.white,
                padding: const EdgeInsets.symmetric(vertical: 14),
                shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(10)),
              ),
              child: _isPaying
                  ? const SizedBox(
                      width: 20,
                      height: 20,
                      child: CircularProgressIndicator(
                          color: Colors.white, strokeWidth: 2),
                    )
                  : const Text('Proceed to Pay',
                      style: TextStyle(fontWeight: FontWeight.w600)),
            ),
          ),
        ],
      ),
    );
  }
}
