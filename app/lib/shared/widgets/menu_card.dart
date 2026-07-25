import 'package:flutter/material.dart';

import '../../core/theme/app_colors.dart';

/// Small labeled section heading, e.g. "ACCOUNT" — shared between the
/// Learning and Design Market profile screens for a consistent look.
class SectionHeader extends StatelessWidget {
  final String title;
  const SectionHeader({super.key, required this.title});

  @override
  Widget build(BuildContext context) {
    return Align(
      alignment: Alignment.centerLeft,
      child: Padding(
        padding: const EdgeInsets.only(left: 4, bottom: 8),
        child: Text(
          title,
          style: const TextStyle(
            fontSize: 11,
            fontWeight: FontWeight.w700,
            color: kMutedForeground,
            letterSpacing: 0.8,
          ),
        ),
      ),
    );
  }
}

/// Rounded card grouping a set of [MenuTile]s.
class MenuCard extends StatelessWidget {
  final List<Widget> children;
  const MenuCard({super.key, required this.children});

  @override
  Widget build(BuildContext context) {
    return Container(
      width: double.infinity,
      decoration: BoxDecoration(
        color: kCard,
        borderRadius: BorderRadius.circular(14),
        boxShadow: [
          BoxShadow(
            color: Colors.black.withOpacity(0.04),
            blurRadius: 6,
            offset: const Offset(0, 2),
          ),
        ],
      ),
      child: Column(children: children),
    );
  }
}

class MenuTile extends StatelessWidget {
  final IconData icon;
  final Color? iconColor;
  final String label;
  final Widget? trailing;
  final VoidCallback? onTap;

  const MenuTile({
    super.key,
    required this.icon,
    this.iconColor,
    required this.label,
    this.trailing,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return ListTile(
      onTap: onTap,
      leading: Icon(icon, color: iconColor ?? kGold, size: 20),
      title: Text(label, style: const TextStyle(fontSize: 14, color: kForeground)),
      trailing: trailing ??
          (onTap != null
              ? const Icon(Icons.chevron_right_rounded, color: kMutedForeground, size: 20)
              : null),
      contentPadding: const EdgeInsets.symmetric(horizontal: 16, vertical: 2),
    );
  }
}

class MenuDivider extends StatelessWidget {
  const MenuDivider({super.key});

  @override
  Widget build(BuildContext context) {
    return const Divider(height: 1, indent: 52, endIndent: 16, color: kBorder);
  }
}
