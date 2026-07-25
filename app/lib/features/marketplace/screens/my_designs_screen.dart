import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../core/theme/app_colors.dart';
import '../models/design_model.dart';
import '../models/earnings_model.dart';
import '../providers/designs_provider.dart';
import '../providers/my_market_provider.dart';

/// Seller-facing list of every design they've uploaded, with sales/view
/// stats, plus a total-earnings summary and a best-sellers chart built from
/// the same per-seller earnings data shown on the Market profile tab.
class MyDesignsScreen extends ConsumerWidget {
  const MyDesignsScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final myDesigns = ref.watch(myDesignsProvider);
    final earnings = ref.watch(earningsProvider);

    return Scaffold(
      backgroundColor: kBackground,
      appBar: AppBar(
        backgroundColor: kBackground,
        elevation: 0,
        title: const Text(
          'My Designs',
          style: TextStyle(fontWeight: FontWeight.w700, fontSize: 17),
        ),
      ),
      body: RefreshIndicator(
        color: kGold,
        onRefresh: () async {
          ref.invalidate(myDesignsProvider);
          ref.invalidate(earningsProvider);
        },
        child: myDesigns.when(
          data: (designs) {
            if (designs.isEmpty) {
              return ListView(
                physics: const AlwaysScrollableScrollPhysics(),
                children: const [
                  SizedBox(height: 120),
                  Center(
                    child: Text(
                      'No designs listed yet.\nUpload one to start selling!',
                      textAlign: TextAlign.center,
                      style: TextStyle(color: kMutedForeground, height: 1.5),
                    ),
                  ),
                ],
              );
            }
            return ListView(
              padding: const EdgeInsets.fromLTRB(16, 8, 16, 32),
              physics: const AlwaysScrollableScrollPhysics(),
              children: [
                earnings.maybeWhen(
                  data: (e) => Column(
                    children: [
                      _EarningsSummary(earnings: e),
                      const SizedBox(height: 16),
                      if (e.items.isNotEmpty) ...[
                        _BestSellersChart(items: e.items),
                        const SizedBox(height: 16),
                      ],
                    ],
                  ),
                  orElse: () => const SizedBox.shrink(),
                ),
                for (int i = 0; i < designs.length; i++) ...[
                  if (i > 0) const SizedBox(height: 10),
                  _MyDesignCard(design: designs[i]),
                ],
              ],
            );
          },
          loading: () => const Center(
            child: CircularProgressIndicator(color: kGold, strokeWidth: 2),
          ),
          error: (_, __) => const Center(
            child: Text(
              'Could not load designs.',
              style: TextStyle(color: kMutedForeground),
            ),
          ),
        ),
      ),
    );
  }
}

/// Total earnings + total sales, matching the wallet card's gold-gradient
/// style used on the Market profile tab.
class _EarningsSummary extends StatelessWidget {
  final EarningsModel earnings;
  const _EarningsSummary({required this.earnings});

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        gradient: kGoldGradient,
        borderRadius: BorderRadius.circular(14),
      ),
      child: Row(
        children: [
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                const Text(
                  'Total earnings',
                  style: TextStyle(fontSize: 13, color: Colors.white70),
                ),
                const SizedBox(height: 6),
                Text(
                  earnings.formattedTotal,
                  style: const TextStyle(
                    fontSize: 24,
                    fontWeight: FontWeight.w700,
                    color: Colors.white,
                  ),
                ),
              ],
            ),
          ),
          Container(width: 1, height: 36, color: Colors.white30),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.end,
              children: [
                const Text(
                  'Total sales',
                  style: TextStyle(fontSize: 13, color: Colors.white70),
                ),
                const SizedBox(height: 6),
                Text(
                  '${earnings.totalSales}',
                  style: const TextStyle(
                    fontSize: 24,
                    fontWeight: FontWeight.w700,
                    color: Colors.white,
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

/// Simple horizontal bar chart ranking this seller's own designs by sales —
/// hand-rolled (no chart dependency) since only a handful of bars are ever
/// shown at once.
class _BestSellersChart extends StatelessWidget {
  final List<EarningsItem> items;
  const _BestSellersChart({required this.items});

  static const _maxBars = 6;

  @override
  Widget build(BuildContext context) {
    final sorted = [...items]..sort((a, b) => b.sales.compareTo(a.sales));
    final top = sorted.take(_maxBars).toList();
    final maxSales = top.first.sales == 0 ? 1 : top.first.sales;

    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: kCard,
        borderRadius: BorderRadius.circular(14),
        border: Border.all(color: kBorder),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Text(
            'Best sellers',
            style: TextStyle(
              fontSize: 14,
              fontWeight: FontWeight.w700,
              color: kForeground,
            ),
          ),
          const SizedBox(height: 14),
          for (int i = 0; i < top.length; i++) ...[
            if (i > 0) const SizedBox(height: 10),
            _BarRow(item: top[i], maxSales: maxSales),
          ],
        ],
      ),
    );
  }
}

