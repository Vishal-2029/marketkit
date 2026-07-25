import 'dart:io';

import 'package:dio/dio.dart';
import 'package:file_picker/file_picker.dart';
import 'package:flutter/foundation.dart' show kIsWeb;
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:image_picker/image_picker.dart';
import '../../../core/theme/app_colors.dart';
import '../models/product_category_model.dart';
import '../providers/products_provider.dart';
import '../providers/my_market_provider.dart';
import '../providers/wallet_provider.dart';

class UploadProductTab extends ConsumerStatefulWidget {
  final VoidCallback? onUploaded;
  const UploadProductTab({super.key, this.onUploaded});

  @override
  ConsumerState<UploadProductTab> createState() => _UploadProductTabState();
}

class _UploadProductTabState extends ConsumerState<UploadProductTab> {
  final _titleCtrl = TextEditingController();
  final _descriptionCtrl = TextEditingController();
  final _priceCtrl = TextEditingController();
  final _otherCategoryCtrl = TextEditingController();
  final List<XFile> _previews = [];
  PlatformFile? _productFile;
  String? _selectedCategoryId;
  bool _isSubmitting = false;

  static const _maxPreviews = 7;
  static const _maxImageBytes = 5 * 1024 * 1024; // 5 MB
  static const _maxFileBytes = 5 * 1024 * 1024; // 5 MB
  static const _allowedExtensions = [
    // Core stitch formats
    'dst', 'pes', 'exp', 'jef', 'vp3', 'xxx', 'hus', 'pec',
    // Tajima / Barudan / ZSK / Pfaff / Wilcom machine variants
    'dsb', 'dsz', 'tbf', 't01', 't03', 't04', 't05', 't09', 't10', 't15', 'u01',
    // Melco / Janome / Elna / Husqvarna / Pfaff / Singer / Happy / Inbro
    'cnd', 'jpx', 'sew', 'jan', 'emd', 'shv', 'pcs', 'pcd', 'pcq', 'pcm',
    'csd', 'tap', 'inb', 'ksm', 'yng',
    // Wilcom / Hiraoka / Laesser / Saurer / Time & Space / native
    'ess', 'esl', 'dat', 'vep', 'mst', 'sas', 'mjd', 'emb', 'emc',
    // BERNINA / Dahao / template / vector-cutting
    'art', 'art50', 'art60', 'art70', 'art80', 'dhp', 'dha', 'dhe',
    'tpl', 'plt', 'dxf',
    // Document formats (e.g. digitized-product reference sheets / instructions)
    'pdf',
  ];

  @override
  void dispose() {
    _titleCtrl.dispose();
    _descriptionCtrl.dispose();
    _priceCtrl.dispose();
    _otherCategoryCtrl.dispose();
    super.dispose();
  }

