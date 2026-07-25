import 'package:flutter/material.dart';

// Primary palette — mirrors the React reference project (stitch-craft-learn)
const kGold = Color(0xFFBF9B6F); // hsl(37, 38%, 60%)
const kGoldLight = Color(0xFFCFB48A); // lighter gold for gradients
const kCream = Color(0xFFDDC9AA); // hsl(34, 38%, 83%)
const kSage = Color(0xFF7FAD8B); // hsl(140, 22%, 58%)
const kTerracotta = Color(0xFFBB7365); // hsl(11, 38%, 58%)

// Neutral palette
const kBackground = Color(0xFFF9F7F3); // hsl(50, 20%, 97.6%)
const kForeground = Color(0xFF1C1C22); // hsl(240, 5%, 11.8%)
const kCard = Color(0xFFFFFFFF);
const kMuted = Color(0xFFF2F0EC); // hsl(40, 7%, 95%)
const kMutedForeground = Color(0xFF8C8780);
const kBorder = Color(0xFFEBEBEB);
const kInput = Color(0xFFF2F0EC);

// State colours
const kSuccess = kSage;
const kDanger = kTerracotta;

// Gold gradient (top-left → bottom-right)
const kGoldGradient = LinearGradient(
  begin: Alignment.topLeft,
  end: Alignment.bottomRight,
  colors: [kGoldLight, kGold],
);
