import 'dart:async';

import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:smooth_page_indicator/smooth_page_indicator.dart';
import 'package:url_launcher/url_launcher.dart';
import '../../../core/theme/app_colors.dart';
import '../../auth/providers/auth_provider.dart';
import '../models/product_model.dart';
import '../models/wallet_model.dart';
import '../providers/products_provider.dart';
import '../providers/my_market_provider.dart';
import '../providers/wallet_provider.dart';
import '../widgets/product_card.dart';
import '../widgets/product_favorite_button.dart';
import 'package:marketkit/core/payments/checkout.dart';

class ProductDetailScreen extends ConsumerStatefulWidget {
  final String productId;
  const ProductDetailScreen({super.key, required this.productId});

  @override
  ConsumerState<ProductDetailScreen> createState() => _ProductDetailScreenState();
}

class _ProductDetailScreenState extends ConsumerState<ProductDetailScreen> {
  final _pageCtrl = PageController();
  Timer? _carouselTimer;
  ProductModel? _product;
  List<ProductModel> _otherProducts = [];
  bool _isLoading = true;
  bool _isPaying = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  @override
  void dispose() {
    _carouselTimer?.cancel();
    _pageCtrl.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    setState(() {
      _isLoading = true;
      _error = null;
    });
    try {
      final product = await ref
          .read(marketServiceProvider)
          .fetchProduct(widget.productId);
      if (!mounted) return;
      setState(() {
        _product = product;
        _isLoading = false;
      });
      _startCarouselTimer();
      _loadOtherProducts(product);
    } catch (_) {
      if (!mounted) return;
      setState(() {
        _isLoading = false;
        _error = 'Could not load this product.';
      });
    }
  }

  /// Advances the preview carousel one photo at a time; a manual swipe just
  /// becomes the new starting point for the next automatic tick.
  void _startCarouselTimer() {
    _carouselTimer?.cancel();
    final count = _product?.previewUrls.length ?? 0;
    if (count <= 1) return;
    _carouselTimer = Timer.periodic(const Duration(seconds: 4), (_) {
      if (!_pageCtrl.hasClients) return;
      final next = ((_pageCtrl.page ?? 0).round() + 1) % count;
      _pageCtrl.animateToPage(
        next,
        duration: const Duration(milliseconds: 500),
        curve: Curves.easeInOut,
      );
    });
  }

  /// "Other products" row — prefers the same category, excludes this product,
  /// capped at 8. Falls back to the general list if the category is empty.
  Future<void> _loadOtherProducts(ProductModel product) async {
    try {
      var results = await ref
          .read(marketServiceProvider)
          .fetchProducts(categoryId: product.categoryId);
      results = results.where((d) => d.id != product.id).toList();
      if (results.isEmpty && product.categoryId != null) {
        results = await ref.read(marketServiceProvider).fetchProducts();
        results = results.where((d) => d.id != product.id).toList();
      }
      if (!mounted) return;
      setState(() => _otherProducts = results.take(8).toList());
    } catch (_) {
      // Non-fatal — the rest of the screen still works without this row.
    }
  }


  void _snack(String msg, {Color? color}) {
    ScaffoldMessenger.of(
      context,
    ).showSnackBar(SnackBar(content: Text(msg), backgroundColor: color));
  }

