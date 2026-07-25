import 'package:flutter/material.dart';
import 'package:shimmer/shimmer.dart';
import '../../core/theme/app_colors.dart';

/// A list of shimmer placeholder cards — used while async data loads.
/// [itemHeight] controls the card height; [count] controls how many show.
class ShimmerList extends StatelessWidget {
  const ShimmerList({
    super.key,
    this.itemHeight = 80,
    this.count = 6,
    this.padding = const EdgeInsets.all(16),
  });

  final double itemHeight;
  final int count;
  final EdgeInsets padding;

  @override
  Widget build(BuildContext context) {
    return ListView.separated(
      padding: padding,
      itemCount: count,
      separatorBuilder: (_, __) => const SizedBox(height: 10),
      itemBuilder: (_, __) => Shimmer.fromColors(
        baseColor: kMuted,
        highlightColor: kCard,
        child: Container(
          height: itemHeight,
          decoration: BoxDecoration(
            color: kMuted,
            borderRadius: BorderRadius.circular(12),
          ),
        ),
      ),
    );
  }
}

/// A single shimmer card placeholder.
class ShimmerCard extends StatelessWidget {
  const ShimmerCard({super.key, this.height = 80});

  final double height;

  @override
  Widget build(BuildContext context) {
    return Shimmer.fromColors(
      baseColor: kMuted,
      highlightColor: kCard,
      child: Container(
        height: height,
        decoration: BoxDecoration(
          color: kMuted,
          borderRadius: BorderRadius.circular(12),
        ),
      ),
    );
  }
}
