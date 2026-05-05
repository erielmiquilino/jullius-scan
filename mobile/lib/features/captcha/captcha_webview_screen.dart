import 'dart:async';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:webview_cookie_manager/webview_cookie_manager.dart';
import 'package:webview_flutter/webview_flutter.dart';

import 'package:jullius_scan/models/captcha_context.dart';
import 'package:jullius_scan/models/session_cookie.dart';
import 'package:jullius_scan/services/api_client.dart';
import 'package:jullius_scan/services/scan_telemetry.dart';

// JS snippet that returns a JSON string describing whether NFC-e content is visible.
//
// SEFAZ pages vary by state and by template (resumed vs detailed view), so we
// match multiple signals. Challenge/captcha markers always win over positive
// receipt signals.
const _nfceContentCheck = '''
(function() {
  var title = document.title || '';
  var href = window.location ? (window.location.href || '') : '';
  var bodyText = document.body ? (document.body.innerText || '') : '';
  var bodyHTML = document.body ? (document.body.innerHTML || '') : '';
  var haystack = (title + '\\n' + href + '\\n' + bodyText + '\\n' + bodyHTML).toLowerCase();
  var challengeMarkers = [
    'captcha',
    'recaptcha',
    'hcaptcha',
    'g-recaptcha',
    'cf-turnstile',
    'challenge-form',
    'challenge-running',
    'securityverify',
    'verificação para prosseguimento',
    'verificacao para prosseguimento',
    'checking your browser',
    'please enable javascript to continue'
  ];
  var hasChallenge = challengeMarkers.some(function(marker) {
    return haystack.indexOf(marker) >= 0;
  });
  var signals = [];
  function mark(name, condition) {
    if (condition) signals.push(name);
  }
  mark('tabResult', !!document.getElementById('tabResult'));
  mark('NFeClass', !!document.querySelector('.NFe'));
  mark('txtTopo', !!document.querySelector('.txtTopo'));
  mark('dadosGerais', !!document.getElementById('DadosGerais'));
  mark('produtosServicos', !!document.getElementById('ProdutosServicos'));
  mark('itemDetalhe', !!document.querySelector('.item-detalhe'));
  mark('detailUrl', /nf[e]?_detalhecert\\.aspx/i.test(href));
  mark('titleNfce', /nfc?-?e|nota fiscal|consulta da nfc|consulta detalhada/i.test(title));
  mark('chaveAcesso', /chave de acesso/i.test(bodyText));
  mark('cnpjTotal', /cnpj/i.test(bodyText) && /(valor total|total\\s+da\\s+nota|total\\s+nfc)/i.test(bodyText));
  mark('eanDetail', /c[oó]digo\\s+ean\\s+(comercial|tribut[aá]vel)/i.test(bodyText));
  mark('productDetail', /dados dos produtos|produtos e servi[cç]os|c[oó]digo ncm/i.test(bodyText));
  return JSON.stringify({
    resolved: !hasChallenge && signals.length > 0,
    has_challenge: hasChallenge,
    signals: signals,
    title: title,
    href: href,
    body_len: bodyText.length
  });
})()
''';

const _mutationObserverScript = '''
(function() {
  if (window.__julliusScanObserverInstalled) return true;
  window.__julliusScanObserverInstalled = true;
  window.__julliusScanDomChanged = true;
  var pending = null;
  var markChanged = function() { window.__julliusScanDomChanged = true; };
  var observer = new MutationObserver(function() {
    if (pending) clearTimeout(pending);
    pending = setTimeout(markChanged, 250);
  });
  if (document.documentElement) {
    observer.observe(document.documentElement, {
      childList: true,
      subtree: true,
      characterData: true,
      attributes: true
    });
  }
  return true;
})()
''';

const _currentUrlScript = 'window.location.href';
const _pageHTMLScript = 'document.documentElement.outerHTML';
const _consumeDomChangedScript = '''
(function() {
  var changed = window.__julliusScanDomChanged === true;
  window.__julliusScanDomChanged = false;
  return changed;
})()
''';

class _ContentCheckResult {
  final bool resolved;
  final bool hasChallenge;
  final List<String> signals;
  final String title;
  final String href;
  final int bodyLength;

  const _ContentCheckResult({
    required this.resolved,
    required this.hasChallenge,
    required this.signals,
    required this.title,
    required this.href,
    required this.bodyLength,
  });
}

