import '../models/design_model.dart';

/// A liked design, stored device-only — mirrors [FavoriteEntry] for videos.
class DesignFavoriteEntry {
  final DesignModel design;
  final DateTime favoritedAt;

  const DesignFavoriteEntry({required this.design, required this.favoritedAt});

  Map<String, dynamic> toJson() => {
        'design': design.toJson(),
        'favorited_at': favoritedAt.toIso8601String(),
      };

  factory DesignFavoriteEntry.fromJson(Map<String, dynamic> json) =>
      DesignFavoriteEntry(
        design: DesignModel.fromJson(json['design'] as Map<String, dynamic>),
        favoritedAt:
            DateTime.tryParse(json['favorited_at'] as String? ?? '') ??
                DateTime.fromMillisecondsSinceEpoch(0),
      );
}
