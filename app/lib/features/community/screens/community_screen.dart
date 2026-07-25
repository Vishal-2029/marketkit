import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../../../core/theme/app_colors.dart';
import '../../../shared/widgets/shimmer_loaders.dart';
import '../../home/widgets/category_pill.dart';
import '../providers/posts_provider.dart';
import '../widgets/create_post_sheet.dart';
import '../widgets/post_card.dart';

class CommunityScreen extends ConsumerStatefulWidget {
  const CommunityScreen({super.key});

  @override
  ConsumerState<CommunityScreen> createState() => _CommunityScreenState();
}

class _CommunityScreenState extends ConsumerState<CommunityScreen> {
  final _searchCtrl = TextEditingController();
  final _scrollCtrl = ScrollController();

  @override
  void initState() {
    super.initState();
    Future.microtask(() => ref.read(postsProvider.notifier).load());
    _scrollCtrl.addListener(_onScroll);
  }

  @override
  void dispose() {
    _searchCtrl.dispose();
    _scrollCtrl.dispose();
    super.dispose();
  }

  void _onScroll() {
    if (_scrollCtrl.position.pixels >=
        _scrollCtrl.position.maxScrollExtent - 300) {
      ref.read(postsProvider.notifier).loadMore();
    }
  }

  void _openCreateSheet() {
    showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      backgroundColor: kCard,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      builder: (_) => const CreatePostSheet(),
    );
  }

  void _openOptionSheet() {
    showModalBottomSheet(
      context: context,
      backgroundColor: kCard,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(20)),
      ),
      builder: (sheetCtx) => SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const SizedBox(height: 12),
            Container(
              width: 36,
              height: 4,
              decoration: BoxDecoration(
                  color: kBorder, borderRadius: BorderRadius.circular(2)),
            ),
            const SizedBox(height: 8),
            ListTile(
              leading: const Icon(Icons.help_outline_rounded, color: kGold),
              title: const Text('Ask a Question',
                  style: TextStyle(fontWeight: FontWeight.w600)),
              subtitle: const Text('Get help from the community',
                  style: TextStyle(fontSize: 12, color: kMutedForeground)),
              onTap: () {
                Navigator.pop(sheetCtx);
                _openCreateSheet();
              },
            ),
            ListTile(
              leading: const Icon(Icons.storefront_outlined, color: kGold),
              title: const Text('Design Market',
                  style: TextStyle(fontWeight: FontWeight.w600)),
              subtitle: const Text('Buy and sell embroidery designs',
                  style: TextStyle(fontSize: 12, color: kMutedForeground)),
              onTap: () {
                Navigator.pop(sheetCtx);
                context.push('/market');
              },
            ),
            const SizedBox(height: 8),
          ],
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final state = ref.watch(postsProvider);

    return Scaffold(
      backgroundColor: kBackground,
      floatingActionButton: FloatingActionButton(
        onPressed: _openOptionSheet,
        backgroundColor: kGold,
        foregroundColor: Colors.white,
        child: const Icon(Icons.add),
      ),
      body: SafeArea(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // Header row
            Padding(
              padding: const EdgeInsets.fromLTRB(16, 16, 16, 10),
              child: Row(
                children: [
                  const Expanded(
                    child: Text(
                      'Community',
                      style: TextStyle(
                        fontSize: 20,
                        fontWeight: FontWeight.w700,
                        color: kForeground,
                      ),
                    ),
                  ),
                  // Sort dropdown
                  PopupMenuButton<PostSort>(
                    initialValue: state.sortBy,
                    onSelected: (s) =>
                        ref.read(postsProvider.notifier).setSort(s),
                    itemBuilder: (_) => [
                      const PopupMenuItem(
                        value: PostSort.newest,
                        child: Row(children: [
                          Icon(Icons.access_time_rounded,
                              size: 16, color: kMutedForeground),
                          SizedBox(width: 8),
                          Text('Newest'),
                        ]),
                      ),
                      const PopupMenuItem(
                        value: PostSort.mostReplied,
                        child: Row(children: [
                          Icon(Icons.forum_outlined,
                              size: 16, color: kMutedForeground),
                          SizedBox(width: 8),
                          Text('Most Replied'),
                        ]),
                      ),
                    ],
                    child: Container(
                      padding: const EdgeInsets.symmetric(
                          horizontal: 10, vertical: 6),
                      decoration: BoxDecoration(
                        color: kCard,
                        borderRadius: BorderRadius.circular(8),
                        border: Border.all(color: kBorder),
                      ),
                      child: Row(
                        mainAxisSize: MainAxisSize.min,
                        children: [
                          Icon(
                            state.sortBy == PostSort.newest
                                ? Icons.access_time_rounded
                                : Icons.forum_outlined,
                            size: 14,
                            color: kGold,
                          ),
                          const SizedBox(width: 4),
                          Text(
                            state.sortBy == PostSort.newest
                                ? 'Newest'
                                : 'Most Replied',
                            style: const TextStyle(
                                fontSize: 12,
                                color: kForeground,
                                fontWeight: FontWeight.w500),
                          ),
                          const SizedBox(width: 2),
                          const Icon(Icons.keyboard_arrow_down_rounded,
                              size: 16, color: kMutedForeground),
                        ],
                      ),
                    ),
                  ),
                ],
              ),
            ),

            // Search bar
            Padding(
              padding: const EdgeInsets.fromLTRB(16, 0, 16, 10),
              child: TextField(
                controller: _searchCtrl,
                decoration: InputDecoration(
                  hintText: 'Search questions…',
                  hintStyle:
                      const TextStyle(color: kMutedForeground, fontSize: 14),
                  prefixIcon: const Icon(Icons.search,
                      color: kMutedForeground, size: 20),
                  suffixIcon: _searchCtrl.text.isNotEmpty
                      ? IconButton(
                          icon: const Icon(Icons.close,
                              color: kMutedForeground, size: 18),
                          onPressed: () {
                            _searchCtrl.clear();
                            ref
                                .read(postsProvider.notifier)
                                .setSearch('');
                          },
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
                onChanged: (q) {
                  ref.read(postsProvider.notifier).setSearch(q);
                },
              ),
            ),

            // Category filter pills
            SizedBox(
              height: 36,
              child: ListView(
                scrollDirection: Axis.horizontal,
                padding: const EdgeInsets.symmetric(horizontal: 16),
                children: [
                  for (final cat in const [
                    ('ALL', 'All'),
                    ('GENERAL', 'General'),
                    ('WILLCOM', 'Wilcom 2006'),
                    ('E4', 'E4'),
                    ('MECAD', 'meCAD'),
                  ]) ...[
                    CategoryPill(
                      label: cat.$2,
                      active: state.activeCategory == cat.$1,
                      onTap: () {
                        _searchCtrl.clear();
                        ref
                            .read(postsProvider.notifier)
                            .setCategory(cat.$1);
                      },
                    ),
                    const SizedBox(width: 8),
                  ],
                ],
              ),
            ),
            const SizedBox(height: 12),

            // Post list
            Expanded(
              child: state.isLoading
                  ? const ShimmerList(
                      itemHeight: 120,
                      count: 5,
                      padding: EdgeInsets.fromLTRB(16, 0, 16, 16),
                    )
                  : state.posts.isEmpty
                      ? _emptyState(state.searchQuery.isNotEmpty)
                      : RefreshIndicator(
                          color: kGold,
                          onRefresh: () =>
                              ref.read(postsProvider.notifier).load(),
                          child: ListView.builder(
                            controller: _scrollCtrl,
                            padding:
                                const EdgeInsets.fromLTRB(16, 0, 16, 100),
                            itemCount: state.posts.length +
                                (state.isLoadingMore ? 1 : 0),
                            itemBuilder: (_, i) {
                              if (i == state.posts.length) {
                                return const Padding(
                                  padding: EdgeInsets.symmetric(vertical: 16),
                                  child: Center(
                                    child: CircularProgressIndicator(
                                        color: kGold, strokeWidth: 2),
                                  ),
                                );
                              }
                              return PostCard(
                                post: state.posts[i],
                                onTap: () => context
                                    .push('/community/${state.posts[i].id}'),
                              );
                            },
                          ),
                        ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _emptyState(bool isSearch) {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(
            isSearch ? Icons.search_off_rounded : Icons.forum_outlined,
            size: 48,
            color: kMutedForeground,
          ),
          const SizedBox(height: 12),
          Text(
            isSearch
                ? 'No questions match your search.'
                : 'No questions yet.\nBe the first to ask!',
            textAlign: TextAlign.center,
            style: const TextStyle(color: kMutedForeground),
          ),
        ],
      ),
    );
  }

}
