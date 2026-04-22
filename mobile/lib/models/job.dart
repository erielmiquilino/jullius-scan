/// Scraping job status values aligned with backend domain.
enum JobStatus {
  queued,
  processing,
  completed,
  failed,
  awaitingCaptcha,
  unknown;

  static JobStatus fromString(String value) {
    const map = {
      'queued': JobStatus.queued,
      'processing': JobStatus.processing,
      'completed': JobStatus.completed,
      'failed': JobStatus.failed,
      'awaiting_captcha': JobStatus.awaitingCaptcha,
    };
    return map[value] ?? JobStatus.unknown;
  }

  bool get isTerminal =>
      this == completed || this == failed || this == awaitingCaptcha;
  bool get isPending => this == queued || this == processing;
  bool get needsCaptcha => this == awaitingCaptcha;
}

/// Failure reason for failed scraping jobs.
enum FailureReason {
  timeout,
  captcha,
  navigation,
  parsing,
  unknown,
  captchaExpired,
  captchaTimeout;

  static FailureReason fromString(String value) {
    const map = {
      'timeout': FailureReason.timeout,
      'captcha': FailureReason.captcha,
      'navigation': FailureReason.navigation,
      'parsing': FailureReason.parsing,
      'captcha_expired': FailureReason.captchaExpired,
      'captcha_timeout': FailureReason.captchaTimeout,
    };
    return map[value] ?? FailureReason.unknown;
  }

  String get displayLabel {
    switch (this) {
      case FailureReason.timeout:
        return 'Tempo esgotado';
      case FailureReason.captcha:
        return 'Captcha Bloqueado';
      case FailureReason.navigation:
        return 'Erro de Navegação';
      case FailureReason.parsing:
        return 'Erro de Leitura';
      case FailureReason.captchaExpired:
        return 'Sessão Expirada';
      case FailureReason.captchaTimeout:
        return 'Captcha não Resolvido';
      case FailureReason.unknown:
        return 'Erro Desconhecido';
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
  final DateTime? captchaPendingAt;

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
    this.captchaPendingAt,
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
      captchaPendingAt: json['captcha_pending_at'] != null
          ? DateTime.parse(json['captcha_pending_at'] as String)
          : null,
    );
  }
}
