import 'package:flutter/material.dart';
import '../../../core/theme/app_colors.dart';

class CategoryPill extends StatelessWidget {
  final String label;
  final bool active;
  final VoidCallback onTap;

  const CategoryPill({
    super.key,
    required this.label,
    required this.active,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: onTap,
      child: AnimatedContainer(
        duration: const Duration(milliseconds: 180),
        padding: const EdgeInsets.symmetric(horizontal: 18, vertical: 8),
        decoration: BoxDecoration(
          color: active ? kPrimary : kCard,
          borderRadius: BorderRadius.circular(50),
          border: Border.all(color: active ? kPrimary : kBorder),
        ),
        child: Text(
          label,
          style: TextStyle(
            color: active ? Colors.white : kMutedForeground,
            fontSize: 13,
            fontWeight: active ? FontWeight.w600 : FontWeight.w400,
          ),
        ),
      ),
    );
  }
}
