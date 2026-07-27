import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../core/theme/app_colors.dart';
import '../providers/wallet_provider.dart';

/// Where withdrawals get paid: UPI ID and/or bank account. Saved once,
/// snapshotted onto each withdrawal request.
class PayoutDetailsScreen extends ConsumerStatefulWidget {
  const PayoutDetailsScreen({super.key});

  @override
  ConsumerState<PayoutDetailsScreen> createState() =>
      _PayoutDetailsScreenState();
}

class _PayoutDetailsScreenState extends ConsumerState<PayoutDetailsScreen> {
  final _upiCtrl = TextEditingController();
  final _accountCtrl = TextEditingController();
  final _ifscCtrl = TextEditingController();
  final _holderCtrl = TextEditingController();
  bool _isSaving = false;
  bool _prefilled = false;

  @override
  void dispose() {
    _upiCtrl.dispose();
    _accountCtrl.dispose();
    _ifscCtrl.dispose();
    _holderCtrl.dispose();
    super.dispose();
  }

  Future<void> _save() async {
    setState(() => _isSaving = true);
    try {
      await ref.read(walletServiceProvider).savePayoutDetails(
            upiId: _upiCtrl.text.trim(),
            bankAccountNumber: _accountCtrl.text.trim(),
            bankIfsc: _ifscCtrl.text.trim().toUpperCase(),
            bankHolderName: _holderCtrl.text.trim(),
          );
      if (!mounted) return;
      ref.invalidate(payoutDetailsProvider);
      ref.invalidate(walletSummaryProvider);
      Navigator.of(context).pop();
      ScaffoldMessenger.of(context).showSnackBar(const SnackBar(
        content: Text('Payout details saved.'),
        backgroundColor: kSuccess,
      ));
    } on DioException catch (e) {
      final msg = e.response?.data?['error'] as String?;
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(
          content: Text(msg ?? 'Could not save payout details.'),
          backgroundColor: kDanger,
        ));
      }
    } catch (_) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(const SnackBar(
          content: Text('Could not save payout details.'),
          backgroundColor: kDanger,
        ));
      }
    } finally {
      if (mounted) setState(() => _isSaving = false);
    }
  }

  InputDecoration _dec(String label, {String? hint}) => InputDecoration(
        labelText: label,
        hintText: hint,
        filled: true,
        fillColor: kInput,
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(12),
          borderSide: BorderSide.none,
        ),
      );

  @override
  Widget build(BuildContext context) {
    final details = ref.watch(payoutDetailsProvider);

    // Prefill once when the saved details arrive.
    details.whenData((d) {
      if (_prefilled) return;
      _prefilled = true;
      _upiCtrl.text = d.upiId;
      _accountCtrl.text = d.bankAccountNumber;
      _ifscCtrl.text = d.bankIfsc;
      _holderCtrl.text = d.bankHolderName;
    });

    return Scaffold(
      backgroundColor: kBackground,
      appBar: AppBar(
        backgroundColor: kBackground,
        elevation: 0,
        title: const Text(
          'Payout Details',
          style: TextStyle(fontWeight: FontWeight.w700, fontSize: 18),
        ),
        centerTitle: false,
      ),
      body: details.isLoading && !_prefilled
          ? const Center(
              child: CircularProgressIndicator(color: kPrimary, strokeWidth: 2))
          : ListView(
              padding: const EdgeInsets.all(16),
              children: [
                const Text(
                  'UPI',
                  style: TextStyle(
                      fontSize: 13,
                      fontWeight: FontWeight.w700,
                      color: kMutedForeground),
                ),
                const SizedBox(height: 8),
                TextField(
                  controller: _upiCtrl,
                  decoration: _dec('UPI ID', hint: 'yourname@upi'),
                ),
                const SizedBox(height: 24),
                const Text(
                  'BANK ACCOUNT',
                  style: TextStyle(
                      fontSize: 13,
                      fontWeight: FontWeight.w700,
                      color: kMutedForeground),
                ),
                const SizedBox(height: 8),
                TextField(
                  controller: _accountCtrl,
                  keyboardType: TextInputType.number,
                  decoration: _dec('Account number'),
                ),
                const SizedBox(height: 12),
                TextField(
                  controller: _ifscCtrl,
                  textCapitalization: TextCapitalization.characters,
                  decoration: _dec('IFSC code', hint: 'HDFC0001234'),
                ),
                const SizedBox(height: 12),
                TextField(
                  controller: _holderCtrl,
                  decoration: _dec('Account holder name'),
                ),
                const SizedBox(height: 12),
                const Text(
                  'Fill either UPI, bank details, or both. Withdrawals are '
                  'paid to the details saved here.',
                  style: TextStyle(fontSize: 12, color: kMutedForeground),
                ),
                const SizedBox(height: 24),
                ElevatedButton(
                  onPressed: _isSaving ? null : _save,
                  style: ElevatedButton.styleFrom(
                    backgroundColor: kPrimary,
                    foregroundColor: Colors.white,
                    padding: const EdgeInsets.symmetric(vertical: 14),
                    shape: RoundedRectangleBorder(
                        borderRadius: BorderRadius.circular(10)),
                  ),
                  child: _isSaving
                      ? const SizedBox(
                          width: 20,
                          height: 20,
                          child: CircularProgressIndicator(
                              color: Colors.white, strokeWidth: 2),
                        )
                      : const Text('Save',
                          style: TextStyle(fontWeight: FontWeight.w600)),
                ),
              ],
            ),
    );
  }
}