class CaptchaWebViewScreen extends StatefulWidget {
  final int jobId;
  final ApiClient apiClient;

  const CaptchaWebViewScreen({
    super.key,
    required this.jobId,
    required this.apiClient,
  });

  @override
  State<CaptchaWebViewScreen> createState() => _CaptchaWebViewScreenState();
}

class _CaptchaWebViewScreenState extends State<CaptchaWebViewScreen> {
  late final WebViewController _webController;
  final _cookieManager = WebviewCookieManager();

  CaptchaContext? _ctx;
  String? _loadError;
  bool _isLoading = true;
  bool _isSubmitting = false;
  bool _captchaResolved = false;
  bool _disposed = false;
  bool _contentCheckLoopStarted = false;
  bool _stuckTelemetryRecorded = false;
  int _pageFinishedCount = 0;
  int _contentCheckCount = 0;
  int _unresolvedSefazChecks = 0;

  @override
  void initState() {
    super.initState();
    ScanTelemetry.setStep('captcha.screen_init');
    ScanTelemetry.log('captcha.screen_init', {'job_id': widget.jobId});
    _loadContext();
  }

  @override
  void dispose() {
    _disposed = true;
    super.dispose();
  }

  Future<void> _loadContext() async {
    final stopwatch = Stopwatch()..start();
    try {
      final ctx = await widget.apiClient.fetchCaptchaContext(widget.jobId);
      stopwatch.stop();
      ScanTelemetry.log('captcha.context_loaded', {
        'job_id': widget.jobId,
        'sefaz_host': Uri.tryParse(ctx.sefazUrl)?.host,
        'sefaz_url_len': ctx.sefazUrl.length,
        'ua_len': ctx.userAgent.length,
        'elapsed_ms': stopwatch.elapsedMilliseconds,
      });
      setState(() => _ctx = ctx);
      _initWebView(ctx);
    } catch (e, stack) {
      stopwatch.stop();
      ScanTelemetry.recordError(
        e,
        stack,
        reason: 'fetchCaptchaContext failed',
        attrs: {
          'job_id': widget.jobId,
          'elapsed_ms': stopwatch.elapsedMilliseconds,
        },
      );
      setState(() {
        _loadError = 'Não foi possível carregar o contexto do captcha: $e';
        _isLoading = false;
      });
    }
  }

  void _initWebView(CaptchaContext ctx) {
    ScanTelemetry.log('captcha.webview_init', {
      'job_id': widget.jobId,
      'sefaz_host': Uri.tryParse(ctx.sefazUrl)?.host,
    });
    _webController = WebViewController()
      ..setJavaScriptMode(JavaScriptMode.unrestricted)
      // Do not override the user agent — the device's native Chrome/Android UA
      // is required for Cloudflare Turnstile to pass. Using the worker's headless
      // Linux user agent causes an immediate "Falha na verificação".
      ..setNavigationDelegate(
        NavigationDelegate(
          onPageStarted: (url) {
            ScanTelemetry.log('captcha.page_started', {
              'host': Uri.tryParse(url)?.host,
              'url_len': url.length,
            });
          },
          onPageFinished: (url) => _onPageFinished(url, ctx.sefazUrl),
          onWebResourceError: (error) {
            ScanTelemetry.log('captcha.web_resource_error', {
              'job_id': widget.jobId,
              'description': error.description,
              'error_type': error.errorType?.name,
              'error_code': error.errorCode,
              'is_main_frame': error.isForMainFrame,
            });
            setState(() => _isLoading = false);
          },
          onHttpError: (error) {
            ScanTelemetry.log('captcha.http_error', {
              'job_id': widget.jobId,
              'status': error.response?.statusCode,
              'host': Uri.tryParse(error.request?.uri.toString() ?? '')?.host,
            });
          },
        ),
      )
      ..loadRequest(Uri.parse(ctx.sefazUrl));
    unawaited(_startContentCheckLoop(ctx.sefazUrl));
  }

