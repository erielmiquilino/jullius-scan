import 'dart:async';

import 'package:flutter/material.dart';

import 'package:jullius_scan/models/api_error.dart';
import 'package:jullius_scan/models/job.dart';
import 'package:jullius_scan/models/submit_receipt.dart';
import 'package:jullius_scan/services/api_client.dart';

/// Screen for submitting a fiscal URL and tracking the scraping job.
///
/// The entry path is manual URL input (QR Code URL pasted as text).
/// Camera/QR scanner integration is planned for a future iteration.
class SubmitReceiptScreen extends StatefulWidget {
  final ApiClient apiClient;

  const SubmitReceiptScreen({super.key, required this.apiClient});

  @override
  State<SubmitReceiptScreen> createState() => _SubmitReceiptScreenState();
}

enum _ScreenState { input, submitting, polling, success, failed }

class _SubmitReceiptScreenState extends State<SubmitReceiptScreen> {
  final _urlController = TextEditingController();
  final _formKey = GlobalKey<FormState>();

  _ScreenState _state = _ScreenState.input;
  SubmitReceiptResponse? _submitResponse;
  Job? _finalJob;
  String? _error;

  Future<void> _submit() async {
    if (!_formKey.currentState!.validate()) return;

    setState(() {
      _state = _ScreenState.submitting;
      _error = null;
    });

    try {
      final response = await widget.apiClient.submitReceipt(
        _urlController.text.trim(),
      );

      setState(() => _submitResponse = response);

      // If the receipt already exists (idempotent), go straight to success
      if (response.alreadyExists) {
        setState(() => _state = _ScreenState.success);
        return;
      }

      // Poll the job until terminal
      setState(() => _state = _ScreenState.polling);

      final job = await widget.apiClient.pollJobUntilTerminal(response.jobId);

      setState(() {
        _finalJob = job;
        _state = job.status == JobStatus.completed
            ? _ScreenState.success
            : _ScreenState.failed;
      });
    } on ApiError catch (e) {
      setState(() {
        _error = e.message;
        _state = _ScreenState.failed;
      });
    } on TimeoutException {
      setState(() {
        _error = 'The scraping job timed out. The server may still be processing it.';
        _state = _ScreenState.failed;
        _finalJob = Job(
          id: _submitResponse?.jobId ?? 0,
          houseId: 0,
          fiscalUrl: _urlController.text.trim(),
          status: JobStatus.failed,
          attempts: 0,
          failureReason: FailureReason.timeout,
          createdAt: DateTime.now(),
        );
      });
    } catch (e) {
      setState(() {
        _error = 'An unexpected error occurred. Please try again.';
        _state = _ScreenState.failed;
      });
    }
  }

  void _reset() {
    setState(() {
      _state = _ScreenState.input;
      _submitResponse = null;
      _finalJob = null;
      _error = null;
      _urlController.clear();
    });
  }

  String? _validateUrl(String? value) {
    if (value == null || value.trim().isEmpty) {
      return 'Please enter a fiscal URL';
    }
    final uri = Uri.tryParse(value.trim());
    if (uri == null || !uri.hasScheme || !uri.hasAuthority) {
      return 'Please enter a valid URL';
    }
    if (uri.scheme != 'http' && uri.scheme != 'https') {
      return 'URL must start with http:// or https://';
    }
    return null;
  }

