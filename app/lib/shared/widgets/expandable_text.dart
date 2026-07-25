import 'package:flutter/material.dart';

/// YouTube-style description text: collapses to [maxLines] with an inline
/// "...more" toggle, expanding to the full text with a "Show less" toggle.
/// Tapping anywhere in the text toggles expand/collapse, matching the
/// YouTube app's description behavior.
class ExpandableText extends StatefulWidget {
  const ExpandableText({
    super.key,
    required this.text,
    this.style,
    this.maxLines = 3,
    this.moreLabel = '...more',
    this.lessLabel = 'Show less',
    this.linkColor,
  });

  final String text;
  final TextStyle? style;
  final int maxLines;
  final String moreLabel;
  final String lessLabel;
  final Color? linkColor;

  @override
  State<ExpandableText> createState() => _ExpandableTextState();
}

class _ExpandableTextState extends State<ExpandableText> {
  bool _expanded = false;

  @override
  Widget build(BuildContext context) {
    if (widget.text.isEmpty) return const SizedBox.shrink();

    final style = widget.style ?? DefaultTextStyle.of(context).style;
    final linkColor = widget.linkColor ?? Theme.of(context).colorScheme.primary;
    final linkStyle = style.copyWith(color: linkColor, fontWeight: FontWeight.w700);
    final textScaler = MediaQuery.textScalerOf(context);

    return LayoutBuilder(
      builder: (context, constraints) {
        final maxWidth = constraints.maxWidth;

        final fullPainter = TextPainter(
          text: TextSpan(text: widget.text, style: style),
          maxLines: widget.maxLines,
          textDirection: TextDirection.ltr,
          textScaler: textScaler,
        )..layout(maxWidth: maxWidth);

        if (!fullPainter.didExceedMaxLines) {
          return Text(widget.text, style: style);
        }

        if (_expanded) {
          return GestureDetector(
            behavior: HitTestBehavior.opaque,
            onTap: () => setState(() => _expanded = false),
            child: Text.rich(
              TextSpan(
                style: style,
                children: [
                  TextSpan(text: widget.text),
                  const TextSpan(text: '  '),
                  TextSpan(text: widget.lessLabel, style: linkStyle),
                ],
              ),
            ),
          );
        }

        final truncated = _buildTruncatedSpan(
          text: widget.text,
          style: style,
          linkStyle: linkStyle,
          maxWidth: maxWidth,
          maxLines: widget.maxLines,
          textScaler: textScaler,
        );

        return GestureDetector(
          behavior: HitTestBehavior.opaque,
          onTap: () => setState(() => _expanded = true),
          child: Text.rich(truncated),
        );
      },
    );
  }

  /// Binary-searches the longest prefix of [text] that, together with an
  /// ellipsis and the "more" label, still fits within [maxLines].
  TextSpan _buildTruncatedSpan({
    required String text,
    required TextStyle style,
    required TextStyle linkStyle,
    required double maxWidth,
    required int maxLines,
    required TextScaler textScaler,
  }) {
    int low = 0;
    int high = text.length;
    String best = '';

    bool fits(String candidate) {
      final painter = TextPainter(
        text: TextSpan(
          style: style,
          children: [
            TextSpan(text: '$candidate… '),
            TextSpan(text: widget.moreLabel, style: linkStyle),
          ],
        ),
        maxLines: maxLines,
        textDirection: TextDirection.ltr,
        textScaler: textScaler,
      )..layout(maxWidth: maxWidth);
      return !painter.didExceedMaxLines;
    }

    while (low <= high) {
      final mid = (low + high) ~/ 2;
      final candidate = text.substring(0, mid).trimRight();
      if (fits(candidate)) {
        best = candidate;
        low = mid + 1;
      } else {
        high = mid - 1;
      }
    }

    return TextSpan(
      style: style,
      children: [
        TextSpan(text: '$best… '),
        TextSpan(text: widget.moreLabel, style: linkStyle),
      ],
    );
  }
}
