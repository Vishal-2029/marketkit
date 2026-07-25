import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../core/theme/app_colors.dart';
import '../../../shared/widgets/shimmer_loaders.dart';
import '../models/design_category_model.dart';
import '../providers/designs_provider.dart';
import '../widgets/design_card.dart';

class AllDesignsTab extends ConsumerStatefulWidget {
  const AllDesignsTab({super.key});

  @override
  ConsumerState<AllDesignsTab> createState() => _AllDesignsTabState();
}

class _AllDesignsTabState extends ConsumerState<AllDesignsTab> {
  final _searchCtrl = TextEditingController();
  final _searchFocus = FocusNode();
  final _scrollCtrl = ScrollController();

  /// The parent section currently being drilled into (its children shown as
  /// a grid). Null while browsing the top-level sections or after a design
  /// list is reached via search suggestions.
  DesignCategorySection? _selectedSection;

  @override
  void initState() {
    super.initState();
    _scrollCtrl.addListener(_onScroll);
    _searchCtrl.addListener(() => setState(() {}));
    _searchFocus.addListener(() => setState(() {}));
  }

  @override
  void dispose() {
    _searchCtrl.dispose();
    _searchFocus.dispose();
    _scrollCtrl.dispose();
    super.dispose();
  }

  void _onScroll() {
    if (_scrollCtrl.position.pixels >=
        _scrollCtrl.position.maxScrollExtent - 300) {
      ref.read(designsProvider.notifier).loadMore();
    }
  }

  void _selectCategory(DesignCategoryModel category) {
    _searchCtrl.text = category.name;
    _searchFocus.unfocus();
    setState(() => _selectedSection = null);
    ref.read(designsProvider.notifier).setCategory(category.id);
  }

  /// Tapping a top-level section: open it and show every design across all
  /// of its sub-sections at once (the backend expands a parent category id
  /// to all its child leaves). A filter button then lets the user narrow to
  /// a single sub-section.
  void _openSection(DesignCategorySection section) {
    _searchFocus.unfocus();
    _searchCtrl.clear();
    setState(() => _selectedSection = section);
    ref.read(designsProvider.notifier).setCategory(section.parent.id);
  }

  /// Filter the open section's designs to one sub-section, or back to all of
  /// them. Passing null (or the parent itself) means "All".
  void _filterByChild(DesignCategoryModel? child) {
    final section = _selectedSection;
    if (section == null) return;
    _searchFocus.unfocus();
    ref
        .read(designsProvider.notifier)
        .setCategory(child?.id ?? section.parent.id);
  }

  /// Breadcrumb "Design Market" — back to the top-level sections.
  void _backToSections() {
    setState(() => _selectedSection = null);
    _searchCtrl.clear();
    ref.read(designsProvider.notifier).setCategory(null);
  }

  void _clearFilters() {
    _searchCtrl.clear();
    _searchFocus.unfocus();
    setState(() => _selectedSection = null);
    ref.read(designsProvider.notifier).setCategory(null);
  }

