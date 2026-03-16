/// Scraping job status values aligned with backend domain.
enum JobStatus {
  queued,
  processing,
  completed,
  failed;

  static JobStatus fromString(String value) {
    return JobStatus.values.firstWhere(
      (s) => s.name == value,
      orElse: () => JobStatus.queued,
    );
  }

  bool get isTerminal => this == completed || this == failed;
  bool get isPending => this == queued || this == processing;
}

/// Failure reason for failed scraping jobs.
enum FailureReason {
  timeout,
  captcha,
  navigation,
  parsing,
  unknown;

  static FailureReason fromString(String value) {
    return FailureReason.values.firstWhere(
      (r) => r.name == value,
      orElse: () => FailureReason.unknown,
    );
  }

  String get displayLabel {
    switch (this) {
      case FailureReason.timeout:
        return 'Timeout';
      case FailureReason.captcha:
        return 'Captcha Blocked';
      case FailureReason.navigation:
        return 'Navigation Error';
      case FailureReason.parsing:
        return 'Parsing Error';
      case FailureReason.unknown:
        return 'Unknown Error';
    }
  }
}

/// A scraping job returned by GET /api/v1/jobs/{id}.
class Job {
  final int id;
  final int houseId;
  final String fiscalUrl;
  final JobStatus status;
  final int attempts;
  final FailureReason? failureReason;
  final String errorDetail;
  final int? receiptId;
  final DateTime createdAt;
  final DateTime? startedAt;
  final DateTime? completedAt;

  const Job({
    required this.id,
    required this.houseId,
    required this.fiscalUrl,
    required this.status,
    required this.attempts,
    this.failureReason,
    this.errorDetail = '',
    this.receiptId,
    required this.createdAt,
    this.startedAt,
    this.completedAt,
  });

  factory Job.fromJson(Map<String, dynamic> json) {
    return Job(
      id: json['id'] as int,
      houseId: json['house_id'] as int,
      fiscalUrl: json['fiscal_url'] as String,
      status: JobStatus.fromString(json['status'] as String),
      attempts: json['attempts'] as int? ?? 0,
      failureReason: json['failure_reason'] != null
          ? FailureReason.fromString(json['failure_reason'] as String)
          : null,
      errorDetail: json['error_detail'] as String? ?? '',
      receiptId: json['receipt_id'] as int?,
      createdAt: DateTime.parse(json['created_at'] as String),
      startedAt: json['started_at'] != null
          ? DateTime.parse(json['started_at'] as String)
          : null,
      completedAt: json['completed_at'] != null
          ? DateTime.parse(json['completed_at'] as String)
          : null,
    );
  }
}