  void _snack(String msg) {
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(msg)));
  }

  /// Live "platform fee / you earn" breakdown under the price field. Uses the
  /// same integer floor math as the server (fee = price*pct/100), so what the
  /// seller sees here is exactly what gets credited on a sale.
  Widget _feeBreakdown() {
    final priceRupees = int.tryParse(_priceCtrl.text.trim());
    if (priceRupees == null || priceRupees <= 0) return const SizedBox.shrink();
    final fee = ref.watch(marketFeeProvider).valueOrNull;
    if (fee == null) return const SizedBox.shrink();

    final priceInPaise = priceRupees * 100;
    final feeInPaise = priceInPaise * fee ~/ 100;
    final netInPaise = priceInPaise - feeInPaise;
    String fmt(int paise) => paise % 100 == 0
        ? '₹${paise ~/ 100}'
        : '₹${(paise / 100).toStringAsFixed(2)}';

    return Container(
      margin: const EdgeInsets.only(top: 10),
      padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 11),
      decoration: BoxDecoration(
        color: kGold.withValues(alpha: 0.07),
        borderRadius: BorderRadius.circular(10),
        border: Border.all(color: kGold.withValues(alpha: 0.25)),
      ),
      child: Column(
        children: [
          Row(
            children: [
              Text('Platform fee ($fee%)',
                  style:
                      const TextStyle(fontSize: 12, color: kMutedForeground)),
              const Spacer(),
              Text('− ${fmt(feeInPaise)}',
                  style: const TextStyle(
                      fontSize: 12,
                      fontWeight: FontWeight.w600,
                      color: kTerracotta)),
            ],
          ),
          const SizedBox(height: 5),
          Row(
            children: [
              const Text('You earn per sale',
                  style: TextStyle(
                      fontSize: 13,
                      fontWeight: FontWeight.w600,
                      color: kForeground)),
              const Spacer(),
              Text(fmt(netInPaise),
                  style: const TextStyle(
                      fontSize: 14,
                      fontWeight: FontWeight.w700,
                      color: kSage)),
            ],
          ),
        ],
      ),
    );
  }

  Future<void> _pickProductFile() async {
    final result = await FilePicker.platform.pickFiles(
      type: FileType.custom,
      allowedExtensions: _allowedExtensions,
      withData: kIsWeb,
    );
    if (result == null || result.files.isEmpty || !mounted) return;
    final file = result.files.first;
    if (file.size > _maxFileBytes) {
      _snack('Product file must be under 5 MB');
      return;
    }
    setState(() => _productFile = file);
  }

  Future<void> _pickPreview() async {
    if (_previews.length >= _maxPreviews) return;
    final picked = await ImagePicker().pickImage(
      source: ImageSource.gallery,
      imageQuality: 75,
      maxWidth: 1280,
    );
    if (picked == null || !mounted) return;
    final size = await picked.length();
    if (!mounted) return;
    if (size > _maxImageBytes) {
      _snack('Each preview must be under 5 MB');
      return;
    }
    setState(() => _previews.add(picked));
  }

  Future<void> _submit() async {
    final title = _titleCtrl.text.trim();
    final priceRupees = int.tryParse(_priceCtrl.text.trim());
    if (title.isEmpty || priceRupees == null) {
      _snack('Please fill in title and price');
      return;
    }
    if (priceRupees < 10 || priceRupees > 100000) {
      _snack('Price must be between ₹10 and ₹1,00,000');
      return;
    }
    if (_productFile == null) {
      _snack('Please select your embroidery product file');
      return;
    }
    if (_previews.isEmpty) {
      _snack('Please add at least one preview photo');
      return;
    }
    if (_selectedCategoryId == null) {
      _snack('Please select a category for your product');
      return;
    }
    final categories = ref.read(categoriesProvider).valueOrNull ?? [];
    final isOtherCategory = categories
        .where((c) => c.id == _selectedCategoryId)
        .any((c) => c.isOther);
    final otherText = _otherCategoryCtrl.text.trim();
    if (isOtherCategory && otherText.isEmpty) {
      _snack('Please describe what kind of product this is');
      return;
    }

    setState(() => _isSubmitting = true);
    try {
      await ref.read(marketServiceProvider).uploadProduct(
            title: title,
            description: _descriptionCtrl.text.trim(),
            priceInPaise: priceRupees * 100,
            file: _productFile!,
            previews: _previews,
            categoryId: _selectedCategoryId!,
            categoryOther: isOtherCategory ? otherText : null,
          );
      if (!mounted) return;
      setState(() {
        _titleCtrl.clear();
        _descriptionCtrl.clear();
        _priceCtrl.clear();
        _otherCategoryCtrl.clear();
        _previews.clear();
        _productFile = null;
        _selectedCategoryId = null;
      });
      _snack('Product listed for sale!');
      ref.read(productsProvider.notifier).load();
      ref.invalidate(myProductsProvider);
      widget.onUploaded?.call();
    } on DioException catch (e) {
      if (!mounted) return;
      final statusCode = e.response?.statusCode;
      final msg =
          e.response?.data?['error'] as String? ?? 'Something went wrong';
      _snack(statusCode == 429
          ? "You're uploading too fast — try again in a few minutes"
          : msg);
    } catch (_) {
      if (mounted) _snack('Upload failed. Please try again.');
    } finally {
      if (mounted) setState(() => _isSubmitting = false);
    }
  }

  bool get _isSelectedCategoryOther {
    final categories = ref.read(categoriesProvider).valueOrNull ?? [];
    return categories
        .where((c) => c.id == _selectedCategoryId)
        .any((c) => c.isOther);
  }

  InputDecoration _decoration(String label, {String? hint}) => InputDecoration(
        labelText: label,
        hintText: hint,
        alignLabelWithHint: true,
        filled: true,
        fillColor: kInput,
        border: OutlineInputBorder(
            borderRadius: BorderRadius.circular(10),
            borderSide: const BorderSide(color: kBorder)),
        enabledBorder: OutlineInputBorder(
            borderRadius: BorderRadius.circular(10),
            borderSide: const BorderSide(color: kBorder)),
      );

  @override
  Widget build(BuildContext context) {
    return ListView(
      padding: const EdgeInsets.fromLTRB(16, 12, 16, 100),
      children: [
        const Text(
          'Sell Your Product',
          style: TextStyle(
              fontSize: 17, fontWeight: FontWeight.w700, color: kForeground),
        ),
        const SizedBox(height: 4),
        const Text(
          'Upload your embroidery product and set your price.',
          style: TextStyle(fontSize: 13, color: kMutedForeground),
        ),
        const SizedBox(height: 16),
        TextField(
          controller: _titleCtrl,
          maxLength: 120,
          decoration:
              _decoration('Title', hint: 'e.g. Rose Border Product'),
        ),
        const SizedBox(height: 12),
        TextField(
          controller: _descriptionCtrl,
          maxLength: 1000,
          maxLines: 3,
          decoration: _decoration('Description',
              hint: 'Stitch count, size, colors used…'),
        ),
        const SizedBox(height: 12),
        TextField(
          controller: _priceCtrl,
          keyboardType: TextInputType.number,
          decoration:
              _decoration('Price (₹)', hint: 'Between 10 and 1,00,000'),
          onChanged: (_) => setState(() {}),
        ),
        _feeBreakdown(),
        const SizedBox(height: 12),
        _CategoryPicker(
          selectedCategoryId: _selectedCategoryId,
          onChanged: (id) => setState(() => _selectedCategoryId = id),
        ),
        if (_isSelectedCategoryOther) ...[
          const SizedBox(height: 12),
          TextField(
            controller: _otherCategoryCtrl,
            maxLength: 200,
            decoration: _decoration('What kind of product is this?',
                hint: 'e.g. Kids cartoon appliqué'),
          ),
        ],
        const SizedBox(height: 16),

        // Product file picker
        GestureDetector(
          onTap: _pickProductFile,
          child: Container(
            padding: const EdgeInsets.all(14),
            decoration: BoxDecoration(
              color: kCard,
              borderRadius: BorderRadius.circular(10),
              border: Border.all(color: _productFile != null ? kGold : kBorder),
            ),
            child: Row(
              children: [
                Icon(
                  _productFile != null
                      ? Icons.check_circle_rounded
                      : Icons.attach_file_rounded,
                  color: _productFile != null ? kSage : kMutedForeground,
                ),
                const SizedBox(width: 10),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        _productFile?.name ?? 'Select product file',
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                        style: const TextStyle(
                            fontSize: 14,
                            fontWeight: FontWeight.w600,
                            color: kForeground),
                      ),
                      const SizedBox(height: 2),
                      Text(
                        _productFile != null
                            ? '${(_productFile!.size / 1024).toStringAsFixed(0)} KB'
                            : 'DST, PES, EXP, JEF, VP3, PDF, and 30+ other machine formats · max 5 MB',
                        style: const TextStyle(
                            fontSize: 11, color: kMutedForeground),
                      ),
                    ],
                  ),
                ),
                if (_productFile != null)
                  IconButton(
                    icon: const Icon(Icons.close,
                        size: 18, color: kMutedForeground),
                    onPressed: () => setState(() => _productFile = null),
                  ),
              ],
            ),
          ),
        ),
        const SizedBox(height: 16),

        // Preview images — Wrap so 7 thumbnails don't overflow a phone width
        Wrap(
          spacing: 8,
          runSpacing: 8,
          children: [
            for (int i = 0; i < _previews.length; i++)
              Stack(
                children: [
                  ClipRRect(
                    borderRadius: BorderRadius.circular(10),
                    child: kIsWeb
                        ? Image.network(_previews[i].path,
                            width: 64, height: 64, fit: BoxFit.cover)
                        : Image.file(File(_previews[i].path),
                            width: 64, height: 64, fit: BoxFit.cover),
                  ),
                  Positioned(
                    top: 2,
                    right: 2,
                    child: GestureDetector(
                      onTap: () => setState(() => _previews.removeAt(i)),
                      child: Container(
                        width: 18,
                        height: 18,
                        decoration: const BoxDecoration(
                          color: Colors.black54,
                          shape: BoxShape.circle,
                        ),
                        child: const Icon(Icons.close,
                            size: 12, color: Colors.white),
                      ),
                    ),
                  ),
                ],
              ),
            if (_previews.length < _maxPreviews)
              GestureDetector(
                onTap: _pickPreview,
                child: Container(
                  width: 64,
                  height: 64,
                  decoration: BoxDecoration(
                    color: kMuted,
                    borderRadius: BorderRadius.circular(10),
                    border: Border.all(color: kBorder),
                  ),
                  child: Column(
                    mainAxisAlignment: MainAxisAlignment.center,
                    children: [
                      const Icon(Icons.add_photo_alternate_outlined,
                          color: kMutedForeground, size: 22),
                      const SizedBox(height: 2),
                      Text(
                        '${_previews.length}/$_maxPreviews',
                        style: const TextStyle(
                            fontSize: 10, color: kMutedForeground),
                      ),
                    ],
                  ),
                ),
              ),
          ],
        ),
        const SizedBox(height: 6),
        const Text(
          'Preview photos buyers will see · up to 7 · 5 MB each',
          style: TextStyle(fontSize: 11, color: kMutedForeground),
        ),
        const SizedBox(height: 20),

        SizedBox(
          width: double.infinity,
          child: ElevatedButton(
            onPressed: _isSubmitting ? null : _submit,
            style: ElevatedButton.styleFrom(
              backgroundColor: kGold,
              foregroundColor: Colors.white,
              padding: const EdgeInsets.symmetric(vertical: 14),
              shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(10)),
            ),
            child: _isSubmitting
                ? const SizedBox(
                    width: 20,
                    height: 20,
                    child: CircularProgressIndicator(
                        color: Colors.white, strokeWidth: 2),
                  )
                : const Text('List for Sale',
                    style: TextStyle(fontWeight: FontWeight.w600)),
          ),
        ),
      ],
    );
  }
}

