import 'dart:async';

import 'package:flutter/material.dart';

import 'package:jullius_scan/features/captcha/captcha_webview_screen.dart';
import 'package:jullius_scan/models/job.dart';
import 'package:jullius_scan/services/api_client.dart';
import 'package:jullius_scan/services/scan_telemetry.dart';

/// Screen that polls a scraping job to completion and handles captcha pauses.
///
/// Navigates to [CaptchaWebViewScreen] when the job enters [JobStatus.awaitingCaptcha],
/// then resumes polling after the user solves the challenge.
class JobTrackingScreen extends StatefulWidget {
  final int jobId;
  final ApiClient apiClient;

  const JobTrackingScreen({
    super.key,
    required this.jobId,
    required this.apiClient,
  });

  @override
  State<JobTrackingScreen> createState() => _JobTrackingScreenState();
}

enum _TrackingState { polling, awaitingCaptcha, completed, failed }

class _JobTrackingScreenState extends State<JobTrackingScreen> {
  _TrackingState _state = _TrackingState.polling;
  Job? _job;
  String? _error;
  bool _disposed = false;

  @override
  void initState() {
    super.initState();
    _startPolling();
  }

  @override
  void dispose() {
    _disposed = true;
    super.dispose();
  }

  Future<void> _startPolling() async {
    await _pollUntilTerminal();
  }

  Future<void> _pollUntilTerminal() async {
    final stopwatch = Stopwatch()..start();
    ScanTelemetry.setStep('job.poll_start');
    ScanTelemetry.log('job.poll_start', {'job_id': widget.jobId});
    try {
      final job = await widget.apiClient.pollJobUntilTerminal(widget.jobId);
      stopwatch.stop();
      if (_disposed) return;

      ScanTelemetry.log('job.poll_terminal', {
        'job_id': job.id,
        'status': job.status.name,
        'attempts': job.attempts,
        'failure_reason': job.failureReason?.name,
        'elapsed_ms': stopwatch.elapsedMilliseconds,
      });

      if (job.status.needsCaptcha) {
        ScanTelemetry.setStep('job.awaiting_captcha');
        setState(() {
          _job = job;
          _state = _TrackingState.awaitingCaptcha;
        });
        await _openCaptchaWebView();
      } else if (job.status == JobStatus.completed) {
        ScanTelemetry.setStep('job.completed');
        setState(() {
          _job = job;
          _state = _TrackingState.completed;
        });
      } else {
        ScanTelemetry.setStep('job.failed');
        setState(() {
          _job = job;
          _state = _TrackingState.failed;
        });
      }
    } on TimeoutException catch (e, stack) {
      stopwatch.stop();
      ScanTelemetry.recordError(
        e,
        stack,
        reason: 'job poll timeout',
        attrs: {
          'job_id': widget.jobId,
          'elapsed_ms': stopwatch.elapsedMilliseconds,
        },
      );
      if (_disposed) return;
      setState(() {
        _error = 'O processamento excedeu o tempo limite.';
        _state = _TrackingState.failed;
      });
    } catch (e, stack) {
      stopwatch.stop();
      ScanTelemetry.recordError(
        e,
        stack,
        reason: 'job poll error',
        attrs: {
          'job_id': widget.jobId,
          'elapsed_ms': stopwatch.elapsedMilliseconds,
        },
      );
      if (_disposed) return;
      setState(() {
        _error = 'Erro inesperado: $e';
        _state = _TrackingState.failed;
      });
    }
  }

