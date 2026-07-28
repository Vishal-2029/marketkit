import 'dart:async';

import 'package:flutter_stripe/flutter_stripe.dart' as stripe_sdk;
import 'package:razorpay_flutter/razorpay_flutter.dart';

import '../config/brand.dart';
import '../config/currency.dart';

/// The `/order` response every payment endpoint returns, in the shape
/// `payments.Checkout` produces on the API side.
class CheckoutOrder {
  final String provider;
  final String orderId;
  final int amountMinor;
  final String currency;
  final String publicKey;
  final String? clientSecret;

  const CheckoutOrder({
    required this.provider,
    required this.orderId,
    required this.amountMinor,
    required this.currency,
    required this.publicKey,
    this.clientSecret,
  });

  factory CheckoutOrder.fromJson(Map<String, dynamic> json) => CheckoutOrder(
        provider: json['provider'] as String? ?? 'razorpay',
        orderId: json['order_id'] as String? ?? '',
        amountMinor: (json['amount_minor'] as num?)?.toInt() ?? 0,
        currency: json['currency'] as String? ?? Currency.code,
        publicKey: json['public_key'] as String? ?? '',
        clientSecret: json['client_secret'] as String?,
      );

  bool get isValid => orderId.isNotEmpty && publicKey.isNotEmpty;
}

/// What the gateway handed back after a successful payment. The API verifies
/// these before crediting anything, so a forged result gets rejected server
/// side — see `payments.VerifyCheckout`.
class CheckoutResult {
  final String orderId;
  final String paymentId;
  final String signature;

  const CheckoutResult({
    required this.orderId,
    required this.paymentId,
    required this.signature,
  });
}

class CheckoutCancelled implements Exception {
  @override
  String toString() => 'Payment cancelled';
}

class CheckoutFailure implements Exception {
  final String message;
  CheckoutFailure(this.message);
  @override
  String toString() => message;
}

/// Drives the payment sheet for whichever gateway the API is configured with.
///
/// Screens call [pay] and await a [CheckoutResult]; they never touch a gateway
/// SDK directly. Adding a gateway means one more branch here, not a change in
/// every checkout screen.
///
/// Stripe requires `Stripe.publishableKey` to be set before the sheet opens;
/// that happens lazily on the first Stripe payment using the key the API
/// returned, so there is nothing to configure at app startup.
class CheckoutService {
  /// Opens the gateway's payment sheet and completes when the user finishes.
  ///
  /// Throws [CheckoutCancelled] if the user backs out and [CheckoutFailure]
  /// for anything else.
  static Future<CheckoutResult> pay({
    required CheckoutOrder order,
    required String description,
    String? email,
    String? phone,
  }) {
    if (!order.isValid) {
      throw CheckoutFailure('Payment setup failed. Please try again.');
    }
    switch (order.provider) {
      case 'stripe':
        return _payStripe(order: order, email: email);
      case 'razorpay':
        return _payRazorpay(
          order: order,
          description: description,
          email: email,
          phone: phone,
        );
      default:
        throw CheckoutFailure('Unsupported payment provider: ${order.provider}');
    }
  }

  // ── Razorpay ───────────────────────────────────────────────────────────────
  // The SDK is callback-based, so the callbacks are bridged onto a Completer.
  static Future<CheckoutResult> _payRazorpay({
    required CheckoutOrder order,
    required String description,
    String? email,
    String? phone,
  }) {
    final completer = Completer<CheckoutResult>();
    final razorpay = Razorpay();

    void finish(FutureOr<void> Function() body) {
      body();
      // Clearing inside the callback would tear down the handler that is
      // currently running, so defer it a tick.
      Future.microtask(razorpay.clear);
    }

    razorpay.on(Razorpay.EVENT_PAYMENT_SUCCESS, (PaymentSuccessResponse r) {
      finish(() {
        if (completer.isCompleted) return;
        if (r.orderId == null || r.paymentId == null || r.signature == null) {
          completer.completeError(
              CheckoutFailure('Payment succeeded but the confirmation was incomplete.'));
          return;
        }
        completer.complete(CheckoutResult(
          orderId: r.orderId!,
          paymentId: r.paymentId!,
          signature: r.signature!,
        ));
      });
    });

    razorpay.on(Razorpay.EVENT_PAYMENT_ERROR, (PaymentFailureResponse r) {
      finish(() {
        if (completer.isCompleted) return;
        if (r.code == Razorpay.PAYMENT_CANCELLED) {
          completer.completeError(CheckoutCancelled());
        } else {
          completer.completeError(
              CheckoutFailure(r.message ?? 'Payment failed. Please try again.'));
        }
      });
    });

    razorpay.on(Razorpay.EVENT_EXTERNAL_WALLET, (ExternalWalletResponse r) {
      // The user left for an external wallet; the webhook is authoritative
      // from here, so stop waiting rather than hanging the UI.
      finish(() {
        if (!completer.isCompleted) completer.completeError(CheckoutCancelled());
      });
    });

    razorpay.open({
      'key': order.publicKey,
      'amount': order.amountMinor,
      'order_id': order.orderId,
      'currency': order.currency,
      'name': Brand.checkoutName,
      'description': description,
      'prefill': {'contact': phone ?? '', 'email': email ?? ''},
    });

    return completer.future;
  }

  // ── Stripe ─────────────────────────────────────────────────────────────────
  static Future<CheckoutResult> _payStripe({
    required CheckoutOrder order,
    String? email,
  }) async {
    final secret = order.clientSecret;
    if (secret == null || secret.isEmpty) {
      throw CheckoutFailure('Payment setup failed: missing client secret.');
    }

    stripe_sdk.Stripe.publishableKey = order.publicKey;
    await stripe_sdk.Stripe.instance.applySettings();

    try {
      await stripe_sdk.Stripe.instance.initPaymentSheet(
        paymentSheetParameters: stripe_sdk.SetupPaymentSheetParameters(
          paymentIntentClientSecret: secret,
          merchantDisplayName: Brand.checkoutName,
          billingDetails: stripe_sdk.BillingDetails(email: email),
        ),
      );
      await stripe_sdk.Stripe.instance.presentPaymentSheet();
    } on stripe_sdk.StripeException catch (e) {
      if (e.error.code == stripe_sdk.FailureCode.Canceled) {
        throw CheckoutCancelled();
      }
      throw CheckoutFailure(e.error.localizedMessage ?? 'Payment failed.');
    }

    // Stripe hands back no signature — the API re-checks the intent's status
    // with Stripe directly, so there is nothing to forward here. The intent id
    // is both the order reference and the payment handle.
    return CheckoutResult(
      orderId: order.orderId,
      paymentId: order.orderId,
      signature: '',
    );
  }
}