  Future<void> _buy() async {
    final product = _product;
    if (product == null) return;

    // Wallet first: if the balance covers the price, offer an instant
    // in-wallet purchase; Razorpay stays as the fallback (and the user can
    // still pick it explicitly).
    try {
      final wallet = await ref.read(walletServiceProvider).fetchSummary();
      if (wallet.balanceMinor >= product.priceMinor && mounted) {
        final useWallet = await _confirmWalletPay(product, wallet);
        if (useWallet == null) return; // dialog dismissed — no purchase
        if (useWallet) {
          await _buyWithWallet(product);
          return;
        }
        // false → user chose "Use Razorpay instead"; fall through.
      }
    } catch (_) {
      // Wallet lookup failed — proceed with the normal Razorpay flow.
    }
    if (!mounted) return;

    final auth = ref.read(authProvider);
    setState(() => _isPaying = true);
    try {
      final order = await ref
          .read(marketServiceProvider)
          .createOrder(product.id);
      final result = await CheckoutService.pay(
        order: CheckoutOrder.fromJson(order),
        description: product.title,
        email: auth.user?.email,
        phone: auth.user?.phone,
      );

      _snack('Payment successful! Unlocking your product...', color: kSuccess);

      String purchaseId = '';
      try {
        purchaseId = await ref.read(marketServiceProvider).verifyPurchase(
              razorpayOrderId: result.orderId,
              razorpayPaymentId: result.paymentId,
              razorpaySignature: result.signature,
            );
      } catch (e) {
        // The server webhook can still capture the purchase.
        _snack('Unlock is taking longer than usual: $e', color: kDanger);
      }

      if (!mounted) return;
      ref.invalidate(myPurchasesProvider);
      ref.read(productsProvider.notifier).load();
      if (purchaseId.isNotEmpty) {
        context.push('/market/purchase/$purchaseId/receipt');
      } else {
        await _load();
      }
    } on CheckoutCancelled {
      // User backed out — nothing to report.
    } on CheckoutFailure catch (e) {
      _snack(e.message, color: kDanger);
    } catch (e) {
      _snack('Could not initiate payment: $e', color: kDanger);
    } finally {
      if (mounted) setState(() => _isPaying = false);
    }
  }