  @override
  void dispose() {
    _urlController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Submit Receipt')),
      body: SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: switch (_state) {
            _ScreenState.input => _buildInputForm(),
            _ScreenState.submitting => _buildProgressState(
                'Submitting...', 'Sending the fiscal URL to the server.'),
            _ScreenState.polling => _buildProgressState(
                'Processing...', 'Waiting for the receipt to be scraped from SEFAZ.'),
            _ScreenState.success => _buildSuccessState(),
            _ScreenState.failed => _buildFailedState(),
          },
        ),
      ),
    );
  }

  Widget _buildInputForm() {
    return Form(
      key: _formKey,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text(
            'Enter NFC-e URL',
            style: Theme.of(context).textTheme.titleLarge,
          ),
          const SizedBox(height: 8),
          Text(
            'Paste the fiscal URL from the QR Code on your receipt.',
            style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                  color: Theme.of(context).colorScheme.onSurfaceVariant,
                ),
          ),
          const SizedBox(height: 24),
          TextFormField(
            controller: _urlController,
            keyboardType: TextInputType.url,
            textInputAction: TextInputAction.done,
            maxLines: 3,
            onFieldSubmitted: (_) => _submit(),
            decoration: const InputDecoration(
              labelText: 'Fiscal URL',
              hintText: 'https://www.sefaz.rs.gov.br/...',
              border: OutlineInputBorder(),
              prefixIcon: Icon(Icons.link),
            ),
            validator: _validateUrl,
          ),
          const SizedBox(height: 24),
          FilledButton.icon(
            onPressed: _submit,
            icon: const Icon(Icons.send),
            label: const Text('Submit'),
          ),
        ],
      ),
    );
  }

  Widget _buildProgressState(String title, String subtitle) {
    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          const SizedBox(
            height: 64,
            width: 64,
            child: CircularProgressIndicator(),
          ),
          const SizedBox(height: 24),
          Text(title, style: Theme.of(context).textTheme.titleLarge),
          const SizedBox(height: 8),
          Text(
            subtitle,
            textAlign: TextAlign.center,
            style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                  color: Theme.of(context).colorScheme.onSurfaceVariant,
                ),
          ),
        ],
      ),
    );
  }

  Widget _buildSuccessState() {
    final alreadyExisted = _submitResponse?.alreadyExists ?? false;

    return Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(Icons.check_circle,
              size: 72, color: Theme.of(context).colorScheme.primary),
          const SizedBox(height: 16),
          Text(
            alreadyExisted ? 'Receipt Already Exists' : 'Receipt Extracted',
            style: Theme.of(context).textTheme.titleLarge,
          ),
          const SizedBox(height: 8),
          Text(
            alreadyExisted
                ? 'This receipt was already scanned for your house.'
                : 'The receipt data has been successfully extracted and saved.',
            textAlign: TextAlign.center,
            style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                  color: Theme.of(context).colorScheme.onSurfaceVariant,
                ),
          ),
          const SizedBox(height: 32),
          FilledButton(
            onPressed: () => Navigator.pop(context, true),
            child: const Text('Back to Receipts'),
          ),
          const SizedBox(height: 8),
          OutlinedButton(
            onPressed: _reset,
            child: const Text('Submit Another'),
          ),
        ],
      ),
    );
  }

  Widget _buildFailedState() {
    final failureReason = _finalJob?.failureReason;

    return Center(
      child: SingleChildScrollView(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            _buildFailureIcon(failureReason),
            const SizedBox(height: 16),
            Text(
              _failureTitle(failureReason),
              style: Theme.of(context).textTheme.titleLarge,
            ),
            const SizedBox(height: 8),
            Text(
              _error ?? _failureDescription(failureReason),
              textAlign: TextAlign.center,
              style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                    color: Theme.of(context).colorScheme.onSurfaceVariant,
                  ),
            ),
            if (_finalJob?.errorDetail.isNotEmpty == true) ...[
              const SizedBox(height: 12),
              Container(
                padding: const EdgeInsets.all(12),
                decoration: BoxDecoration(
                  color: Theme.of(context).colorScheme.surfaceContainerHighest,
                  borderRadius: BorderRadius.circular(8),
                ),
                child: Text(
                  _finalJob!.errorDetail,
                  style: Theme.of(context).textTheme.bodySmall?.copyWith(
                        fontFamily: 'monospace',
                      ),
                ),
              ),
            ],
            const SizedBox(height: 32),
            FilledButton.icon(
              onPressed: () {
                setState(() {
                  _state = _ScreenState.input;
                  _error = null;
                });
              },
              icon: const Icon(Icons.refresh),
              label: const Text('Try Again'),
            ),
            const SizedBox(height: 8),
            OutlinedButton(
              onPressed: () => Navigator.pop(context, false),
              child: const Text('Cancel'),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildFailureIcon(FailureReason? reason) {
    final IconData icon;
    final Color color;

    switch (reason) {
      case FailureReason.timeout:
        icon = Icons.timer_off;
        color = Theme.of(context).colorScheme.error;
      case FailureReason.captcha:
        icon = Icons.smart_toy;
        color = Colors.orange;
      case FailureReason.navigation:
        icon = Icons.explore_off;
        color = Theme.of(context).colorScheme.error;
      case FailureReason.parsing:
        icon = Icons.code_off;
        color = Theme.of(context).colorScheme.error;
      case FailureReason.unknown:
      case null:
        icon = Icons.error_outline;
        color = Theme.of(context).colorScheme.error;
    }

    return Icon(icon, size: 72, color: color);
  }

  String _failureTitle(FailureReason? reason) {
    switch (reason) {
      case FailureReason.timeout:
        return 'Extraction Timed Out';
      case FailureReason.captcha:
        return 'Captcha Blocked';
      case FailureReason.navigation:
        return 'Navigation Failed';
      case FailureReason.parsing:
        return 'Parsing Failed';
      case FailureReason.unknown:
      case null:
        return 'Extraction Failed';
    }
  }

  String _failureDescription(FailureReason? reason) {
    switch (reason) {
      case FailureReason.timeout:
        return 'The SEFAZ page took too long to respond. Please try again later.';
      case FailureReason.captcha:
        return 'SEFAZ is showing a captcha challenge. This is a known limitation -- please try again later.';
      case FailureReason.navigation:
        return 'Could not navigate to the SEFAZ page. Please check the URL and try again.';
      case FailureReason.parsing:
        return 'The receipt data could not be extracted from the page. The page structure may have changed.';
      case FailureReason.unknown:
      case null:
        return 'An unexpected error occurred during extraction. Please try again.';
    }
  }
}
