import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../../core/theme/app_colors.dart';
import '../../../shared/widgets/notifications_bell.dart';
import '../../notifications/providers/notifications_provider.dart';
import '../../market_plans/screens/market_plans_tab.dart';
import '../widgets/payout_details_banner.dart';
import 'all_products_tab.dart';
import 'favorite_products_tab.dart';
import 'market_profile_tab.dart';
import 'upload_product_tab.dart';

class MarketplaceShell extends ConsumerStatefulWidget {
  const MarketplaceShell({super.key});

  @override
  ConsumerState<MarketplaceShell> createState() => _MarketplaceShellState();
}

class _MarketplaceShellState extends ConsumerState<MarketplaceShell> {
  int _index = 0;

  @override
  void initState() {
    super.initState();
    Future.microtask(() => ref.read(notificationsProvider.notifier).load());
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: kBackground,
      appBar: AppBar(
        backgroundColor: kBackground,
        // No back arrow: Product Market is a top-level mode, not a pushed
        // page. The only way back to Learning is the switch in the Profile
        // tab — so a stray auto-added leading arrow must never appear.
        automaticallyImplyLeading: false,
        title: const Text(
          'Product Market',
          style: TextStyle(fontWeight: FontWeight.w700, fontSize: 18),
        ),
        centerTitle: false,
        elevation: 0,
        actions: const [
          NotificationsBellButton(),
          SizedBox(width: 16),
        ],
      ),
      body: PayoutDetailsBanner(
        child: SafeArea(
          child: IndexedStack(
            index: _index,
            children: [
              const AllProductsTab(),
              UploadProductTab(onUploaded: () => setState(() => _index = 4)),
              const MarketPlansTab(),
              const FavoriteProductsTab(),
              MarketProfileTab(onViewPlans: () => setState(() => _index = 2)),
            ],
          ),
        ),
      ),
      bottomNavigationBar: BottomNavigationBar(
        type: BottomNavigationBarType.fixed,
        currentIndex: _index,
        onTap: (i) => setState(() => _index = i),
        items: const [
          BottomNavigationBarItem(
            icon: Icon(Icons.grid_view_outlined),
            activeIcon: Icon(Icons.grid_view_rounded),
            label: 'Product',
          ),
          BottomNavigationBarItem(
            icon: Icon(Icons.file_upload_outlined),
            activeIcon: Icon(Icons.file_upload_rounded),
            label: 'Upload',
          ),
          BottomNavigationBarItem(
            icon: Icon(Icons.workspace_premium_outlined),
            activeIcon: Icon(Icons.workspace_premium),
            label: 'Plans',
          ),
          BottomNavigationBarItem(
            icon: Icon(Icons.star_outline_rounded),
            activeIcon: Icon(Icons.star_rounded),
            label: 'Favorites',
          ),
          BottomNavigationBarItem(
            icon: Icon(Icons.person_outline_rounded),
            activeIcon: Icon(Icons.person_rounded),
            label: 'Profile',
          ),
        ],
      ),
    );
  }
}
