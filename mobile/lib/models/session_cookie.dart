/// A browser session cookie to be sent to POST /api/v1/jobs/{id}/captcha/resume.
class SessionCookie {
  final String name;
  final String value;
  final String domain;
  final String path;
  final double? expires;
  final bool httpOnly;
  final bool secure;
  final String? sameSite;

  const SessionCookie({
    required this.name,
    required this.value,
    required this.domain,
    required this.path,
    this.expires,
    required this.httpOnly,
    required this.secure,
    this.sameSite,
  });

  Map<String, dynamic> toJson() => {
        'name': name,
        'value': value,
        'domain': domain,
        'path': path,
        if (expires != null) 'expires': expires,
        'http_only': httpOnly,
        'secure': secure,
        if (sameSite != null) 'same_site': sameSite,
      };
}
