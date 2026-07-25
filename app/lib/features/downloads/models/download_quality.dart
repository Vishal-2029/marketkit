class DownloadQuality {
  final String quality;
  final int sizeBytes;

  const DownloadQuality({required this.quality, required this.sizeBytes});

  factory DownloadQuality.fromJson(Map<String, dynamic> json) => DownloadQuality(
        quality: json['quality'] as String,
        sizeBytes: json['size_bytes'] as int? ?? 0,
      );
}
