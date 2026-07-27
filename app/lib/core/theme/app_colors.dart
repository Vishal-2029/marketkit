import 'package:flutter/material.dart';

/// App palette. Rebrand by changing [kPrimary] and [kPrimaryLight]; the rest is
/// neutral and will keep working with any hue.
///
/// These values mirror the admin panel's CSS tokens in `web/src/index.css`
/// (`--primary`, `--background`, …) — change both so the app and panel stay
/// visually consistent.

// ── Brand ───────────────────────────────────────────────────────────────────
const kPrimary = Color(0xFF4F46E5); // indigo 600 — hsl(243, 75%, 59%)
const kPrimaryLight = Color(0xFF818CF8); // indigo 400 — hsl(234, 89%, 74%)
const kAccentSoft = Color(0xFFEEF2FF); // indigo 50  — tinted surfaces
const kAccentStrong = Color(0xFF3730A3); // indigo 800 — text on soft accent

// ── Neutrals ────────────────────────────────────────────────────────────────
const kBackground = Color(0xFFFAFAFA); // hsl(0, 0%, 98%)
const kForeground = Color(0xFF18181B); // zinc 900
const kCard = Color(0xFFFFFFFF);
const kMuted = Color(0xFFF4F4F5); // zinc 100
const kMutedForeground = Color(0xFF71717A); // zinc 500
const kBorder = Color(0xFFE4E4E7); // zinc 200
const kInput = Color(0xFFF4F4F5); // zinc 100

// ── State ───────────────────────────────────────────────────────────────────
const kSuccess = Color(0xFF16A34A); // green 600
const kWarning = Color(0xFFD97706); // amber 600
const kDanger = Color(0xFFDC2626); // red 600

// ── Gradient (top-left → bottom-right) ──────────────────────────────────────
const kPrimaryGradient = LinearGradient(
  begin: Alignment.topLeft,
  end: Alignment.bottomRight,
  colors: [kPrimaryLight, kPrimary],
);
