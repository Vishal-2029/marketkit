import 'package:marketkit/core/config/currency.dart';
class ProductModel {
  final String id;
  final String title;
  final String description;
  final int priceMinor;
  final String fileName;
  final String fileFormat;
  final int fileSizeBytes;
  final List<String> previewUrls;
  final String sellerName;
  final bool featuredSeller;
  final bool isMine;
  final bool isPurchased;
  final bool isActive;
  final int salesCount;
  final int viewCount;
  final DateTime? createdAt;
  final String? categoryId;

  const ProductModel({
    required this.id,
    required this.title,
    required this.description,
    required this.priceMinor,
    required this.fileName,
    required this.fileFormat,
    required this.fileSizeBytes,
    required this.previewUrls,
    required this.sellerName,
    this.featuredSeller = false,
    required this.isMine,
    required this.isPurchased,
    required this.isActive,
    required this.salesCount,
    this.viewCount = 0,
    this.createdAt,
    this.categoryId,
  });

  String get formattedPrice => Currency.format(priceMinor);

  factory ProductModel.fromJson(Map<String, dynamic> json) => ProductModel(
        id: json['id'] as String? ?? '',
        title: json['title'] as String? ?? '',
        description: json['description'] as String? ?? '',
        priceMinor: (json['price_minor'] as num?)?.toInt() ?? 0,
        fileName: json['file_name'] as String? ?? '',
        fileFormat: json['file_format'] as String? ?? '',
        fileSizeBytes: (json['file_size_bytes'] as num?)?.toInt() ?? 0,
        previewUrls: (json['preview_urls'] as List<dynamic>? ?? [])
            .map((e) => e.toString())
            .toList(),
        sellerName: json['seller_name'] as String? ?? '',
        featuredSeller: json['featured_seller'] as bool? ?? false,
        isMine: json['is_mine'] as bool? ?? false,
        isPurchased: json['is_purchased'] as bool? ?? false,
        isActive: json['is_active'] as bool? ?? true,
        salesCount: (json['sales_count'] as num?)?.toInt() ?? 0,
        viewCount: (json['view_count'] as num?)?.toInt() ?? 0,
        createdAt: json['created_at'] != null
            ? DateTime.tryParse(json['created_at'] as String)
            : null,
        categoryId: json['category_id'] as String?,
      );

  Map<String, dynamic> toJson() => {
        'id': id,
        'title': title,
        'description': description,
        'price_minor': priceMinor,
        'file_name': fileName,
        'file_format': fileFormat,
        'file_size_bytes': fileSizeBytes,
        'preview_urls': previewUrls,
        'seller_name': sellerName,
        'featured_seller': featuredSeller,
        'is_mine': isMine,
        'is_purchased': isPurchased,
        'is_active': isActive,
        'sales_count': salesCount,
        'view_count': viewCount,
        'created_at': createdAt?.toIso8601String(),
        'category_id': categoryId,
      };
}
