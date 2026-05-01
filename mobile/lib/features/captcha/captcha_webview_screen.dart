import 'package:flutter/material.dart';
import 'package:webview_cookie_manager/webview_cookie_manager.dart';
import 'package:webview_flutter/webview_flutter.dart';

import 'package:jullius_scan/models/captcha_context.dart';
import 'package:jullius_scan/models/session_cookie.dart';
import 'package:jullius_scan/services/api_client.dart';

// JS snippet that returns true when the NFC-e receipt content is visible.
//
// SEFAZ pages vary by state and by template (resumed vs detailed view), so we
// match multiple signals. Returning true here triggers cookie extraction and
// resume submission to the backend.
const _nfceContentCheck = '''
(function() {
  if (document.getElementById('tabResult')) return true;
  if (document.querySelector('.NFe')) return true;
  if (document.querySelector('.txtTopo')) return true;
  var title = (document.title || '').toLowerCase();
  if (/nfc?-?e|nota fiscal|consulta da nfc/.test(title)) return true;
  var body = document.body ? (document.body.innerText || '') : '';
  if (/chave de acesso/i.test(body)) return true;
  return false;
})()
''';

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

  @override
  void initState() {
    super.initState();
    _loadContext();
  }

  Future<void> _loadContext() async {
    try {
      final ctx = await widget.apiClient.fetchCaptchaContext(widget.jobId);
      setState(() => _ctx = ctx);
      _initWebView(ctx);
    } catch (e) {
      setState(() {
        _loadError = 'Não foi possível carregar o contexto do captcha: $e';
        _isLoading = false;
      });
    }
  }

  void _initWebView(CaptchaContext ctx) {
    _webController = WebViewController()
      ..setJavaScriptMode(JavaScriptMode.unrestricted)
      // Do not override the user agent — the device's native Chrome/Android UA
      // is required for Cloudflare Turnstile to pass. Using the worker's headless
      // Linux user agent causes an immediate "Falha na verificação".
      ..setNavigationDelegate(
        NavigationDelegate(
          onPageFinished: (url) => _onPageFinished(url, ctx.sefazUrl),
          onWebResourceError: (error) {
            setState(() => _isLoading = false);
          },
        ),
      )
      ..loadRequest(Uri.parse(ctx.sefazUrl));
  }

  Future<void> _onPageFinished(String url, String sefazUrl) async {
    setState(() => _isLoading = false);

    if (_captchaResolved || _isSubmitting) return;

    // Check if the page now shows NFC-e receipt content.
    final result = await _webController.runJavaScriptReturningResult(
      _nfceContentCheck,
    );
    final resolved = result.toString() == 'true';

    if (resolved) {
      await _extractAndSubmitCookies(url, sefazUrl);
    }
  }

  Future<void> _extractAndSubmitCookies(
    String currentUrl,
    String sefazUrl,
  ) async {
    if (_isSubmitting) return;
    setState(() => _isSubmitting = true);

    try {
      final host = Uri.parse(sefazUrl).host;
      final cookiesUrl = currentUrl.isNotEmpty ? currentUrl : sefazUrl;
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

      // Capture the WebView's actual user-agent so the worker can replay the
      // Cloudflare session with the same UA that the cookie was issued for.
      String? webViewUA;
      try {
        final uaResult = await _webController.runJavaScriptReturningResult(
          'navigator.userAgent',
        );
        final raw = uaResult.toString();
        // runJavaScriptReturningResult wraps strings in quotes on some platforms.
        webViewUA = raw.startsWith('"') && raw.endsWith('"')
            ? raw.substring(1, raw.length - 1)
            : raw;
      } catch (_) {
        // If we can't get the UA, proceed without it — the backend will keep
        // whatever was previously stored, which is better than failing entirely.
      }

      await widget.apiClient.submitCaptchaResume(
        widget.jobId,
        cookies,
        userAgent: webViewUA,
      );

      setState(() => _captchaResolved = true);
      if (mounted) {
        Navigator.of(context).pop(true);
      }
    } catch (e) {
      setState(() => _isSubmitting = false);
      if (!mounted) return;
      _showResumeError(e.toString(), sefazUrl);
    }
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
              _extractAndSubmitCookies(sefazUrl, sefazUrl);
            },
            child: const Text('Tentar novamente'),
          ),
        ],
      ),
    );
  }

  void _cancel() {
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
        if (_isLoading || _isSubmitting)
          const Center(child: CircularProgressIndicator()),
      ],
    );
  }
}
