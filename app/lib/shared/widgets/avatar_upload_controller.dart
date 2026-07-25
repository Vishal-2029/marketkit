import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:image_cropper/image_cropper.dart';
import 'package:image_picker/image_picker.dart';
import '../../core/theme/app_colors.dart';
import '../../features/auth/providers/auth_provider.dart';

/// Shared pick → crop → upload / remove avatar flow for Learning and
/// Product Market profile screens.
mixin AvatarUploadController<T extends ConsumerStatefulWidget> on ConsumerState<T> {
  bool uploadingAvatar = false;
  bool removingAvatar = false;

  bool get avatarBusy => uploadingAvatar || removingAvatar;

  Future<void> pickAndUploadAvatar() async {
    final picked = await ImagePicker().pickImage(
      source: ImageSource.gallery,
      imageQuality: 95,
    );
    if (picked == null || !mounted) return;

    // Locked to a circular square crop so it fits the avatar.
    final cropped = await ImageCropper().cropImage(
      sourcePath: picked.path,
      aspectRatio: const CropAspectRatio(ratioX: 1, ratioY: 1),
      maxWidth: 512,
      maxHeight: 512,
      compressFormat: ImageCompressFormat.jpg,
      compressQuality: 90,
      uiSettings: [
        AndroidUiSettings(
          toolbarTitle: 'Edit Photo',
          toolbarColor: kGold,
          toolbarWidgetColor: Colors.white,
          activeControlsWidgetColor: kGold,
          backgroundColor: Colors.black,
          cropStyle: CropStyle.circle,
          lockAspectRatio: true,
          hideBottomControls: false,
        ),
        IOSUiSettings(
          title: 'Edit Photo',
          cropStyle: CropStyle.circle,
          aspectRatioLockEnabled: true,
          resetAspectRatioEnabled: false,
          aspectRatioPickerButtonHidden: true,
        ),
      ],
    );
    if (cropped == null || !mounted) return;

    await uploadAvatar(XFile(cropped.path));
  }

  Future<void> uploadAvatar(XFile picked) async {
    setState(() => uploadingAvatar = true);
    try {
      await ref.read(authProvider.notifier).uploadAvatar(picked);
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Profile picture updated')),
        );
      }
    } catch (e) {
      if (mounted) {
        var message = 'Failed to upload photo';
        try {
          final data = (e as dynamic).response?.data;
          if (data is Map && data['error'] != null) {
            message = data['error'] as String;
          }
        } catch (_) {}
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(
            content: Text(message),
            action: SnackBarAction(
              label: 'Retry',
              onPressed: () => uploadAvatar(picked),
            ),
          ),
        );
      }
    } finally {
      if (mounted) setState(() => uploadingAvatar = false);
    }
  }

  Future<void> removeAvatar() async {
    setState(() => removingAvatar = true);
    try {
      await ref.read(authProvider.notifier).removeAvatar();
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Profile picture removed')),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Failed to remove photo')),
        );
      }
    } finally {
      if (mounted) setState(() => removingAvatar = false);
    }
  }

  void showAvatarOptions(bool hasAvatar) {
    showModalBottomSheet<void>(
      context: context,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(16)),
      ),
      builder: (ctx) => SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            ListTile(
              leading: const Icon(Icons.photo_camera_rounded),
              title: const Text('Change photo'),
              onTap: () {
                Navigator.pop(ctx);
                pickAndUploadAvatar();
              },
            ),
            if (hasAvatar)
              ListTile(
                leading: const Icon(Icons.delete_outline_rounded, color: Colors.red),
                title: const Text('Remove photo', style: TextStyle(color: Colors.red)),
                onTap: () {
                  Navigator.pop(ctx);
                  removeAvatar();
                },
              ),
            const SizedBox(height: 8),
          ],
        ),
      ),
    );
  }
}
