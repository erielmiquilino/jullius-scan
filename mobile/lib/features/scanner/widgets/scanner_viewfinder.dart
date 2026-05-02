import 'package:flutter/material.dart';

class ScannerViewfinder extends StatelessWidget {
  final double aspectRatio;
  final double widthFraction;
  final double cornerRadius;

  const ScannerViewfinder({
    super.key,
    required this.aspectRatio,
    this.widthFraction = 0.8,
    this.cornerRadius = 12,
  });

  @override
  Widget build(BuildContext context) {
    return IgnorePointer(
      child: LayoutBuilder(
        builder: (context, constraints) {
          final cutoutWidth = constraints.maxWidth * widthFraction;
          final cutoutHeight = cutoutWidth / aspectRatio;
          return CustomPaint(
            size: Size(constraints.maxWidth, constraints.maxHeight),
            painter: _ViewfinderPainter(
              cutoutSize: Size(cutoutWidth, cutoutHeight),
              cornerRadius: cornerRadius,
            ),
          );
        },
      ),
    );
  }
}

class _ViewfinderPainter extends CustomPainter {
  final Size cutoutSize;
  final double cornerRadius;

  _ViewfinderPainter({required this.cutoutSize, required this.cornerRadius});

  @override
  void paint(Canvas canvas, Size size) {
    final cutoutRect = Rect.fromCenter(
      center: Offset(size.width / 2, size.height / 2),
      width: cutoutSize.width,
      height: cutoutSize.height,
    );
    final cutoutRRect =
        RRect.fromRectAndRadius(cutoutRect, Radius.circular(cornerRadius));

    final mask = Path()
      ..addRect(Rect.fromLTWH(0, 0, size.width, size.height))
      ..addRRect(cutoutRRect)
      ..fillType = PathFillType.evenOdd;
    canvas.drawPath(mask, Paint()..color = Colors.black54);

    final borderPaint = Paint()
      ..color = Colors.white
      ..style = PaintingStyle.stroke
      ..strokeWidth = 2;
    canvas.drawRRect(cutoutRRect, borderPaint);

    _drawCorners(canvas, cutoutRect);
  }

  void _drawCorners(Canvas canvas, Rect rect) {
    const cornerLength = 20.0;
    final paint = Paint()
      ..color = Colors.white
      ..style = PaintingStyle.stroke
      ..strokeWidth = 4
      ..strokeCap = StrokeCap.round;

    void corner(Offset a, Offset b, Offset c) {
      canvas.drawLine(a, b, paint);
      canvas.drawLine(b, c, paint);
    }

    corner(
      rect.topLeft + const Offset(0, cornerLength),
      rect.topLeft,
      rect.topLeft + const Offset(cornerLength, 0),
    );
    corner(
      rect.topRight + const Offset(0, cornerLength),
      rect.topRight,
      rect.topRight + const Offset(-cornerLength, 0),
    );
    corner(
      rect.bottomLeft + const Offset(0, -cornerLength),
      rect.bottomLeft,
      rect.bottomLeft + const Offset(cornerLength, 0),
    );
    corner(
      rect.bottomRight + const Offset(0, -cornerLength),
      rect.bottomRight,
      rect.bottomRight + const Offset(-cornerLength, 0),
    );
  }

  @override
  bool shouldRepaint(covariant _ViewfinderPainter old) =>
      old.cutoutSize != cutoutSize || old.cornerRadius != cornerRadius;
}
