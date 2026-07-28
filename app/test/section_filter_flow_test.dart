import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared_preferences/shared_preferences.dart';

import 'package:marketkit/features/marketplace/models/product_category_model.dart';
import 'package:marketkit/features/marketplace/models/product_model.dart';
import 'package:marketkit/features/marketplace/providers/products_provider.dart';
import 'package:marketkit/features/marketplace/screens/all_products_tab.dart';
import 'package:marketkit/features/marketplace/services/market_service.dart';

ProductModel _product(String id, String title, String catId) =>
    ProductModel.fromJson({
      "id": id, "title": title, "description": "",
      "price_minor": 20000, "file_name": "$id.zip", "file_size_bytes": 0,
      "file_format": "dst", "is_active": true, "sales_count": 0,
      "category_id": catId, "preview_urls": <String>[],
      "seller_name": "Seller", "featured_seller": false,
      "is_mine": false, "is_purchased": false,
    });

final _cats = [
  ProductCategoryModel.fromJson(
      {"id": "p1", "parent_id": null, "name": "Section 1", "display_order": 1, "is_other": false}),
  ProductCategoryModel.fromJson(
      {"id": "c11", "parent_id": "p1", "name": "Section 1.1", "display_order": 1, "is_other": false}),
  ProductCategoryModel.fromJson(
      {"id": "c12", "parent_id": "p1", "name": "Section 1.2", "display_order": 2, "is_other": false}),
];

class FakeMarketService extends MarketService {
  String? lastCategoryId = '<none>';

  @override
  Future<List<ProductCategoryModel>> fetchCategories() async => _cats;

  @override
  Future<List<ProductModel>> fetchProducts(
      {String? search, String? categoryId, int page = 1}) async {
    lastCategoryId = categoryId;
    if (categoryId == 'p1') {
      return [_product('d1', 'All A', 'c11'), _product('d2', 'All B', 'c12')];
    }
    if (categoryId == 'c11') return [_product('d1', 'Only 1.1', 'c11')];
    if (categoryId == 'c12') return [_product('d2', 'Only 1.2', 'c12')];
    return [];
  }
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();
  SharedPreferences.setMockInitialValues({});

  testWidgets('section → all-children grid → filter button → filter sheet',
      (tester) async {
    tester.view.physicalSize = const Size(400, 900);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    final fake = FakeMarketService();
    await tester.pumpWidget(
      ProviderScope(
        overrides: [marketServiceProvider.overrideWithValue(fake)],
        child: const MaterialApp(home: Scaffold(body: AllProductsTab())),
      ),
    );
    for (var i = 0; i < 8; i++) {
      await tester.pump(const Duration(milliseconds: 100));
    }

    // Home shows the parent section row.
    expect(find.text('Section 1'), findsWidgets);

    // Tap the parent section → loads all children's products (parent id p1).
    await tester.tap(find.text('Section 1').first);
    for (var i = 0; i < 8; i++) {
      await tester.pump(const Duration(milliseconds: 100));
    }
    expect(fake.lastCategoryId, 'p1',
        reason: 'opening a parent loads all its children (parent id)');

    // The filter button (tune icon) is now visible.
    expect(find.byIcon(Icons.tune_rounded), findsOneWidget);

    // Tap it → the filter sheet lists All + each sub-section.
    await tester.tap(find.byIcon(Icons.tune_rounded));
    for (var i = 0; i < 8; i++) {
      await tester.pump(const Duration(milliseconds: 100));
    }
    expect(find.text('All'), findsOneWidget);
    expect(find.text('Section 1.1'), findsOneWidget);
    expect(find.text('Section 1.2'), findsOneWidget);

    // Pick Section 1.1 → loads only that child's products.
    await tester.tap(find.text('Section 1.1'));
    for (var i = 0; i < 8; i++) {
      await tester.pump(const Duration(milliseconds: 100));
    }
    expect(fake.lastCategoryId, 'c11',
        reason: 'selecting a sub-section filters to just it');

    expect(tester.takeException(), isNull);
  });
}
