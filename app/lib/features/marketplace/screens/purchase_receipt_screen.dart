import 'dart:io';

import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:path_provider/path_provider.dart';
import '../../../core/theme/app_colors.dart';
import '../models/purchase_model.dart';
import '../providers/designs_provider.dart';

/// Shown immediately after a successful purchase. Auto-downloads the invoice
/// PDF and the design file to disk; status rows reflect progress with retry.
class PurchaseReceiptScreen extends ConsumerStatefulWidget {
  final String purchaseId;

  const PurchaseReceiptScreen({super.key, required this.purchaseId});

  @override
  ConsumerState<PurchaseReceiptScreen> createState() =>
      _PurchaseReceiptScreenState();
}

enum _DlStatus { pending, downloading, done, failed }

class _PurchaseReceiptScreenState extends ConsumerState<PurchaseReceiptScreen> {
  PurchaseModel? _purchase;
  bool _loading = true;
  String? _error;

  _DlStatus _invoiceStatus = _DlStatus.pending;
  _DlStatus _fileStatus = _DlStatus.pending;
  String? _invoiceError;
  String? _fileError;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _bootstrap());
  }

  Future<void> _bootstrap() async {
    try {
      final purchases =
          await ref.read(marketServiceProvider).fetchMyPurchases();
      PurchaseModel? match;
      for (final p in purchases) {
        if (p.id == widget.purchaseId) {
          match = p;
          break;
        }
      }
      if (!mounted) return;
      if (match == null) {
        setState(() {
          _loading = false;
          _error = 'Purchase not found';
        });
        return;
      }
      setState(() {
        _purchase = match;
        _loading = false;
      });
      _startDownloads();
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _loading = false;
        _error = 'Failed to load purchase';
      });
    }
  }

  Future<void> _startDownloads() async {
    await Future.wait([_downloadInvoice(), _downloadDesignFile()]);
  }

  Future<void> _downloadInvoice() async {
    setState(() {
      _invoiceStatus = _DlStatus.downloading;
      _invoiceError = null;
    });
    try {
      await ref
          .read(marketServiceProvider)
          .downloadInvoice(widget.purchaseId);
      if (!mounted) return;
      setState(() => _invoiceStatus = _DlStatus.done);
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _invoiceStatus = _DlStatus.failed;
        _invoiceError = 'Invoice download incomplete — tap to retry';
      });
    }
  }

  Future<void> _downloadDesignFile() async {
    final purchase = _purchase;
    if (purchase == null) return;
    setState(() {
      _fileStatus = _DlStatus.downloading;
      _fileError = null;
    });
    try {
      final service = ref.read(marketServiceProvider);
      final info = await service.fetchDownloadUrl(purchase.designId);
      final url = info['url'] as String?;
      final fileName = (info['file_name'] as String?) ?? 'design-file';
      if (url == null || url.isEmpty) throw Exception('No download URL');

      final dir = await getApplicationDocumentsDirectory();
      final savePath = '${dir.path}/designs/$fileName';
      await Directory('${dir.path}/designs').create(recursive: true);
      await service.downloadFileToPath(url, savePath);
      if (!mounted) return;
      setState(() => _fileStatus = _DlStatus.done);
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _fileStatus = _DlStatus.failed;
        _fileError = 'Design file download incomplete — tap to retry';
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: kBackground,
      appBar: AppBar(
        backgroundColor: kBackground,
        elevation: 0,
        leading: IconButton(
          icon: const Icon(Icons.close_rounded),
          onPressed: () => context.pop(),
        ),
        title: const Text(
          'Purchase Receipt',
          style: TextStyle(fontWeight: FontWeight.w700, fontSize: 17),
        ),
      ),
      body: _loading
          ? const Center(child: CircularProgressIndicator(color: kGold))
          : _error != null
              ? Center(child: Text(_error!, style: const TextStyle(color: kMutedForeground)))
              : _buildBody(),
    );
  }

  Widget _buildBody() {
    final p = _purchase!;
    final design = p.design;
    final preview = design?.previewUrls.isNotEmpty == true
        ? design!.previewUrls.first
        : null;

    return ListView(
      padding: const EdgeInsets.fromLTRB(20, 8, 20, 40),
      children: [
        if (preview != null)
          ClipRRect(
            borderRadius: BorderRadius.circular(12),
            child: AspectRatio(
              aspectRatio: 16 / 10,
              child: CachedNetworkImage(
                imageUrl: preview,
                fit: BoxFit.cover,
                placeholder: (_, __) => Container(color: kMuted),
                errorWidget: (_, __, ___) => Container(
                  color: kMuted,
                  child: const Icon(Icons.image_not_supported_outlined),
                ),
              ),
            ),
          ),
        const SizedBox(height: 20),
        Text(
          design?.title ?? 'Design',
          style: const TextStyle(
            fontSize: 20,
            fontWeight: FontWeight.w700,
            color: kForeground,
          ),
        ),
        if (design != null && design.sellerName.isNotEmpty) ...[
          const SizedBox(height: 6),
          Text(
            'Seller: ${design.sellerName}',
            style: const TextStyle(fontSize: 14, color: kMutedForeground),
          ),
        ],
        const SizedBox(height: 16),
        _infoRow('Amount paid', p.formattedAmount),
        _infoRow(
          'Purchase date',
          p.paidAt != null
              ? '${p.paidAt!.day.toString().padLeft(2, '0')} '
                  '${_month(p.paidAt!.month)} ${p.paidAt!.year}'
              : '—',
        ),
        const SizedBox(height: 20),
        Container(
          padding: const EdgeInsets.all(14),
          decoration: BoxDecoration(
            color: kMuted.withValues(alpha: 0.5),
            borderRadius: BorderRadius.circular(10),
          ),
          child: const Text(
            'This sale is final — no returns or refunds are processed through the app. '
            "If there's an issue with the design you purchased, please contact support "
            "from the design's page instead of requesting a return.",
            style: TextStyle(fontSize: 13, color: kMutedForeground, height: 1.45),
          ),
        ),
        const SizedBox(height: 28),
        const Text(
          'Downloads',
          style: TextStyle(fontSize: 15, fontWeight: FontWeight.w700),
        ),
        const SizedBox(height: 12),
        _statusTile(
          label: 'Invoice',
          status: _invoiceStatus,
          error: _invoiceError,
          onRetry: _downloadInvoice,
        ),
        const SizedBox(height: 8),
        _statusTile(
          label: 'Design file',
          status: _fileStatus,
          error: _fileError,
          onRetry: _downloadDesignFile,
        ),
        const SizedBox(height: 32),
        SizedBox(
          width: double.infinity,
          child: ElevatedButton(
            onPressed: () => context.go('/market'),
            style: ElevatedButton.styleFrom(
              backgroundColor: kGold,
              foregroundColor: Colors.white,
              padding: const EdgeInsets.symmetric(vertical: 14),
              shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(10)),
            ),
            child: const Text('Back to Design Market',
                style: TextStyle(fontWeight: FontWeight.w600)),
          ),
        ),
      ],
    );
  }

  Widget _infoRow(String label, String value) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 8),
      child: Row(
        children: [
          Text(label, style: const TextStyle(fontSize: 14, color: kMutedForeground)),
          const Spacer(),
          Text(value,
              style: const TextStyle(
                  fontSize: 14, fontWeight: FontWeight.w600, color: kForeground)),
        ],
      ),
    );
  }

  Widget _statusTile({
    required String label,
    required _DlStatus status,
    String? error,
    required VoidCallback onRetry,
  }) {
    final done = status == _DlStatus.done;
    final failed = status == _DlStatus.failed;
    final busy = status == _DlStatus.downloading;

    String text;
    if (done) {
      text = '$label downloaded ✓';
    } else if (failed) {
      text = error ?? '$label download incomplete — tap to retry';
    } else if (busy) {
      text = 'Downloading $label…';
    } else {
      text = 'Waiting to download $label…';
    }

    return Material(
      color: done
          ? kSage.withValues(alpha: 0.12)
          : failed
              ? kTerracotta.withValues(alpha: 0.1)
              : kCard,
      borderRadius: BorderRadius.circular(10),
      child: InkWell(
        onTap: failed ? onRetry : null,
        borderRadius: BorderRadius.circular(10),
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 14),
          child: Row(
            children: [
              if (busy)
                const SizedBox(
                  width: 18,
                  height: 18,
                  child: CircularProgressIndicator(strokeWidth: 2, color: kGold),
                )
              else
                Icon(
                  done
                      ? Icons.check_circle_rounded
                      : failed
                          ? Icons.refresh_rounded
                          : Icons.hourglass_empty_rounded,
                  size: 20,
                  color: done
                      ? kSage
                      : failed
                          ? kTerracotta
                          : kMutedForeground,
                ),
              const SizedBox(width: 12),
              Expanded(
                child: Text(
                  text,
                  style: TextStyle(
                    fontSize: 14,
                    fontWeight: FontWeight.w600,
                    color: done
                        ? kSage
                        : failed
                            ? kTerracotta
                            : kForeground,
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  String _month(int m) {
    const names = [
      '', 'Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun',
      'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'
    ];
    return names[m];
  }
}
