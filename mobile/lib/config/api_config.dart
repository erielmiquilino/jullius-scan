/// API configuration for the Jullius Scan backend.
///
/// The base URL can be overridden at build time via:
///   flutter run --dart-define=API_BASE_URL=http://192.168.1.100:8080
///
/// Defaults:
///   - Debug: http://10.0.2.2:8080 (Android emulator -> host localhost)
///   - Release: https://jullius-api.skadi.digital
class ApiConfig {
  ApiConfig._();

  static const String _buildTimeUrl = String.fromEnvironment(
    'API_BASE_URL',
    defaultValue: '',
  );

  static String get baseUrl {
    if (_buildTimeUrl.isNotEmpty) return _buildTimeUrl;
    const isRelease = bool.fromEnvironment('dart.vm.product');
    return isRelease
        ? 'https://jullius-api.skadi.digital'
        : 'http://10.0.2.2:8080';
  }

  static const Duration pollInterval = Duration(seconds: 1);
  static const Duration pollTimeout = Duration(seconds: 30);
}