  /// null = dismissed, true = pay from wallet, false = use Razorpay.
  Future<bool?> _confirmWalletPay(ProductModel product, WalletSummary wallet) {
    return showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Pay from wallet?'),
        content: Text(
          'Pay ${product.formattedPrice} from your wallet balance of '
          '${wallet.formattedBalance}? The product unlocks instantly.\n\n'
          'This sale is final — no returns or refunds. If there\'s an '
          'issue with the product, you can contact support afterward.',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: const Text('Use Razorpay instead'),
          ),
          ElevatedButton(
            onPressed: () => Navigator.pop(ctx, true),
            style: ElevatedButton.styleFrom(
              backgroundColor: kPrimary,
              foregroundColor: Colors.white,
            ),
            child: const Text('Pay from Wallet'),
          ),
        ],
      ),
    );
  }

  Future<void> _buyWithWallet(ProductModel product) async {
    setState(() => _isPaying = true);
    try {
      final purchase = await ref
          .read(walletServiceProvider)
          .purchaseWithWallet(product.id);
      if (!mounted) return;
      ref.invalidate(myPurchasesProvider);
      ref.invalidate(walletSummaryProvider);
      ref.invalidate(walletTransactionsProvider);
      ref.read(productsProvider.notifier).load();
      context.push('/market/purchase/${purchase.id}/receipt');
    } catch (e) {
      // Covers the balance-changed-underneath race (server returns 400) and
      // anything else — the user can retry, which re-offers Razorpay.
      if (mounted) {
        _snack('Wallet payment failed. Please try again.', color: kDanger);
      }
    } finally {
      if (mounted) setState(() => _isPaying = false);
    }
  }




  Future<void> _download() async {
    final product = _product;
    if (product == null) return;
    try {
      final data = await ref
          .read(marketServiceProvider)
          .fetchDownloadUrl(product.id);
      final url = data['url'] as String?;
      if (url == null) throw Exception('no url');
      await launchUrl(Uri.parse(url), mode: LaunchMode.externalApplication);
    } catch (_) {
      _snack('Download failed. Please try again.', color: kDanger);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: kBackground,
      appBar: AppBar(
        backgroundColor: kBackground,
        title: const Text(
          'Product',
          style: TextStyle(fontWeight: FontWeight.w700, fontSize: 18),
        ),
        centerTitle: false,
        elevation: 0,
      ),
      body: SafeArea(
        child: _isLoading
            ? const Center(
                child: CircularProgressIndicator(color: kPrimary, strokeWidth: 2),
              )
            : _error != null
            ? _errorState()
            : _content(),
      ),
      bottomNavigationBar: _product == null ? null : _bottomBar(),
    );
  }

  Widget _content() {
    final product = _product!;
    final categoriesAsync = ref.watch(categoriesProvider);
    String? categoryName;
    categoriesAsync.whenData((cats) {
      for (final c in cats) {
        if (c.id == product.categoryId) {
          categoryName = c.name;
          break;
        }
      }
    });

    return ListView(
      padding: const EdgeInsets.fromLTRB(16, 8, 16, 24),
      children: [
        // 1. Photo area — auto-rotating carousel with a favorite button
        // overlaid top-right.
        Stack(
          children: [
            ClipRRect(
              borderRadius: BorderRadius.circular(14),
              child: SizedBox(
                height: 260,
                child: product.previewUrls.isEmpty
                    ? Container(
                        color: kMuted,
                        child: const Icon(
                          Icons.design_services_outlined,
                          size: 48,
                          color: kMutedForeground,
                        ),
                      )
                    : PageView(
                        controller: _pageCtrl,
                        children: [
                          for (final url in product.previewUrls)
                            CachedNetworkImage(
                              imageUrl: url,
                              fit: BoxFit.cover,
                              placeholder: (_, __) => Container(color: kMuted),
                              errorWidget: (_, __, ___) => Container(
                                color: kMuted,
                                child: const Icon(
                                  Icons.image_not_supported_outlined,
                                  color: kMutedForeground,
                                ),
                              ),
                            ),
                        ],
                      ),
              ),
            ),
            Positioned(
              top: 10,
              right: 10,
              child: ProductFavoriteButton(product: product),
            ),
          ],
        ),
        if (product.previewUrls.length > 1) ...[
          const SizedBox(height: 10),
          Center(
            child: SmoothPageIndicator(
              controller: _pageCtrl,
              count: product.previewUrls.length,
              effect: const WormEffect(
                dotWidth: 8,
                dotHeight: 8,
                activeDotColor: kPrimary,
                dotColor: kBorder,
              ),
            ),
          ),
        ],
        const SizedBox(height: 18),

        // 2. Product name
        Text(
          product.title,
          style: const TextStyle(
            fontSize: 20,
            fontWeight: FontWeight.w700,
            color: kForeground,
          ),
        ),
        const SizedBox(height: 12),

        // 3. Product price — the buyer-facing gross price, not the seller net.
        Container(
          padding: const EdgeInsets.all(14),
          decoration: BoxDecoration(
            color: kPrimary.withValues(alpha: 0.08),
            borderRadius: BorderRadius.circular(12),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                product.formattedPrice,
                style: const TextStyle(
                  fontSize: 26,
                  fontWeight: FontWeight.w800,
                  color: kPrimary,
                ),
              ),
              const SizedBox(height: 2),
              const Text(
                'Price to buy this product',
                style: TextStyle(fontSize: 12, color: kMutedForeground),
              ),
            ],
          ),
        ),
        const SizedBox(height: 20),

        // 4. Product information
        _sectionTitle('Product information'),
        const SizedBox(height: 8),
        Container(
          decoration: BoxDecoration(
            color: kCard,
            border: Border.all(color: kBorder),
            borderRadius: BorderRadius.circular(12),
          ),
          padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 4),
          child: Column(
            children: [
              _specRow(Icons.description_outlined, 'File format',
                  product.fileFormat.toUpperCase()),
              _specRow(Icons.sd_storage_outlined, 'File size',
                  '${(product.fileSizeBytes / 1024).toStringAsFixed(0)} KB'),
              _specRow(Icons.category_outlined, 'Category',
                  categoryName ?? '—'),
              _specRow(Icons.sell_outlined, 'Sold', '${product.salesCount}'),
              _specRow(
                Icons.person_outline_rounded,
                'Seller',
                product.sellerName.isNotEmpty ? product.sellerName : 'Seller',
                trailing: product.featuredSeller
                    ? const Icon(Icons.workspace_premium,
                        size: 14, color: kPrimary)
                    : null,
                isLast: true,
              ),
            ],
          ),
        ),

        // 5. Product description
        if (product.description.isNotEmpty) ...[
          const SizedBox(height: 20),
          _sectionTitle('Description'),
          const SizedBox(height: 8),
          Text(
            product.description,
            style: const TextStyle(
              fontSize: 14,
              color: kForeground,
              height: 1.5,
            ),
          ),
        ],

        // 6. Other products — horizontally scrollable, same category preferred.
        if (_otherProducts.isNotEmpty) ...[
          const SizedBox(height: 20),
          _sectionTitle('Other products'),
          const SizedBox(height: 10),
          SizedBox(
            height: 210,
            child: ListView.separated(
              scrollDirection: Axis.horizontal,
              itemCount: _otherProducts.length,
              separatorBuilder: (_, __) => const SizedBox(width: 10),
              itemBuilder: (_, i) {
                final d = _otherProducts[i];
                return SizedBox(
                  width: 140,
                  child: ProductCard(
                    product: d,
                    onTap: () => context.push('/market/product/${d.id}'),
                  ),
                );
              },
            ),
          ),
        ],
      ],
    );
  }

  Widget _sectionTitle(String text) => Text(
        text,
        style: const TextStyle(
          fontSize: 15,
          fontWeight: FontWeight.w700,
          color: kForeground,
        ),
      );

  Widget _specRow(
    IconData icon,
    String label,
    String value, {
    Widget? trailing,
    bool isLast = false,
  }) {
    return Container(
      padding: const EdgeInsets.symmetric(vertical: 10),
      decoration: BoxDecoration(
        border: isLast
            ? null
            : const Border(bottom: BorderSide(color: kBorder)),
      ),
      child: Row(
        children: [
          Icon(icon, size: 16, color: kMutedForeground),
          const SizedBox(width: 8),
          Text(label,
              style: const TextStyle(fontSize: 13, color: kMutedForeground)),
          const SizedBox(width: 12),
          // Flexible + ellipsis so a long value (e.g. a long seller name)
          // truncates instead of overflowing the row.
          Expanded(
            child: Text(
              value,
              textAlign: TextAlign.right,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: const TextStyle(
                fontSize: 13,
                fontWeight: FontWeight.w600,
                color: kForeground,
              ),
            ),
          ),
          if (trailing != null) ...[
            const SizedBox(width: 4),
            trailing,
          ],
        ],
      ),
    );
  }

  Widget _bottomBar() {
    final product = _product!;
    final owned = product.isPurchased || product.isMine;
    return Container(
      padding: EdgeInsets.fromLTRB(
        16,
        12,
        16,
        12 + MediaQuery.of(context).padding.bottom,
      ),
      decoration: const BoxDecoration(
        color: kCard,
        border: Border(top: BorderSide(color: kBorder)),
      ),
      child: Row(
        children: [
          Column(
            // Without min, this Column's default MainAxisSize.max expands to
            // fill the bottomNavigationBar slot's loose height (up to the full
            // screen), inflating the whole bar to screen height and leaving
            // the body ListView zero height — the bug that made this page
            // render blank.
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const Text(
                'Price',
                style: TextStyle(fontSize: 11, color: kMutedForeground),
              ),
              Text(
                product.formattedPrice,
                style: const TextStyle(
                  fontSize: 20,
                  fontWeight: FontWeight.w700,
                  color: kPrimary,
                ),
              ),
            ],
          ),
          const SizedBox(width: 16),
          Expanded(
            child: ElevatedButton.icon(
              onPressed: _isPaying ? null : (owned ? _download : _buy),
              icon: Icon(
                owned ? Icons.download_rounded : Icons.shopping_bag_outlined,
                size: 18,
              ),
              label: _isPaying
                  ? const SizedBox(
                      width: 20,
                      height: 20,
                      child: CircularProgressIndicator(
                        color: Colors.white,
                        strokeWidth: 2,
                      ),
                    )
                  : Text(
                      owned ? 'Download' : 'Buy Now',
                      style: const TextStyle(fontWeight: FontWeight.w600),
                    ),
              style: ElevatedButton.styleFrom(
                backgroundColor: owned ? kSuccess : kPrimary,
                foregroundColor: Colors.white,
                padding: const EdgeInsets.symmetric(vertical: 14),
                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(10),
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _errorState() {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          const Icon(Icons.error_outline, size: 48, color: kMutedForeground),
          const SizedBox(height: 12),
          Text(_error!, style: const TextStyle(color: kMutedForeground)),
          const SizedBox(height: 16),
          ElevatedButton(onPressed: _load, child: const Text('Retry')),
        ],
      ),
    );
  }
}
