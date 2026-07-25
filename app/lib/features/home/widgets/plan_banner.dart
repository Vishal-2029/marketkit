import 'package:flutter/material.dart';
import '../../../core/theme/app_colors.dart';

class PlanBanner extends StatelessWidget {
  final VoidCallback? onViewPlansTap;
  const PlanBanner({super.key, this.onViewPlansTap});

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: kCream.withOpacity(0.4),
        borderRadius: BorderRadius.circular(14),
        border: Border(left: BorderSide(color: kGold, width: 3)),
      ),
      child: Row(
        children: [
          const Icon(Icons.workspace_premium_rounded, color: kGold, size: 28),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: const [
                Text(
                  'No active subscription',
                  style: TextStyle(
                    fontSize: 14,
                    fontWeight: FontWeight.w600,
                    color: kForeground,
                  ),
                ),
                SizedBox(height: 2),
                Text(
                  'Upgrade to unlock all videos',
                  style: TextStyle(fontSize: 12, color: kMutedForeground),
                ),
              ],
            ),
          ),
          const SizedBox(width: 8),
          GestureDetector(
            onTap: onViewPlansTap,
            child: Container(
              padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 7),
              decoration: BoxDecoration(
                gradient: kGoldGradient,
                borderRadius: BorderRadius.circular(8),
              ),
              child: const Text(
                'View Plans',
                style: TextStyle(
                  color: Colors.white,
                  fontSize: 12,
                  fontWeight: FontWeight.w600,
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }
}