  /// Bottom-sheet filter menu: the open section's sub-sections plus "All".
  /// Mirrors the circular filter button in the design mockup.
  void _openFilterSheet() {
    final section = _selectedSection;
    if (section == null || section.children.isEmpty) return;
    final activeId = ref.read(designsProvider).categoryId;

    showModalBottomSheet<void>(
      context: context,
      backgroundColor: kCard,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      builder: (ctx) {
        Widget tile(String label, bool selected, VoidCallback onTap) {
          return ListTile(
            title: Text(
              label,
              style: TextStyle(
                fontSize: 15,
                fontWeight: selected ? FontWeight.w700 : FontWeight.w500,
                color: selected ? kGold : kForeground,
              ),
            ),
            trailing: selected
                ? const Icon(Icons.check_rounded, color: kGold, size: 20)
                : null,
            onTap: () {
              Navigator.pop(ctx);
              onTap();
            },
          );
        }

        final allSelected = activeId == null || activeId == section.parent.id;
        return SafeArea(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              const SizedBox(height: 8),
              Container(
                width: 40,
                height: 4,
                decoration: BoxDecoration(
                  color: kBorder,
                  borderRadius: BorderRadius.circular(2),
                ),
              ),
              Padding(
                padding: const EdgeInsets.fromLTRB(20, 14, 20, 6),
                child: Align(
                  alignment: Alignment.centerLeft,
                  child: Text(
                    section.parent.name,
                    style: const TextStyle(
                      fontSize: 13,
                      fontWeight: FontWeight.w700,
                      color: kMutedForeground,
                    ),
                  ),
                ),
              ),
              Flexible(
                child: SingleChildScrollView(
                  child: Column(
                    children: [
                      tile('All', allSelected, () => _filterByChild(null)),
                      for (final child in section.children)
                        tile(child.name, activeId == child.id,
                            () => _filterByChild(child)),
                    ],
                  ),
                ),
              ),
              const SizedBox(height: 8),
            ],
          ),
        );
      },
    );
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(designsProvider);
    final categoriesAsync = ref.watch(categoriesProvider);
    final query = _searchCtrl.text.trim().toLowerCase();

    final suggestions = query.isEmpty
        ? const <DesignCategoryModel>[]
        : categoriesAsync.maybeWhen(
            data: (cats) => cats
                .where((c) => !c.isOther && c.name.toLowerCase().contains(query))
                .toList(),
            orElse: () => const <DesignCategoryModel>[],
          );
    final showSuggestions =
        _searchFocus.hasFocus && query.isNotEmpty && suggestions.isNotEmpty;

    final isBrowsingSections = state.searchQuery.isEmpty &&
        state.categoryId == null &&
        _selectedSection == null;

    // A specific sub-section is active (not "All") when the current filter is
    // one of the open section's children rather than the parent itself.
    final section = _selectedSection;
    String? crumbChildName;
    if (section != null &&
        state.categoryId != null &&
        state.categoryId != section.parent.id) {
      for (final c in section.children) {
        if (c.id == state.categoryId) {
          crumbChildName = c.name;
          break;
        }
      }
    }

    // The filter button shows whenever a multi-child section is open.
    final showFilterButton =
        section != null && section.children.isNotEmpty && !showSuggestions;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 12, 16, 10),
          child: TextField(
            controller: _searchCtrl,
            focusNode: _searchFocus,
            decoration: InputDecoration(
              hintText: 'Search designs or categories…',
              hintStyle:
                  const TextStyle(color: kMutedForeground, fontSize: 14),
              prefixIcon:
                  const Icon(Icons.search, color: kMutedForeground, size: 20),
              suffixIcon: _searchCtrl.text.isNotEmpty
                  ? IconButton(
                      icon: const Icon(Icons.close,
                          color: kMutedForeground, size: 18),
                      onPressed: _clearFilters,
                    )
                  : null,
              filled: true,
              fillColor: kCard,
              contentPadding: const EdgeInsets.symmetric(vertical: 10),
              border: OutlineInputBorder(
                borderRadius: BorderRadius.circular(10),
                borderSide: BorderSide.none,
              ),
            ),
            onSubmitted: (q) {
              setState(() => _selectedSection = null);
              ref.read(designsProvider.notifier).setSearch(q);
            },
          ),
        ),
        if (!showSuggestions && section != null)
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 0, 16, 10),
            child: _Breadcrumb(
              parentName: section.parent.name,
              childName: crumbChildName,
              onHome: _backToSections,
              onParent: crumbChildName != null ? () => _filterByChild(null) : null,
            ),
          ),
        Expanded(
          child: showSuggestions
              ? _CategorySuggestions(
                  suggestions: suggestions, onSelect: _selectCategory)
              : isBrowsingSections
                  ? _CategorySections(onOpenSection: _openSection)
                  : Stack(
                      children: [
                        _DesignsGrid(state: state, scrollCtrl: _scrollCtrl),
                        if (showFilterButton)
                          Positioned(
                            right: 16,
                            bottom: 16,
                            child: _FilterButton(onTap: _openFilterSheet),
                          ),
                      ],
                    ),
        ),
      ],
    );
  }
}

/// The circular filter button (bottom-right) from the mockup — tapping it
/// opens the sub-section filter sheet.
class _FilterButton extends StatelessWidget {
  final VoidCallback onTap;
  const _FilterButton({required this.onTap});

  @override
  Widget build(BuildContext context) {
    return Material(
      color: kGold,
      shape: const CircleBorder(),
      elevation: 3,
      child: InkWell(
        customBorder: const CircleBorder(),
        onTap: onTap,
        child: const SizedBox(
          width: 56,
          height: 56,
          child: Icon(Icons.tune_rounded, color: Colors.white, size: 26),
        ),
      ),
    );
  }
}

class _CategorySuggestions extends StatelessWidget {
  final List<DesignCategoryModel> suggestions;
  final void Function(DesignCategoryModel) onSelect;

  const _CategorySuggestions({required this.suggestions, required this.onSelect});

