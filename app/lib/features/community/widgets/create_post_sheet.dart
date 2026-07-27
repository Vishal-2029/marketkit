import 'package:marketkit/core/config/feature_catalog.dart';
import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart' show kIsWeb;
import 'package:flutter/material.dart';
import 'dart:io';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:image_picker/image_picker.dart';
import '../../../core/theme/app_colors.dart';
import '../providers/posts_provider.dart';

class CreatePostSheet extends ConsumerStatefulWidget {
  const CreatePostSheet({super.key});

  @override
  ConsumerState<CreatePostSheet> createState() => _CreatePostSheetState();
}

class _CreatePostSheetState extends ConsumerState<CreatePostSheet> {
  final _titleController = TextEditingController();
  final _contentController = TextEditingController();
  String _selectedCategory = 'GENERAL';
  bool _isSubmitting = false;
  final List<XFile> _images = [];
  final List<int> _imageSizes = [];
  int _totalBytes = 0;

  static final _categories = FeatureCatalog.postCategoryKeys;
  static const _maxImages = 3;
  static const _maxImageBytes = 5 * 1024 * 1024; // 5 MB per photo
  static const _maxTotalBytes = 15 * 1024 * 1024; // 15 MB combined

  @override
  void dispose() {
    _titleController.dispose();
    _contentController.dispose();
    super.dispose();
  }

  Future<void> _pickImage() async {
    if (_images.length >= _maxImages) return;
    final picked = await ImagePicker().pickImage(
      source: ImageSource.gallery,
      imageQuality: 75,
      maxWidth: 1280,
    );
    if (picked == null || !mounted) return;
    final size = await picked.length();
    if (!mounted) return;
    if (size > _maxImageBytes) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Each photo must be under 5 MB')),
      );
      return;
    }
    if (_totalBytes + size > _maxTotalBytes) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Total photos must be under 15 MB')),
      );
      return;
    }
    setState(() {
      _images.add(picked);
      _imageSizes.add(size);
      _totalBytes += size;
    });
  }

  void _removeImage(int index) => setState(() {
        _totalBytes -= _imageSizes[index];
        _images.removeAt(index);
        _imageSizes.removeAt(index);
      });

  Future<void> _submit() async {
    final title = _titleController.text.trim();
    final content = _contentController.text.trim();
    if (title.isEmpty || content.isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('Please fill in title and description')),
      );
      return;
    }

    setState(() => _isSubmitting = true);
    try {
      await ref.read(postsProvider.notifier).createPost(
            category: _selectedCategory,
            title: title,
            content: content,
            images: _images,
          );
      if (mounted) {
        Navigator.pop(context);
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Question posted!')),
        );
      }
    } on DioException catch (e) {
      if (!mounted) return;
      final statusCode = e.response?.statusCode;
      final msg = e.response?.data?['error'] as String? ?? 'Something went wrong';
      if (statusCode == 429) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
              content: Text("You're posting too fast — try again in a few minutes")),
        );
      } else {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(
          content: Text(msg),
          action: SnackBarAction(label: 'Retry', onPressed: _submit),
        ));
      }
    } catch (_) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(
          content: const Text('Upload failed. Please try again.'),
          action: SnackBarAction(label: 'Retry', onPressed: _submit),
        ));
      }
    } finally {
      if (mounted) setState(() => _isSubmitting = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: EdgeInsets.only(
        left: 16,
        right: 16,
        top: 20,
        bottom: MediaQuery.of(context).viewInsets.bottom + 20,
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Handle
          Center(
            child: Container(
              width: 36,
              height: 4,
              decoration: BoxDecoration(
                  color: kBorder, borderRadius: BorderRadius.circular(2)),
            ),
          ),
          const SizedBox(height: 16),
          const Text(
            'Ask a Question',
            style: TextStyle(
                fontSize: 17, fontWeight: FontWeight.w700, color: kForeground),
          ),
          const SizedBox(height: 16),

          // Category
          DropdownButtonFormField<String>(
            value: _selectedCategory,
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
            items: _categories
                .map((c) => DropdownMenuItem(value: c, child: Text(c)))
                .toList(),
            onChanged: (v) => setState(() => _selectedCategory = v!),
          ),
          const SizedBox(height: 12),

          // Title
          TextField(
            controller: _titleController,
            maxLength: 120,
            decoration: InputDecoration(
              labelText: 'Title',
              hintText: 'What do you want to ask?',
              filled: true,
              fillColor: kInput,
              border: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(10),
                  borderSide: const BorderSide(color: kBorder)),
              enabledBorder: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(10),
                  borderSide: const BorderSide(color: kBorder)),
            ),
          ),
          const SizedBox(height: 12),

          // Content
          TextField(
            controller: _contentController,
            maxLength: 1000,
            maxLines: 4,
            decoration: InputDecoration(
              labelText: 'Description',
              hintText: 'Describe your question in detail...',
              alignLabelWithHint: true,
              filled: true,
              fillColor: kInput,
              border: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(10),
                  borderSide: const BorderSide(color: kBorder)),
              enabledBorder: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(10),
                  borderSide: const BorderSide(color: kBorder)),
            ),
          ),
          const SizedBox(height: 12),

          // Image picker row
          Row(
            children: [
              // Thumbnails for selected images
              for (int i = 0; i < _images.length; i++) ...[
                _ImageThumb(
                  xfile: _images[i],
                  onRemove: () => _removeImage(i),
                ),
                const SizedBox(width: 8),
              ],
              // Add button (shown if under limit)
              if (_images.length < _maxImages)
                GestureDetector(
                  onTap: _pickImage,
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
                          '${_images.length}/$_maxImages',
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
            'Up to 3 photos · 5 MB each · 15 MB total',
            style: TextStyle(fontSize: 11, color: kMutedForeground),
          ),
          const SizedBox(height: 16),

          SizedBox(
            width: double.infinity,
            child: ElevatedButton(
              onPressed: _isSubmitting ? null : _submit,
              style: ElevatedButton.styleFrom(
                backgroundColor: kPrimary,
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
                  : const Text('Post Question',
                      style: TextStyle(fontWeight: FontWeight.w600)),
            ),
          ),
        ],
      ),
    );
  }
}

class _ImageThumb extends StatelessWidget {
  final XFile xfile;
  final VoidCallback onRemove;
  const _ImageThumb({required this.xfile, required this.onRemove});

  @override
  Widget build(BuildContext context) {
    return Stack(
      children: [
        ClipRRect(
          borderRadius: BorderRadius.circular(10),
          child: kIsWeb
              ? Image.network(xfile.path,
                  width: 64, height: 64, fit: BoxFit.cover)
              : Image.file(File(xfile.path),
                  width: 64, height: 64, fit: BoxFit.cover),
        ),
        Positioned(
          top: 2,
          right: 2,
          child: GestureDetector(
            onTap: onRemove,
            child: Container(
              width: 18,
              height: 18,
              decoration: const BoxDecoration(
                color: Colors.black54,
                shape: BoxShape.circle,
              ),
              child: const Icon(Icons.close, size: 12, color: Colors.white),
            ),
          ),
        ),
      ],
    );
  }
}
