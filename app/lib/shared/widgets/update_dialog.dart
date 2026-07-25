import 'package:dio/dio.dart';
import 'package:flutter/material.dart';

import '../../core/services/update_service.dart';
import '../../core/theme/app_colors.dart';

class UpdateDialog extends StatefulWidget {
  final AppVersionInfo info;
  const UpdateDialog({super.key, required this.info});

  @override
  State<UpdateDialog> createState() => _UpdateDialogState();
}

class _UpdateDialogState extends State<UpdateDialog> {
  double? _progress; // null = idle, 0.0–1.0 = downloading
  bool _installed = false;
  String? _error;
  CancelToken? _cancelToken;

  Future<void> _startDownload() async {
    setState(() {
      _progress = 0;
      _error = null;
    });
    _cancelToken = CancelToken();

    try {
      final path = await UpdateService.instance.downloadApk(
        widget.info.downloadUrl,
        onProgress: (p) {
          if (mounted) setState(() => _progress = p);
        },
        cancelToken: _cancelToken,
      );
      if (!mounted) return;
      setState(() => _installed = true);
      await UpdateService.instance.installApk(path);
      // After installApk() the OS shows the installer UI — nothing more to do here.
    } on DioException catch (e) {
      if (e.type == DioExceptionType.cancel) return;
      if (mounted) {
        setState(() {
          _progress = null;
          _error = 'Download failed. Tap to retry.';
        });
      }
    } catch (_) {
      if (mounted) {
        setState(() {
          _progress = null;
          _error = 'Something went wrong. Tap to retry.';
        });
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final mandatory = widget.info.isMandatory;

    return PopScope(
      canPop: !mandatory && _progress == null,
      child: AlertDialog(
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(16)),
        title: Text('Update Available  •  v${widget.info.versionName}'),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            if (mandatory)
              Container(
                padding:
                    const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                decoration: BoxDecoration(
                  color: kTerracotta.withValues(alpha: 0.1),
                  borderRadius: BorderRadius.circular(6),
                ),
                child: const Text(
                  'This update is required to continue using the app.',
                  style: TextStyle(fontSize: 12, color: kTerracotta),
                ),
              ),
            if (widget.info.releaseNotes.isNotEmpty) ...[
              const SizedBox(height: 10),
              Text(
                widget.info.releaseNotes,
                style: const TextStyle(
                    fontSize: 13, color: kMutedForeground),
              ),
            ],
            if (_progress != null && !_installed) ...[
              const SizedBox(height: 16),
              LinearProgressIndicator(
                value: _progress,
                color: kGold,
                backgroundColor: kMuted,
              ),
              const SizedBox(height: 6),
              Text(
                '${((_progress ?? 0) * 100).toStringAsFixed(0)}%',
                style: const TextStyle(
                    fontSize: 12, color: kMutedForeground),
              ),
            ],
            if (_installed) ...[
              const SizedBox(height: 12),
              const Text(
                'Download complete. Opening installer…',
                style: TextStyle(fontSize: 13, color: kSage),
              ),
            ],
            if (_error != null) ...[
              const SizedBox(height: 8),
              Text(
                _error!,
                style:
                    const TextStyle(color: kTerracotta, fontSize: 13),
              ),
            ],
          ],
        ),
        actions: [
          if (!mandatory && _progress == null && !_installed)
            TextButton(
              onPressed: () => Navigator.of(context).pop(),
              child: const Text('Later'),
            ),
          if (_progress == null || _error != null)
            FilledButton(
              onPressed: _startDownload,
              style: FilledButton.styleFrom(backgroundColor: kGold),
              child: Text(_error != null ? 'Retry' : 'Update Now'),
            ),
        ],
      ),
    );
  }
}