  Future<void> _onPageFinished(String url, String sefazUrl) async {
    _pageFinishedCount++;
    if (mounted) {
      setState(() => _isLoading = false);
    }

    if (_captchaResolved || _isSubmitting) {
      ScanTelemetry.log('captcha.page_finished_ignored', {
        'job_id': widget.jobId,
        'count': _pageFinishedCount,
        'resolved': _captchaResolved,
        'submitting': _isSubmitting,
      });
      return;
    }

    try {
      await _webController.runJavaScriptReturningResult(
        _mutationObserverScript,
      );
    } catch (e, stack) {
      ScanTelemetry.recordError(
        e,
        stack,
        reason: 'install DOM observer failed',
        attrs: {'job_id': widget.jobId, 'count': _pageFinishedCount},
      );
    }

    await _checkForResolvedContent(
      reason: 'page_finished',
      currentUrl: url,
      sefazUrl: sefazUrl,
    );
  }

  Future<void> _startContentCheckLoop(String sefazUrl) async {
    if (_contentCheckLoopStarted) return;
    _contentCheckLoopStarted = true;
    while (!_disposed && !_captchaResolved) {
      await Future.delayed(const Duration(seconds: 2));
      if (_disposed || _captchaResolved || _isSubmitting || _ctx == null) {
        continue;
      }
      final domChanged = await _consumeDomChanged();
      await _checkForResolvedContent(
        reason: domChanged ? 'dom_mutation' : 'periodic',
        currentUrl: null,
        sefazUrl: sefazUrl,
      );
    }
  }

  Future<void> _checkForResolvedContent({
    required String reason,
    required String? currentUrl,
    required String sefazUrl,
  }) async {
    if (_captchaResolved || _isSubmitting) return;

    _contentCheckCount++;
    _ContentCheckResult? check;
    Object? jsError;
    try {
      final result = await _webController.runJavaScriptReturningResult(
        _nfceContentCheck,
      );
      check = _decodeContentCheck(result);
    } catch (e, stack) {
      jsError = e;
      ScanTelemetry.recordError(
        e,
        stack,
        reason: 'content check JS error',
        attrs: {'job_id': widget.jobId, 'count': _contentCheckCount},
      );
    }

    if (check == null) {
      ScanTelemetry.log('captcha.content_check_failed', {
        'job_id': widget.jobId,
        'count': _contentCheckCount,
        'reason': reason,
        'error': jsError,
      });
      return;
    }

    ScanTelemetry.log('captcha.content_check', {
      'job_id': widget.jobId,
      'count': _contentCheckCount,
      'page_finished_count': _pageFinishedCount,
      'reason': reason,
      'host': Uri.tryParse(
        check.href.isNotEmpty ? check.href : (currentUrl ?? ''),
      )?.host,
      'path': Uri.tryParse(
        check.href.isNotEmpty ? check.href : (currentUrl ?? ''),
      )?.path,
      'resolved': check.resolved,
      'has_challenge': check.hasChallenge,
      'signals': check.signals.join(','),
      'title': check.title,
      'body_len': check.bodyLength,
    });

    if (check.resolved) {
      ScanTelemetry.setStep('captcha.content_visible');
      await _extractAndSubmitCookies(
        check.href.isNotEmpty ? check.href : (currentUrl ?? ''),
        sefazUrl,
      );
      return;
    }

    await _recordStuckTelemetryIfNeeded(check, sefazUrl);
  }

  _ContentCheckResult? _decodeContentCheck(Object result) {
    final raw = result.toString();
    try {
      dynamic decoded = jsonDecode(raw);
      if (decoded is String) {
        decoded = jsonDecode(decoded);
      }
      if (decoded is! Map<String, dynamic>) {
        return null;
      }
      final rawSignals = decoded['signals'];
      final signals = rawSignals is List
          ? rawSignals.map((s) => s.toString()).toList()
          : <String>[];
      return _ContentCheckResult(
        resolved: decoded['resolved'] == true,
        hasChallenge: decoded['has_challenge'] == true,
        signals: signals,
        title: decoded['title']?.toString() ?? '',
        href: decoded['href']?.toString() ?? '',
        bodyLength: decoded['body_len'] is num
            ? (decoded['body_len'] as num).toInt()
            : 0,
      );
    } catch (_) {
      return null;
    }
  }

