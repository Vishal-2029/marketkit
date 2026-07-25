class DesignCategoryModel {
  final String id;
  final String? parentId;
  final String name;
  final int displayOrder;
  final bool isOther;
  // Admin-set banner photo for this section's row on the Design Market home
  // page. Null/empty until an admin uploads one — callers fall back to the
  // first design's own preview in that case.
  final String? photoUrl;

  const DesignCategoryModel({
    required this.id,
    this.parentId,
    required this.name,
    required this.displayOrder,
    required this.isOther,
    this.photoUrl,
  });

  factory DesignCategoryModel.fromJson(Map<String, dynamic> json) =>
      DesignCategoryModel(
        id: json['id'] as String? ?? '',
        parentId: json['parent_id'] as String?,
        name: json['name'] as String? ?? '',
        displayOrder: (json['display_order'] as num?)?.toInt() ?? 0,
        isOther: json['is_other'] as bool? ?? false,
        photoUrl: json['photo_url'] as String?,
      );
}

/// A top-level section paired with its sub-sections, built client-side from
/// the flat category list the backend returns.
class DesignCategorySection {
  final DesignCategoryModel parent;
  final List<DesignCategoryModel> children;

  const DesignCategorySection({required this.parent, required this.children});
}

List<DesignCategorySection> buildDesignCategorySections(
    List<DesignCategoryModel> categories) {
  final parents = categories.where((c) => c.parentId == null && !c.isOther).toList()
    ..sort((a, b) => a.displayOrder.compareTo(b.displayOrder));
  return [
    for (final parent in parents)
      DesignCategorySection(
        parent: parent,
        children: categories.where((c) => c.parentId == parent.id).toList()
          ..sort((a, b) => a.displayOrder.compareTo(b.displayOrder)),
      ),
  ];
}
