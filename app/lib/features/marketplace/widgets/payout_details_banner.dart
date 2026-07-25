import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../core/theme/app_colors.dart';
import '../providers/wallet_provider.dart';

/// Persistent nudge for sellers who haven't added payout details yet.
/// Stays visible — no dismiss/snooze — until UPI or bank details are on file.
class PayoutDetailsBanner extends ConsumerWidget {
  const PayoutDetailsBanner({super.key, required this.child});

  final Widget child;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final summary = ref.watch(walletSummaryProvider);
    final needsPayoutDetails = summary.maybeWhen(
      data: (s) => !s.hasUpi && !s.hasBank,
      orElse: () => false,
    );

    return Column(
      children: [
        if (needsPayoutDetails)
          Material(
            color: kGold.withValues(alpha: 0.15),
            child: InkWell(
              onTap: () => context.push('/market/wallet/payout-details'),
              child: const SafeArea(
                bottom: false,
                child: Padding(
                  padding: EdgeInsets.symmetric(horizontal: 16, vertical: 10),
                  child: Row(
                    children: [
                      Icon(Icons.account_balance_outlined, color: kGold, size: 18),
                      SizedBox(width: 10),
                      Expanded(
                        child: Text(
                          'Add your bank/UPI details to receive payouts for your sales',
                          style: TextStyle(
                            fontSize: 12.5,
                            fontWeight: FontWeight.w600,
                            color: kForeground,
                          ),
                        ),
                      ),
                      SizedBox(width: 8),
                      Icon(Icons.chevron_right_rounded, color: kGold, size: 18),
                    ],
                  ),
                ),
              ),
            ),
          ),
        Expanded(child: child),
      ],
    );
  }
}