class _BarRow extends StatelessWidget {
  final EarningsItem item;
  final int maxSales;
  const _BarRow({required this.item, required this.maxSales});

  @override
  Widget build(BuildContext context) {
    final fraction = maxSales == 0 ? 0.0 : item.sales / maxSales;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Expanded(
              child: Text(
                item.title,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: const TextStyle(fontSize: 12.5, color: kForeground),
              ),
            ),
            const SizedBox(width: 8),
            Text(
              '${item.sales} sold',
              style: const TextStyle(
                fontSize: 11,
                fontWeight: FontWeight.w600,
                color: kMutedForeground,
              ),
            ),
          ],
        ),
        const SizedBox(height: 4),
        ClipRRect(
          borderRadius: BorderRadius.circular(4),
          child: LayoutBuilder(
            builder: (context, constraints) => Stack(
              children: [
                Container(height: 8, color: kMuted),
                Container(
                  height: 8,
                  width: constraints.maxWidth * fraction,
                  color: kGold,
                ),
              ],
            ),
          ),
        ),
      ],
    );
  }
}

class _MyDesignCard extends ConsumerWidget {
  final DesignModel design;
  const _MyDesignCard({required this.design});

  Future<void> _confirmDelete(BuildContext context, WidgetRef ref) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Remove design?'),
        content: Text(
          design.salesCount > 0
              ? 'This design has sales — it will be unlisted, but buyers keep their downloads.'
              : 'This will permanently remove the design.',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: const Text('Cancel'),
          ),
          TextButton(
            onPressed: () => Navigator.pop(ctx, true),
            child: const Text('Remove', style: TextStyle(color: kTerracotta)),
          ),
        ],
      ),
    );
    if (confirmed != true) return;
    try {
      await ref.read(marketServiceProvider).deleteDesign(design.id);
      ref.invalidate(myDesignsProvider);
      ref.read(designsProvider.notifier).load();
    } catch (_) {
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Could not remove design.')),
        );
      }
    }
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final desc = design.description.trim();
    return InkWell(
      onTap: () => context.push('/market/design/${design.id}'),
      borderRadius: BorderRadius.circular(14),
      child: Container(
        padding: const EdgeInsets.all(12),
        decoration: BoxDecoration(
          color: kCard,
          borderRadius: BorderRadius.circular(14),
          border: Border.all(color: kBorder),
        ),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            ClipRRect(
              borderRadius: BorderRadius.circular(10),
              child: design.previewUrls.isNotEmpty
                  ? CachedNetworkImage(
                      imageUrl: design.previewUrls.first,
                      width: 88,
                      height: 88,
                      fit: BoxFit.cover,
                    )
                  : Container(
                      width: 88,
                      height: 88,
                      color: kMuted,
                      child: const Icon(
                        Icons.design_services_outlined,
                        size: 28,
                        color: kMutedForeground,
                      ),
                    ),
            ),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Flexible(
                        child: Text(
                          design.title,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: const TextStyle(
                            fontSize: 15,
                            fontWeight: FontWeight.w700,
                            color: kForeground,
                          ),
                        ),
                      ),
                      if (!design.isActive) ...[
                        const SizedBox(width: 6),
                        Container(
                          padding: const EdgeInsets.symmetric(
                            horizontal: 6,
                            vertical: 2,
                          ),
                          decoration: BoxDecoration(
                            color: kMuted,
                            borderRadius: BorderRadius.circular(4),
                          ),
                          child: const Text(
                            'Unlisted',
                            style: TextStyle(
                              fontSize: 9,
                              color: kMutedForeground,
                            ),
                          ),
                        ),
                      ],
                    ],
                  ),
                  if (desc.isNotEmpty) ...[
                    const SizedBox(height: 4),
                    Text(
                      desc,
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(
                        fontSize: 12,
                        color: kMutedForeground,
                        height: 1.35,
                      ),
                    ),
                  ],
                  const SizedBox(height: 8),
                  Text(
                    design.formattedPrice,
                    style: const TextStyle(
                      fontSize: 14,
                      fontWeight: FontWeight.w700,
                      color: kGold,
                    ),
                  ),
                  const SizedBox(height: 4),
                  Text(
                    '${design.salesCount} sold · ${design.viewCount} views',
                    style: const TextStyle(
                      fontSize: 12,
                      color: kMutedForeground,
                    ),
                  ),
                ],
              ),
            ),
            IconButton(
              icon: const Icon(
                Icons.delete_outline,
                size: 20,
                color: kTerracotta,
              ),
              onPressed: () => _confirmDelete(context, ref),
            ),
          ],
        ),
      ),
    );
  }
}