  Future<void> _recordStuckTelemetryIfNeeded(
    _ContentCheckResult check,
    String sefazUrl,
  ) async {
    if (_stuckTelemetryRecorded || check.hasChallenge) return;

    final checkUri = Uri.tryParse(check.href);
    final sefazUri = Uri.tryParse(sefazUrl);
    final host = checkUri?.host ?? sefazUri?.host ?? '';
    final expectedHost = sefazUri?.host ?? '';
    final isExpectedHost =
        host == expectedHost ||
        (expectedHost.isNotEmpty && host.endsWith(expectedHost));
    if (!isExpectedHost) return;

    _unresolvedSefazChecks++;
    if (_unresolvedSefazChecks < 8) return;

    _stuckTelemetryRecorded = true;
    await ScanTelemetry.recordError(
      StateError('Captcha WebView content not recognized'),
      StackTrace.current,
      reason: 'captcha content unresolved',
      attrs: {
        'job_id': widget.jobId,
        'check_count': _contentCheckCount,
        'page_finished_count': _pageFinishedCount,
        'host': host,
        'path': checkUri?.path ?? '',
        'title': check.title,
        'body_len': check.bodyLength,
        'signals': check.signals.join(','),
      },
    );
  }

  Future<void> _extractAndSubmitCookies(
    String currentUrl,
    String sefazUrl,
  ) async {
    if (_isSubmitting) return;
    setState(() => _isSubmitting = true);

    final stopwatch = Stopwatch()..start();
    try {
      final host = Uri.parse(sefazUrl).host;
      final effectiveCurrentUrl = await _captureCurrentUrl(currentUrl);
      final cookiesUrl = effectiveCurrentUrl.isNotEmpty
          ? effectiveCurrentUrl
          : sefazUrl;
      final rawCookies = await _cookieManager.getCookies(cookiesUrl);
      final cookies = rawCookies
          .map(
            (c) => SessionCookie(
              name: c.name,
              value: c.value,
              domain: (c.domain == null || c.domain!.isEmpty)
                  ? host
                  : c.domain!,
              path: (c.path == null || c.path!.isEmpty) ? '/' : c.path!,
              expires: c.expires != null
                  ? c.expires!.millisecondsSinceEpoch / 1000.0
                  : null,
              httpOnly: c.httpOnly,
              secure: c.secure,
              sameSite: null,
            ),
          )
          .toList();

      // Capture cookie names (no values) for diagnostics — names alone reveal
      // whether Cloudflare's cf_clearance / __cf_bm cookies are present.
      final cookieNames = cookies.map((c) => c.name).toList();
      final hasCfClearance = cookieNames.any((n) => n == 'cf_clearance');
      final hasCfBm = cookieNames.any((n) => n == '__cf_bm');

      ScanTelemetry.log('captcha.cookies_extracted', {
        'job_id': widget.jobId,
        'count': cookies.length,
        'host': host,
        'has_cf_clearance': hasCfClearance,
        'has_cf_bm': hasCfBm,
        'names': cookieNames.join(','),
      });

      // Capture the WebView's actual user-agent so the worker can replay the
      // Cloudflare session with the same UA that the cookie was issued for.
      String? webViewUA;
      try {
        final uaResult = await _webController.runJavaScriptReturningResult(
          'navigator.userAgent',
        );
        webViewUA = _decodeJavaScriptString(uaResult);
        ScanTelemetry.log('captcha.ua_captured', {
          'job_id': widget.jobId,
          'len': webViewUA.length,
          'prefix': webViewUA.length > 80
              ? webViewUA.substring(0, 80)
              : webViewUA,
        });
      } catch (e, stack) {
        ScanTelemetry.recordError(
          e,
          stack,
          reason: 'capture UA failed',
          attrs: {'job_id': widget.jobId},
        );
        // If we can't get the UA, proceed without it — the backend will keep
        // whatever was previously stored, which is better than failing entirely.
      }

      String? pageHTML;
      try {
        pageHTML = await _capturePageHTML();
        ScanTelemetry.log('captcha.page_html_captured', {
          'job_id': widget.jobId,
          'len': pageHTML.length,
        });
      } catch (e, stack) {
        ScanTelemetry.recordError(
          e,
          stack,
          reason: 'capture page HTML failed',
          attrs: {'job_id': widget.jobId},
        );
      }

      ScanTelemetry.setStep('captcha.resume_submit');
      await widget.apiClient.submitCaptchaResume(
        widget.jobId,
        cookies,
        userAgent: webViewUA,
        currentUrl: effectiveCurrentUrl,
        pageHtml: pageHTML,
      );
      stopwatch.stop();
      ScanTelemetry.log('captcha.resume_submitted', {
        'job_id': widget.jobId,
        'cookie_count': cookies.length,
        'elapsed_ms': stopwatch.elapsedMilliseconds,
        'current_host': Uri.tryParse(effectiveCurrentUrl)?.host,
        'page_html_len': pageHTML?.length,
      });

      setState(() => _captchaResolved = true);
      if (mounted) {
        Navigator.of(context).pop(true);
      }
    } catch (e, stack) {
      stopwatch.stop();
      ScanTelemetry.recordError(
        e,
        stack,
        reason: 'submitCaptchaResume failed',
        attrs: {
          'job_id': widget.jobId,
          'elapsed_ms': stopwatch.elapsedMilliseconds,
        },
      );
      if (!mounted) return;
      setState(() => _isSubmitting = false);
      _showResumeError(e.toString(), sefazUrl);
    }
  }