  Future<void> _openCaptchaWebView() async {
    if (!mounted) return;

    ScanTelemetry.log('captcha.webview_open', {'job_id': widget.jobId});
    final resolved = await Navigator.of(context).push<bool>(
      MaterialPageRoute(
        builder: (_) => CaptchaWebViewScreen(
          jobId: widget.jobId,
          apiClient: widget.apiClient,
        ),
      ),
    );
    ScanTelemetry.log('captcha.webview_closed', {
      'job_id': widget.jobId,
      'resolved': resolved,
    });

    if (_disposed) return;

    if (resolved == true) {
      // Resume polling — job was re-enqueued by the backend after cookie submission.
      ScanTelemetry.setStep('job.poll_resume');
      setState(() => _state = _TrackingState.polling);
      await _pollUntilTerminal();
    } else {
      // User cancelled; job remains in awaiting_captcha until backend timeout.
      ScanTelemetry.setStep('captcha.cancelled');
      setState(() => _state = _TrackingState.awaitingCaptcha);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Processando Nota')),
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: switch (_state) {
            _TrackingState.polling => _buildPolling(),
            _TrackingState.awaitingCaptcha => _buildAwaitingCaptcha(),
            _TrackingState.completed => _buildCompleted(),
            _TrackingState.failed => _buildFailed(),
          },
        ),
      ),
    );
  }

  Widget _buildPolling() {
    return const Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          SizedBox(height: 64, width: 64, child: CircularProgressIndicator()),
          SizedBox(height: 24),
          Text('Extraindo dados da nota fiscal…'),
        ],
      ),
    );
  }

  Widget _buildAwaitingCaptcha() {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          const Icon(Icons.smart_toy_outlined, size: 72, color: Colors.orange),
          const SizedBox(height: 16),
          Text(
            'Verificação de Segurança',
            style: Theme.of(context).textTheme.titleLarge,
          ),
          const SizedBox(height: 8),
          const Text(
            'A SEFAZ exibiu um desafio de captcha. Toque no botão abaixo para resolver.',
            textAlign: TextAlign.center,
          ),
          const SizedBox(height: 32),
          FilledButton.icon(
            icon: const Icon(Icons.open_in_browser),
            label: const Text('Resolver Captcha'),
            onPressed: _openCaptchaWebView,
          ),
          const SizedBox(height: 12),
          OutlinedButton(
            onPressed: () => Navigator.of(context).pop(false),
            child: const Text('Cancelar'),
          ),
        ],
      ),
    );
  }

  Widget _buildCompleted() {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(Icons.check_circle,
              size: 72, color: Theme.of(context).colorScheme.primary),
          const SizedBox(height: 16),
          Text('Nota Registrada!', style: Theme.of(context).textTheme.titleLarge),
          const SizedBox(height: 32),
          FilledButton(
            onPressed: () => Navigator.of(context).pop(true),
            child: const Text('Ver Cupons'),
          ),
        ],
      ),
    );
  }

  Widget _buildFailed() {
    final reason = _job?.failureReason;
    final isCaptchaRelated = reason == FailureReason.captchaExpired ||
        reason == FailureReason.captchaTimeout;

    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(
            isCaptchaRelated ? Icons.timer_off : Icons.error_outline,
            size: 72,
            color: Theme.of(context).colorScheme.error,
          ),
          const SizedBox(height: 16),
          Text(
            reason?.displayLabel ?? 'Falha na Extração',
            style: Theme.of(context).textTheme.titleLarge,
          ),
          const SizedBox(height: 8),
          Text(
            _failureDescription(reason),
            textAlign: TextAlign.center,
            style: Theme.of(context)
                .textTheme
                .bodyMedium
                ?.copyWith(color: Theme.of(context).colorScheme.onSurfaceVariant),
          ),
          if (_error != null) ...[
            const SizedBox(height: 8),
            Text(_error!, textAlign: TextAlign.center),
          ],
          const SizedBox(height: 32),
          OutlinedButton.icon(
            icon: const Icon(Icons.qr_code_scanner),
            label: const Text('Enviar Novamente'),
            onPressed: () => Navigator.of(context).pop(false),
          ),
        ],
      ),
    );
  }

  String _failureDescription(FailureReason? reason) {
    switch (reason) {
      case FailureReason.captchaExpired:
        return 'A sessão do captcha expirou antes de ser processada. Tente enviar a nota novamente.';
      case FailureReason.captchaTimeout:
        return 'O captcha não foi resolvido dentro do prazo. Envie a nota novamente para uma nova tentativa.';
      case FailureReason.timeout:
        return 'A página da SEFAZ demorou muito para responder. Tente novamente mais tarde.';
      case FailureReason.navigation:
        return 'Não foi possível navegar até a página da SEFAZ. Verifique a URL e tente novamente.';
      case FailureReason.parsing:
        return 'Não foi possível extrair os dados da nota. A estrutura da página pode ter mudado.';
      case FailureReason.captcha:
      case FailureReason.unknown:
      case null:
        return 'Ocorreu um erro inesperado. Tente novamente.';
    }
  }
}
