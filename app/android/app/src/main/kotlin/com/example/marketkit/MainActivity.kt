package com.example.marketkit

import io.flutter.embedding.android.FlutterFragmentActivity

// flutter_stripe's payment sheet is a Fragment, so the host must be a
// FlutterFragmentActivity. Using plain FlutterActivity compiles fine and then
// crashes at runtime the first time a Stripe sheet is presented.
class MainActivity : FlutterFragmentActivity()
