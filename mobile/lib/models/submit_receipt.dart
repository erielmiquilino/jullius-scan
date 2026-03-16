import 'dart:convert';

import 'package:jullius_scan/models/job.dart';

/// Request body for POST /api/v1/receipts.
class SubmitReceiptRequest {
  final String fiscalUrl;

  const SubmitReceiptRequest({required this.fiscalUrl});

  String toJsonString() => jsonEncode({'fiscal_url': fiscalUrl});
}

/// Response from POST /api/v1/receipts.
class SubmitReceiptResponse {
  final int jobId;
  final JobStatus status;
  final String fiscalUrl;
  final int? receiptId;
  final String message;

  const SubmitReceiptResponse({
    required this.jobId,
    required this.status,
    required this.fiscalUrl,
    this.receiptId,
    this.message = '',
  });

  factory SubmitReceiptResponse.fromJson(Map<String, dynamic> json) {
    return SubmitReceiptResponse(
      jobId: json['job_id'] as int,
      status: JobStatus.fromString(json['status'] as String),
      fiscalUrl: json['fiscal_url'] as String,
      receiptId: json['receipt_id'] as int?,
      message: json['message'] as String? ?? '',
    );
  }

  /// Whether the receipt already existed (idempotent response).
  bool get alreadyExists =>
      status == JobStatus.completed && receiptId != null;
}