  @override
  Widget build(BuildContext context) {
    return ListView.separated(
      padding: const EdgeInsets.symmetric(horizontal: 16),
      itemCount: suggestions.length,
      separatorBuilder: (_, __) => const Divider(height: 1),
      itemBuilder: (_, i) {
        final c = suggestions[i];
        return ListTile(
          leading: const Icon(Icons.sell_outlined, color: kGold, size: 20),
          title: Text(c.name, style: const TextStyle(fontSize: 14)),
          onTap: () => onSelect(c),
        );
      },
    );
  }
}

class _CategorySections extends ConsumerWidget {
  final void Function(DesignCategorySection) onOpenSection;
  const _CategorySections({required this.onOpenSection});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final categoriesAsync = ref.watch(categoriesProvider);

    return categoriesAsync.when(
      loading: () => const ShimmerList(
        itemHeight: 92,
        count: 5,
        padding: EdgeInsets.fromLTRB(16, 12, 16, 16),
      ),
      error: (_, __) => const Center(
        child: Text('Could not load categories.',
            style: TextStyle(color: kMutedForeground)),
      ),
      data: (categories) {
        final sections = buildDesignCategorySections(categories);
        if (sections.isEmpty) {
          return const Center(
            child: Text('No designs for sale yet.\nBe the first to upload one!',
                textAlign: TextAlign.center,
                style: TextStyle(color: kMutedForeground)),
          );
        }
        // Every admin-defined section is always listed, whether or not it
        // has designs yet — sections with none show a placeholder photo
        // until something is uploaded into them.
        return ListView.builder(
          padding: const EdgeInsets.fromLTRB(0, 12, 0, 100),
          itemCount: sections.length,
          itemBuilder: (_, i) => _SectionRow(
            section: sections[i],
            index: i,
            onOpen: () => onOpenSection(sections[i]),
          ),
        );
      },
    );
  }
}

