/// Response from GET /api/v1/jobs/{id}/captcha.
class CaptchaContext {
  final String sefazUrl;
  final String userAgent;

  const CaptchaContext({required this.sefazUrl, required this.userAgent});

  factory CaptchaContext.fromJson(Map<String, dynamic> json) {
    return CaptchaContext(
      sefazUrl: json['sefaz_url'] as String,
      userAgent: json['user_agent'] as String,
    );
  }
}