class _CategoryPicker extends ConsumerWidget {
  final String? selectedCategoryId;
  final ValueChanged<String?> onChanged;

  const _CategoryPicker({
    required this.selectedCategoryId,
    required this.onChanged,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final categoriesAsync = ref.watch(categoriesProvider);

    return categoriesAsync.when(
      loading: () => const LinearProgressIndicator(color: kGold),
      error: (_, __) => const Text(
        'Could not load categories.',
        style: TextStyle(fontSize: 12, color: kTerracotta),
      ),
      data: (categories) {
        final sections = buildProductCategorySections(categories);
        final other = categories.where((c) => c.isOther).toList();

        final items = <DropdownMenuItem<String>>[
          for (final section in sections) ...[
            DropdownMenuItem(
              value: section.parent.id,
              child: Text(section.parent.name,
                  style: const TextStyle(fontWeight: FontWeight.w600)),
            ),
            for (final child in section.children)
              DropdownMenuItem(
                value: child.id,
                child: Text('   ${child.name}'),
              ),
          ],
          for (final o in other)
            DropdownMenuItem(value: o.id, child: Text(o.name)),
        ];

        return DropdownButtonFormField<String>(
          value: selectedCategoryId,
          isExpanded: true,
          items: items,
          onChanged: onChanged,
          decoration: InputDecoration(
            labelText: 'Category',
            filled: true,
            fillColor: kInput,
            border: OutlineInputBorder(
                borderRadius: BorderRadius.circular(10),
                borderSide: const BorderSide(color: kBorder)),
            enabledBorder: OutlineInputBorder(
                borderRadius: BorderRadius.circular(10),
                borderSide: const BorderSide(color: kBorder)),
          ),
        );
      },
    );
  }
}
