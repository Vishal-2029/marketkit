import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/theme/app_colors.dart';
import '../models/product_thread_message.dart';
import '../providers/products_provider.dart';

/// Private buyer <-> admin support thread for a purchased product. Only the
/// buyer and admins can see this conversation — the seller has no access.
class PurchaseThreadScreen extends ConsumerStatefulWidget {
  final String productId;
  final String productTitle;

  const PurchaseThreadScreen({
    super.key,
    required this.productId,
    required this.productTitle,
  });

  @override
  ConsumerState<PurchaseThreadScreen> createState() =>
      _PurchaseThreadScreenState();
}

class _PurchaseThreadScreenState extends ConsumerState<PurchaseThreadScreen> {
  final _controller = TextEditingController();
  List<ProductThreadMessage> _messages = [];
  bool _isLoading = true;
  bool _isPosting = false;

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    try {
      final messages =
          await ref.read(marketServiceProvider).fetchProductMessages(widget.productId);
      if (!mounted) return;
      setState(() {
        _messages = messages;
        _isLoading = false;
      });
    } catch (_) {
      if (!mounted) return;
      setState(() => _isLoading = false);
    }
  }

  Future<void> _post() async {
    final content = _controller.text.trim();
    if (content.isEmpty || _isPosting) return;
    setState(() => _isPosting = true);
    try {
      final message = await ref
          .read(marketServiceProvider)
          .postProductMessage(widget.productId, content);
      if (!mounted) return;
      _controller.clear();
      setState(() => _messages = [..._messages, message]);
    } catch (_) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Failed to send message')),
      );
    } finally {
      if (mounted) setState(() => _isPosting = false);
    }
  }

  String _timeAgo(DateTime dt) {
    final diff = DateTime.now().difference(dt);
    if (diff.inSeconds < 60) return 'just now';
    if (diff.inMinutes < 60) return '${diff.inMinutes}m ago';
    if (diff.inHours < 24) return '${diff.inHours}h ago';
    if (diff.inDays < 7) return '${diff.inDays}d ago';
    return '${(diff.inDays / 7).floor()}w ago';
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: kBackground,
      appBar: AppBar(
        backgroundColor: kBackground,
        title: Text(
          widget.productTitle,
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
          style: const TextStyle(fontWeight: FontWeight.w700, fontSize: 16),
        ),
        centerTitle: false,
        elevation: 0,
      ),
      body: SafeArea(
        child: Column(
          children: [
            Container(
              width: double.infinity,
              padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
              color: kGold.withValues(alpha: 0.08),
              child: const Text(
                'Only you and the admin can see this conversation.',
                style: TextStyle(fontSize: 12, color: kMutedForeground),
              ),
            ),
            Expanded(
              child: _isLoading
                  ? const Center(
                      child: CircularProgressIndicator(color: kGold, strokeWidth: 2))
                  : _messages.isEmpty
                      ? const Center(
                          child: Padding(
                            padding: EdgeInsets.all(24),
                            child: Text(
                              'Have a question or an issue with this product?\nAsk here — the admin will respond.',
                              textAlign: TextAlign.center,
                              style: TextStyle(color: kMutedForeground, fontSize: 13),
                            ),
                          ),
                        )
                      : ListView.separated(
                          padding: const EdgeInsets.all(16),
                          itemCount: _messages.length,
                          separatorBuilder: (_, __) => const SizedBox(height: 10),
                          itemBuilder: (_, i) {
                            final m = _messages[i];
                            return Container(
                              padding: const EdgeInsets.symmetric(
                                  horizontal: 10, vertical: 8),
                              decoration: BoxDecoration(
                                color: m.isAdmin
                                    ? kGold.withValues(alpha: 0.08)
                                    : kCard,
                                borderRadius: BorderRadius.circular(10),
                                border: Border.all(color: kBorder),
                              ),
                              child: Column(
                                crossAxisAlignment: CrossAxisAlignment.start,
                                children: [
                                  Row(
                                    children: [
                                      if (m.isAdmin)
                                        const Icon(Icons.support_agent,
                                            size: 14, color: kGold),
                                      if (m.isAdmin) const SizedBox(width: 4),
                                      Text(
                                        m.isAdmin ? '${m.userName} (Admin)' : m.userName,
                                        style: const TextStyle(
                                            fontSize: 12,
                                            fontWeight: FontWeight.w600,
                                            color: kForeground),
                                      ),
                                      const SizedBox(width: 8),
                                      Text(
                                        _timeAgo(m.createdAt),
                                        style: const TextStyle(
                                            fontSize: 11, color: kMutedForeground),
                                      ),
                                    ],
                                  ),
                                  const SizedBox(height: 4),
                                  Text(
                                    m.content,
                                    style: const TextStyle(
                                        fontSize: 13, color: kMutedForeground, height: 1.4),
                                  ),
                                ],
                              ),
                            );
                          },
                        ),
            ),
            Container(
              color: kCard,
              padding: EdgeInsets.fromLTRB(
                  12, 8, 8, 8 + MediaQuery.of(context).viewInsets.bottom),
              child: Row(
                children: [
                  Expanded(
                    child: TextField(
                      controller: _controller,
                      textInputAction: TextInputAction.send,
                      onSubmitted: (_) => _post(),
                      decoration: InputDecoration(
                        hintText: 'Describe your question or issue…',
                        hintStyle:
                            const TextStyle(color: kMutedForeground, fontSize: 13),
                        filled: true,
                        fillColor: kMuted,
                        contentPadding:
                            const EdgeInsets.symmetric(horizontal: 14, vertical: 10),
                        border: OutlineInputBorder(
                          borderRadius: BorderRadius.circular(20),
                          borderSide: BorderSide.none,
                        ),
                      ),
                      style: const TextStyle(fontSize: 13, color: kForeground),
                    ),
                  ),
                  const SizedBox(width: 4),
                  _isPosting
                      ? const SizedBox(
                          width: 40,
                          height: 40,
                          child: Center(
                            child: SizedBox(
                              width: 20,
                              height: 20,
                              child: CircularProgressIndicator(
                                  color: kGold, strokeWidth: 2),
                            ),
                          ),
                        )
                      : IconButton(
                          icon: const Icon(Icons.send_rounded),
                          color: kGold,
                          onPressed: _post,
                          tooltip: 'Send',
                        ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}
