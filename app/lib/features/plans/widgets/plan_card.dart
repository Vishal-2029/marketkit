import 'package:flutter/material.dart';
import '../../../core/theme/app_colors.dart';
import '../models/plan_model.dart';

class PlanCard extends StatelessWidget {
  final PlanModel plan;
  final bool isCurrentPlan;
  final bool isProcessing;
  final VoidCallback onBuyTap;

  const PlanCard({
    super.key,
    required this.plan,
    required this.isCurrentPlan,
    required this.isProcessing,
    required this.onBuyTap,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      decoration: BoxDecoration(
        color: kCard,
        borderRadius: BorderRadius.circular(16),
        border: Border(
          left: BorderSide(
            color: isCurrentPlan ? kSage : kGold,
            width: 4,
          ),
        ),
        boxShadow: [
          BoxShadow(
            color: Colors.black.withOpacity(0.05),
            blurRadius: 8,
            offset: const Offset(0, 2),
          ),
        ],
      ),
      padding: const EdgeInsets.all(20),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(
                child: Text(
                  plan.name,
                  style: const TextStyle(
                    fontSize: 18,
                    fontWeight: FontWeight.w700,
                    color: kForeground,
                  ),
                ),
              ),
              if (isCurrentPlan)
                Container(
                  padding:
                      const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
                  decoration: BoxDecoration(
                    color: kSage.withOpacity(0.15),
                    borderRadius: BorderRadius.circular(20),
                  ),
                  child: const Text(
                    'Current Plan',
                    style: TextStyle(
                      fontSize: 11,
                      fontWeight: FontWeight.w600,
                      color: kSage,
                    ),
                  ),
                ),
            ],
          ),
          const SizedBox(height: 4),
          RichText(
            text: TextSpan(
              children: [
                TextSpan(
                  text: plan.formattedPrice,
                  style: const TextStyle(
                    fontSize: 24,
                    fontWeight: FontWeight.w800,
                    color: kGold,
                  ),
                ),
                TextSpan(
                  text: ' / ${plan.durationLabel}',
                  style: const TextStyle(
                    fontSize: 13,
                    color: kMutedForeground,
                  ),
                ),
              ],
            ),
          ),
          if (plan.description.trim().isNotEmpty) ...[
            const SizedBox(height: 8),
            Text(
              plan.description,
              style: const TextStyle(
                fontSize: 13,
                height: 1.4,
                color: kMutedForeground,
              ),
            ),
          ],
          const SizedBox(height: 16),
          const Divider(),
          const SizedBox(height: 12),
          _FeatureRow(label: 'Wilcom 2006', enabled: plan.hasWillcom),
          const SizedBox(height: 8),
          _FeatureRow(label: 'E4', enabled: plan.hasE4),
          const SizedBox(height: 8),
          _FeatureRow(label: 'meCAD', enabled: plan.hasMecad),
          const SizedBox(height: 20),
          if (!isCurrentPlan)
            SizedBox(
              width: double.infinity,
              child: ElevatedButton(
                onPressed: isProcessing ? null : onBuyTap,
                style: ElevatedButton.styleFrom(
                  backgroundColor: kGold,
                  foregroundColor: Colors.white,
                  padding: const EdgeInsets.symmetric(vertical: 14),
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(10),
                  ),
                ),
                child: isProcessing
                    ? const SizedBox(
                        height: 18,
                        width: 18,
                        child: CircularProgressIndicator(
                          color: Colors.white,
                          strokeWidth: 2,
                        ),
                      )
                    : const Text(
                        'Buy Now',
                        style: TextStyle(
                            fontSize: 15, fontWeight: FontWeight.w600),
                      ),
              ),
            ),
        ],
      ),
    );
  }
}

class _FeatureRow extends StatelessWidget {
  final String label;
  final bool enabled;

  const _FeatureRow({required this.label, required this.enabled});

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Icon(
          enabled ? Icons.check_circle_rounded : Icons.cancel_rounded,
          size: 18,
          color: enabled ? kSage : kMutedForeground,
        ),
        const SizedBox(width: 8),
        Text(
          label,
          style: TextStyle(
            fontSize: 14,
            color: enabled ? kForeground : kMutedForeground,
            fontWeight: enabled ? FontWeight.w500 : FontWeight.normal,
          ),
        ),
      ],
    );
  }
}
