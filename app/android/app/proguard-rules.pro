# ProGuard rules for Flutter app with Firebase, Riverpod, Dio, and other plugins

# ───────────────────────────────────────────────────────────────────────────────
# General Rules
# ───────────────────────────────────────────────────────────────────────────────

# Keep all Flutter native interfaces
-keep class io.flutter.** { *; }
-keep class io.flutter.embedding.** { *; }

# Keep Flutter app components
-keep class com.designexpress.stitch_craft_learn.** { *; }

# ───────────────────────────────────────────────────────────────────────────────
# Firebase
# ───────────────────────────────────────────────────────────────────────────────
-keep class com.google.firebase.** { *; }
-keep interface com.google.firebase.** { *; }
-keep class com.google.android.gms.** { *; }
-keep interface com.google.android.gms.** { *; }

# Suppress Firebase warnings
-dontwarn com.google.firebase.**
-dontwarn com.google.android.gms.**

# ───────────────────────────────────────────────────────────────────────────────
# Dio Networking Library
# ───────────────────────────────────────────────────────────────────────────────
-keep class io.flutter.plugins.** { *; }
-dontwarn io.flutter.plugins.**

# ───────────────────────────────────────────────────────────────────────────────
# Riverpod State Management
# ───────────────────────────────────────────────────────────────────────────────
# Generated Riverpod code must not be obfuscated
-keep class riverpod_generated.** { *; }
-keep class *.g.dart { *; }

# ───────────────────────────────────────────────────────────────────────────────
# Video Player & Chewie
# ───────────────────────────────────────────────────────────────────────────────
-keep class com.google.android.exoplayer2.** { *; }
-dontwarn com.google.android.exoplayer2.**

# ───────────────────────────────────────────────────────────────────────────────
# Image Picker
# ───────────────────────────────────────────────────────────────────────────────
-keep class com.yalantis.ucrop.** { *; }
-dontwarn com.yalantis.ucrop.**

# ───────────────────────────────────────────────────────────────────────────────
# Razorpay Payment
# ───────────────────────────────────────────────────────────────────────────────
-keep class com.razorpay.** { *; }
-dontwarn com.razorpay.**

# ───────────────────────────────────────────────────────────────────────────────
# Shared Preferences
# ───────────────────────────────────────────────────────────────────────────────
-keep class androidx.security.crypto.** { *; }
-dontwarn androidx.security.crypto.**

# Secure storage (Android Keystore / EncryptedSharedPreferences)
-keep class com.it_nomads.fluttersecurestorage.** { *; }
-dontwarn com.it_nomads.fluttersecurestorage.**

# ───────────────────────────────────────────────────────────────────────────────
# Suppress Common Warnings
# ───────────────────────────────────────────────────────────────────────────────
-dontwarn android.webkit.**
-dontwarn androidx.lifecycle.**
-dontwarn org.conscrypt.**
-dontwarn sun.misc.Unsafe

# Flutter deferred components reference Play Core optionally at runtime
-dontwarn com.google.android.play.core.splitcompat.SplitCompatApplication
-dontwarn com.google.android.play.core.splitinstall.**
-dontwarn com.google.android.play.core.tasks.**

# ───────────────────────────────────────────────────────────────────────────────
# Optimization
# ───────────────────────────────────────────────────────────────────────────────
-optimizationpasses 5
-verbose

# Remove logging in release builds
-assumenosideeffects class android.util.Log {
  public static *** d(...);
  public static *** v(...);
  public static *** i(...);
}
