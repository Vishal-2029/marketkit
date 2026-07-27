import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../core/theme/app_colors.dart';
import '../models/wallet_model.dart';
import '../providers/wallet_provider.dart';

class WalletTransactionsScreen extends ConsumerWidget {
  const WalletTransactionsScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final txs = ref.watch(walletTransactionsProvider);

    return Scaffold(
      backgroundColor: kBackground,
      appBar: AppBar(
        backgroundColor: kBackground,
        elevation: 0,
        title: const Text(
          'Wallet History',
          style: TextStyle(fontWeight: FontWeight.w700, fontSize: 18),
        ),
        centerTitle: false,
      ),
      body: RefreshIndicator(
        color: kPrimary,
        onRefresh: () async => ref.invalidate(walletTransactionsProvider),
        child: txs.when(
          data: (list) => list.isEmpty
              ? ListView(
                  children: const [
                    SizedBox(height: 120),
                    Center(
                      child: Text('No wallet activity yet.',
                          style: TextStyle(color: kMutedForeground)),
                    ),
                  ],
                )
              : ListView.builder(
                  padding: const EdgeInsets.all(16),
                  itemCount: list.length,
                  itemBuilder: (_, i) => _TxTile(tx: list[i]),
                ),
          loading: () => const Center(
              child: CircularProgressIndicator(color: kPrimary, strokeWidth: 2)),
          error: (_, __) => ListView(
            children: const [
              SizedBox(height: 120),
              Center(
                child: Text('Could not load wallet history.',
                    style: TextStyle(color: kMutedForeground)),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _TxTile extends StatelessWidget {
  final WalletTransactionModel tx;
  const _TxTile({required this.tx});

  IconData get _icon => switch (tx.type) {
        'TOPUP' => Icons.add_circle_outline_rounded,
        'PURCHASE_DEBIT' => Icons.shopping_bag_outlined,
        'SALE_CREDIT' => Icons.sell_outlined,
        'WITHDRAWAL' => Icons.account_balance_outlined,
        _ => Icons.swap_horiz_rounded,
      };

  @override
  Widget build(BuildContext context) {
    final credit = tx.isCredit;
    final amountColor = credit ? kSuccess : kDanger;
    final sign = credit ? '+' : '−';
    final d = tx.createdAt;
    final date =
        '${d.day.toString().padLeft(2, '0')}/${d.month.toString().padLeft(2, '0')}/${d.year}';

    return Container(
      margin: const EdgeInsets.only(bottom: 8),
      padding: const EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: kCard,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: kBorder),
      ),
      child: Row(
        children: [
          Container(
            width: 38,
            height: 38,
            decoration: BoxDecoration(
              color: amountColor.withValues(alpha: 0.1),
              shape: BoxShape.circle,
            ),
            child: Icon(_icon, size: 18, color: amountColor),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  tx.label,
                  style: const TextStyle(
                      fontSize: 13,
                      fontWeight: FontWeight.w600,
                      color: kForeground),
                ),
                const SizedBox(height: 2),
                Text(
                  '$date · balance ${formatPaise(tx.balanceAfterInPaise)}',
                  style:
                      const TextStyle(fontSize: 11, color: kMutedForeground),
                ),
              ],
            ),
          ),
          Text(
            '$sign${formatPaise(tx.amountInPaise.abs())}',
            style: TextStyle(
              fontSize: 14,
              fontWeight: FontWeight.w700,
              color: amountColor,
            ),
          ),
        ],
      ),
    );
  }
}
