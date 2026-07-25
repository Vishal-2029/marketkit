import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/network/api_endpoints.dart';
import '../../core/theme/app_colors.dart';
import '../../features/auth/providers/auth_provider.dart';

/// Avatar + name + email header, shared by the Learning and Design Market
/// profile screens so both stay visually identical for free. Identity data
/// itself already comes from the single shared [authProvider].
class ProfileHeader extends ConsumerWidget {
  const ProfileHeader({
    super.key,
    this.onAvatarTap,
    this.busy = false,
    this.showCameraBadge = false,
  });

  final VoidCallback? onAvatarTap;
  final bool busy;
  final bool showCameraBadge;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final user = ref.watch(authProvider).user;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.center,
      children: [
        GestureDetector(
          onTap: busy ? null : onAvatarTap,
          child: Stack(
            children: [
              CircleAvatar(
                radius: 44,
                backgroundColor: kGold,
                child: user?.avatarUrl != null
                    ? ClipOval(
                        child: CachedNetworkImage(
                          imageUrl: user!.avatarUrl!.startsWith('http')
                              ? user.avatarUrl!
                              : '${ApiEndpoints.staticBase}${user.avatarUrl!}',
                          width: 88,
                          height: 88,
                          fit: BoxFit.cover,
                          placeholder: (_, __) => const CircularProgressIndicator(
                              strokeWidth: 2, color: Colors.white),
                          errorWidget: (_, __, ___) => Text(
                            user.name.isNotEmpty ? user.name[0].toUpperCase() : '?',
                            style: const TextStyle(
                                fontSize: 32,
                                fontWeight: FontWeight.w700,
                                color: Colors.white),
                          ),
                        ),
                      )
                    : Text(
                        user != null && user.name.isNotEmpty
                            ? user.name[0].toUpperCase()
                            : '?',
                        style: const TextStyle(
                          fontSize: 32,
                          fontWeight: FontWeight.w700,
                          color: Colors.white,
                        ),
                      ),
              ),
              if (showCameraBadge)
                Positioned(
                  bottom: 0,
                  right: 0,
                  child: Container(
                    width: 26,
                    height: 26,
                    decoration: const BoxDecoration(
                      color: kGold,
                      shape: BoxShape.circle,
                    ),
                    child: busy
                        ? const Padding(
                            padding: EdgeInsets.all(5),
                            child: CircularProgressIndicator(
                                strokeWidth: 2, color: Colors.white),
                          )
                        : const Icon(Icons.camera_alt_rounded, size: 14, color: Colors.white),
                  ),
                ),
            ],
          ),
        ),
        const SizedBox(height: 12),
        Text(
          user?.name ?? '',
          style: const TextStyle(
            fontSize: 20,
            fontWeight: FontWeight.w700,
            color: kForeground,
          ),
        ),
        const SizedBox(height: 4),
        Text(
          user?.email ?? '',
          style: const TextStyle(fontSize: 13, color: kMutedForeground),
        ),
      ],
    );
  }
}
