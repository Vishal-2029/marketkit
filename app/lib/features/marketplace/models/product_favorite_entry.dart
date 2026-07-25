import '../models/product_model.dart';

/// A liked product, stored device-only — mirrors [FavoriteEntry] for videos.
class ProductFavoriteEntry {
  final ProductModel product;
  final DateTime favoritedAt;

  const ProductFavoriteEntry({required this.product, required this.favoritedAt});

  Map<String, dynamic> toJson() => {
        'product': product.toJson(),
        'favorited_at': favoritedAt.toIso8601String(),
      };

  factory ProductFavoriteEntry.fromJson(Map<String, dynamic> json) =>
      ProductFavoriteEntry(
        product: ProductModel.fromJson(json['product'] as Map<String, dynamic>),
        favoritedAt:
            DateTime.tryParse(json['favorited_at'] as String? ?? '') ??
                DateTime.fromMillisecondsSinceEpoch(0),
      );
}
