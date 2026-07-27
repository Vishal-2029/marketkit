import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:razorpay_flutter/razorpay_flutter.dart';
import '../../../core/theme/app_colors.dart';
import '../../auth/providers/auth_provider.dart';
import '../providers/wallet_provider.dart';
import 'package:marketkit/core/config/brand.dart';

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
  late Razorpay _razorpay;
  final _amountCtrl = TextEditingController();
  bool _isPaying = false;

  @override
  void initState() {
    super.initState();
    _razorpay = Razorpay();
    _razorpay.on(Razorpay.EVENT_PAYMENT_SUCCESS, _handleSuccess);
    _razorpay.on(Razorpay.EVENT_PAYMENT_ERROR, _handleFailure);
    _razorpay.on(Razorpay.EVENT_EXTERNAL_WALLET, _handleExternalWallet);
  }

  @override
  void dispose() {
    _razorpay.clear();
    _amountCtrl.dispose();
    super.dispose();
  }

  void _snack(String msg, {Color? color}) {
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(content: Text(msg), backgroundColor: color),
    );
  }

  int? get _amountInPaise {
    final rupees = int.tryParse(_amountCtrl.text.trim());
    if (rupees == null || rupees <= 0) return null;
    return rupees * 100;
  }

  Future<void> _pay() async {
    final amount = _amountInPaise;
    if (amount == null) {
      _snack('Enter a valid amount.', color: kDanger);
      return;
    }
    if (amount < 1000) {
      _snack('Minimum top-up is ₹10.', color: kDanger);
      return;
    }
    final auth = ref.read(authProvider);
    setState(() => _isPaying = true);
    try {
      final order =
          await ref.read(walletServiceProvider).createTopupOrder(amount);
      final keyId = order['key_id'] as String?;
      final orderId = order['order_id'] as String?;
      if (keyId == null || orderId == null) {
        _snack('Payment setup failed. Please try again.', color: kDanger);
        return;
      }
      _razorpay.open({
        'key': keyId,
        'amount': order['amount'],
        'order_id': orderId,
        'currency': order['currency'] ?? 'INR',
        'name': Brand.checkoutName,
        'description': 'Wallet top-up',
        'prefill': {
          'contact': auth.user?.phone ?? '',
          'email': auth.user?.email ?? '',
        },
      });
    } catch (e) {
      _snack('Could not initiate payment: $e', color: kDanger);
    } finally {
      if (mounted) setState(() => _isPaying = false);
    }
  }

  void _handleSuccess(PaymentSuccessResponse response) async {
    final orderId = response.orderId;
    final paymentId = response.paymentId;
    final signature = response.signature;
    try {
      if (orderId != null && paymentId != null && signature != null) {
        await ref.read(walletServiceProvider).verifyTopup(
              razorpayOrderId: orderId,
              razorpayPaymentId: paymentId,
              razorpaySignature: signature,
            );
      }
      _snack('Money added to your wallet!', color: kSuccess);
    } catch (_) {
      // Server webhook still credits the topup even if verify fails here.
      _snack('Payment received — your wallet will be credited shortly.');
    }
    if (!mounted) return;
    ref.invalidate(walletSummaryProvider);
    ref.invalidate(walletTransactionsProvider);
    Navigator.of(context).pop();
  }

  void _handleFailure(PaymentFailureResponse response) {
    _snack('Payment failed: ${response.message ?? 'Unknown error'}',
        color: kDanger);
  }

  void _handleExternalWallet(ExternalWalletResponse response) {
    _snack('External wallet selected: ${response.walletName ?? 'Unknown'}');
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
          const SizedBox(height: 12),
          Wrap(
            spacing: 8,
            children: [
              for (final amount in [100, 500, 1000, 2000])
                ActionChip(
                  label: Text('₹$amount'),
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
              onPressed: _isPaying || _amountInPaise == null ? null : _pay,
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
