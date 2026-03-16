import 'dart:convert';

/// Standard error response from the API.
class ApiError implements Exception {
  final String message;
  final String code;
  final int statusCode;

  const ApiError({
    required this.message,
    required this.code,
    required this.statusCode,
  });

  factory ApiError.fromResponse(int statusCode, String body) {
    try {
      final json = jsonDecode(body) as Map<String, dynamic>;
      return ApiError(
        message: json['error'] as String? ?? 'Unknown error',
        code: json['code'] as String? ?? 'UNKNOWN',
        statusCode: statusCode,
      );
    } catch (_) {
      return ApiError(
        message: body.isNotEmpty ? body : 'Unknown error',
        code: 'UNKNOWN',
        statusCode: statusCode,
      );
    }
  }

  bool get isUnauthorized => statusCode == 401;
  bool get isForbidden => statusCode == 403;
  bool get isNotFound => statusCode == 404;
  bool get isNotProvisioned => code == 'NO_HOUSE' || isForbidden;

  @override
  String toString() => 'ApiError($statusCode $code): $message';
}