  Future<bool> _consumeDomChanged() async {
    try {
      final result = await _webController.runJavaScriptReturningResult(
        _consumeDomChangedScript,
      );
      return result.toString() == 'true';
    } catch (_) {
      return false;
    }
  }

  Future<String> _captureCurrentUrl(String fallback) async {
    try {
      final result = await _webController.runJavaScriptReturningResult(
        _currentUrlScript,
      );
      final value = _decodeJavaScriptString(result);
      if (value.isNotEmpty) return value;
    } catch (_) {
      // The caller already has the latest navigation URL as a safe fallback.
    }
    return fallback;
  }

  Future<String> _capturePageHTML() async {
    final result = await _webController.runJavaScriptReturningResult(
      _pageHTMLScript,
    );
    return _decodeJavaScriptString(result);
  }

  String _decodeJavaScriptString(Object result) {
    final raw = result.toString();
    try {
      final decoded = jsonDecode(raw);
      if (decoded is String) return decoded;
    } catch (_) {
      // Some platforms already return the raw string without JSON quoting.
    }
    if (raw.startsWith('"') && raw.endsWith('"') && raw.length >= 2) {
      return raw.substring(1, raw.length - 1);
    }
    return raw;
  }

  void _showResumeError(String error, String sefazUrl) {
    showDialog<bool>(
      context: context,
      barrierDismissible: false,
      builder: (ctx) => AlertDialog(
        title: const Text('Erro ao enviar captcha'),
        content: Text(
          'Não foi possível confirmar a resolução do captcha.\n\n$error',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(ctx).pop(false),
            child: const Text('Cancelar'),
          ),
          ElevatedButton(
            onPressed: () {
              Navigator.of(ctx).pop(true);
              ScanTelemetry.log('captcha.resume_retry', {
                'job_id': widget.jobId,
              });
              _extractAndSubmitCookies(sefazUrl, sefazUrl);
            },
            child: const Text('Tentar novamente'),
          ),
        ],
      ),
    );
  }

  void _cancel() {
    ScanTelemetry.log('captcha.user_cancelled', {'job_id': widget.jobId});
    Navigator.of(context).pop(false);
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Resolver Captcha'),
        leading: IconButton(
          icon: const Icon(Icons.close),
          tooltip: 'Cancelar',
          onPressed: _cancel,
        ),
      ),
      body: _buildBody(),
    );
  }

  Widget _buildBody() {
    if (_loadError != null) {
      return Center(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Text(_loadError!, textAlign: TextAlign.center),
        ),
      );
    }

    if (_ctx == null) {
      return const Center(child: CircularProgressIndicator());
    }

    return Stack(
      children: [
        WebViewWidget(controller: _webController),
        Positioned(
          left: 16,
          right: 16,
          bottom: 16,
          child: SafeArea(
            child: Card(
              elevation: 4,
              child: Padding(
                padding: const EdgeInsets.all(12),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    const Text(
                      'Quando a nota fiscal estiver visível, o app deve continuar automaticamente.',
                      textAlign: TextAlign.center,
                    ),
                    const SizedBox(height: 8),
                    SizedBox(
                      width: double.infinity,
                      child: FilledButton.icon(
                        icon: const Icon(Icons.check_circle_outline),
                        label: const Text('Já resolvi, continuar'),
                        onPressed: _isSubmitting
                            ? null
                            : () {
                                ScanTelemetry.log('captcha.manual_continue', {
                                  'job_id': widget.jobId,
                                });
                                _extractAndSubmitCookies(
                                  _ctx?.sefazUrl ?? '',
                                  _ctx?.sefazUrl ?? '',
                                );
                              },
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ),
        ),
        if (_isLoading || _isSubmitting)
          const Center(child: CircularProgressIndicator()),
      ],
    );
  }
}