/// One collapsed section row: a full-bleed "photo" tile (the admin-set
/// section photo if one exists, otherwise the first design's own preview,
/// otherwise a placeholder) with a dark gradient scrim washing in from one
/// side and the section name sitting directly on top of it, alternating side
/// down the list. The design photos themselves stay hidden until the section
/// is tapped open — this row never shows more than the one representative
/// photo.
class _SectionRow extends ConsumerWidget {
  final DesignCategorySection section;
  final int index;
  final VoidCallback onOpen;
  const _SectionRow({
    required this.section,
    required this.index,
    required this.onOpen,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    // An admin-set section photo always wins; only fall back to the first
    // design's own preview when the admin hasn't uploaded one.
    String? previewUrl = section.parent.photoUrl;
    if (previewUrl == null || previewUrl.isEmpty) {
      final designsAsync = ref.watch(sectionDesignsProvider(section.parent.id));
      previewUrl = designsAsync.maybeWhen(
        data: (designs) => designs.isNotEmpty && designs.first.previewUrls.isNotEmpty
            ? designs.first.previewUrls.first
            : null,
        orElse: () => null,
      );
    }
    final labelOnLeft = index.isEven;

    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 0, 16, 12),
      child: ClipRRect(
        borderRadius: BorderRadius.circular(20),
        child: Material(
          color: kCard,
          child: InkWell(
            onTap: onOpen,
            child: SizedBox(
              height: 104,
              child: Stack(
                fit: StackFit.expand,
                children: [
                  _SectionPhoto(previewUrl: previewUrl),
                  DecoratedBox(
                    decoration: BoxDecoration(
                      gradient: LinearGradient(
                        begin: labelOnLeft
                            ? Alignment.centerLeft
                            : Alignment.centerRight,
                        end: labelOnLeft
                            ? Alignment.centerRight
                            : Alignment.centerLeft,
                        stops: const [0.0, 0.32],
                        colors: [
                          Colors.black.withValues(alpha: 0.82),
                          Colors.black.withValues(alpha: 0.0),
                        ],
                      ),
                    ),
                  ),
                  Align(
                    alignment: labelOnLeft
                        ? Alignment.centerLeft
                        : Alignment.centerRight,
                    child: Padding(
                      padding: const EdgeInsets.symmetric(horizontal: 20),
                      child: Text(
                        section.parent.name,
                        maxLines: 2,
                        overflow: TextOverflow.ellipsis,
                        textAlign:
                            labelOnLeft ? TextAlign.left : TextAlign.right,
                        style: const TextStyle(
                          fontSize: 16,
                          fontWeight: FontWeight.w800,
                          color: Colors.white,
                          height: 1.2,
                        ),
                      ),
                    ),
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _SectionPhoto extends StatelessWidget {
  final String? previewUrl;
  const _SectionPhoto({required this.previewUrl});

  @override
  Widget build(BuildContext context) {
    return previewUrl != null
        ? CachedNetworkImage(
            imageUrl: previewUrl!,
            fit: BoxFit.cover,
            width: double.infinity,
            height: double.infinity,
          )
        : Container(
            color: kMuted,
            alignment: Alignment.center,
            child: const Icon(
              Icons.image_outlined,
              size: 26,
              color: kMutedForeground,
            ),
          );
  }
}

class _DesignsGrid extends ConsumerWidget {
  final DesignsState state;
  final ScrollController scrollCtrl;
  const _DesignsGrid({required this.state, required this.scrollCtrl});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    if (state.isLoading) {
      return const ShimmerList(
        itemHeight: 180,
        count: 4,
        padding: EdgeInsets.fromLTRB(16, 0, 16, 16),
      );
    }
    if (state.error != null) {
      return Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Icon(Icons.error_outline, size: 48, color: kMutedForeground),
            const SizedBox(height: 12),
            Text(state.error!, style: const TextStyle(color: kMutedForeground)),
            const SizedBox(height: 16),
            ElevatedButton(
              onPressed: () => ref.read(designsProvider.notifier).load(),
              child: const Text('Retry'),
            ),
          ],
        ),
      );
    }
    if (state.designs.isEmpty) {
      final isFiltered = state.searchQuery.isNotEmpty || state.categoryId != null;
      return Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              isFiltered ? Icons.search_off_rounded : Icons.storefront_outlined,
              size: 48,
              color: kMutedForeground,
            ),
            const SizedBox(height: 12),
            Text(
              isFiltered
                  ? 'No designs match this search.'
                  : 'No designs for sale yet.\nBe the first to upload one!',
              textAlign: TextAlign.center,
              style: const TextStyle(color: kMutedForeground),
            ),
          ],
        ),
      );
    }
    return RefreshIndicator(
      color: kGold,
      onRefresh: () => ref.read(designsProvider.notifier).load(),
      child: GridView.builder(
        controller: scrollCtrl,
        padding: const EdgeInsets.fromLTRB(16, 0, 16, 100),
        gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
          crossAxisCount: 2,
          mainAxisSpacing: 12,
          crossAxisSpacing: 12,
          childAspectRatio: 0.8,
        ),
        itemCount: state.designs.length + (state.isLoadingMore ? 2 : 0),
        itemBuilder: (_, i) {
          if (i >= state.designs.length) {
            return const Center(
              child: CircularProgressIndicator(color: kGold, strokeWidth: 2),
            );
          }
          final design = state.designs[i];
          return DesignCard(
            design: design,
            onTap: () => context.push('/market/design/${design.id}'),
          );
        },
      ),
    );
  }
}

/// "Design Market > Parent > Child" trail. Every crumb but the last is
/// tappable and jumps back to that level.
class _Breadcrumb extends StatelessWidget {
  final String? parentName;
  final String? childName;
  final VoidCallback onHome;
  final VoidCallback? onParent;

  const _Breadcrumb({
    this.parentName,
    this.childName,
    required this.onHome,
    this.onParent,
  });

  @override
  Widget build(BuildContext context) {
    final crumbs = <_Crumb>[
      _Crumb('Design Market', onHome),
      if (parentName != null) _Crumb(parentName!, onParent),
      if (childName != null) _Crumb(childName!, null),
    ];
    return Wrap(
      crossAxisAlignment: WrapCrossAlignment.center,
      children: [
        for (var i = 0; i < crumbs.length; i++) ...[
          if (i > 0)
            const Padding(
              padding: EdgeInsets.symmetric(horizontal: 4),
              child:
                  Icon(Icons.chevron_right, size: 14, color: kMutedForeground),
            ),
          GestureDetector(
            onTap: crumbs[i].onTap,
            child: Text(
              crumbs[i].label,
              style: TextStyle(
                fontSize: 12,
                fontWeight: i == crumbs.length - 1
                    ? FontWeight.w700
                    : FontWeight.w600,
                color: i == crumbs.length - 1 ? kGold : kMutedForeground,
              ),
            ),
          ),
        ],
      ],
    );
  }
}

class _Crumb {
  final String label;
  final VoidCallback? onTap;
  const _Crumb(this.label, this.onTap);
}
