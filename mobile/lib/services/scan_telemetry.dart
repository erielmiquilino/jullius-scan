import 'package:firebase_crashlytics/firebase_crashlytics.dart';
import 'package:flutter/foundation.dart';

/// Thin wrapper around Firebase Crashlytics for the scan + captcha flow.
///
/// In release builds:
///   - [log] adds a breadcrumb (max ~64KB rolling buffer, attached to the next
///     crash report). Crashlytics does NOT ship breadcrumbs unless a fatal or
///     non-fatal error is recorded — they are evidence for crashes, not a
///     standalone log stream.
///   - [recordError] sends a non-fatal error with stack trace, accumulated
///     breadcrumbs, custom keys, and user identifier. Use this in catch blocks
///     where the bug is actively being investigated.
///   - [setStep] writes the most recent step name as a custom key so the
///     dashboard shows where the user was when the next crash fires.
///
/// In debug builds the Crashlytics SDK is disabled (see main.dart), so all
/// methods route to debugPrint instead.
class ScanTelemetry {
  ScanTelemetry._();

  static const String _stepKey = 'last_scan_step';
  static const int _maxValueLen = 200;

  static void log(String event, [Map<String, Object?>? attrs]) {
    final msg = _format(event, attrs);
    if (kDebugMode) {
      debugPrint('[scan] $msg');
      return;
    }
    FirebaseCrashlytics.instance.log(msg);
  }

  /// Mark the most recent step in the flow. Overwrites the previous value so
  /// the next crash report shows the latest known position.
  static Future<void> setStep(String step) async {
    if (kDebugMode) {
      debugPrint('[scan] step=$step');
      return;
    }
    await FirebaseCrashlytics.instance.setCustomKey(_stepKey, step);
  }

  static Future<void> setCustomKey(String key, Object value) async {
    if (kDebugMode) {
      debugPrint('[scan] $key=$value');
      return;
    }
    await FirebaseCrashlytics.instance.setCustomKey(key, value);
  }

  /// Record a non-fatal error so the breadcrumb trail and custom keys are
  /// flushed to Crashlytics. Use for caught exceptions that the user can
  /// recover from but that we still want to investigate.
  static Future<void> recordError(
    Object error,
    StackTrace? stack, {
    String? reason,
    Map<String, Object?>? attrs,
  }) async {
    if (kDebugMode) {
      debugPrint('[scan][error] ${reason ?? error}\n$stack');
      return;
    }
    if (attrs != null) {
      for (final entry in attrs.entries) {
        final v = entry.value;
        if (v == null) continue;
        await FirebaseCrashlytics.instance.setCustomKey(entry.key, v);
      }
    }
    await FirebaseCrashlytics.instance.recordError(
      error,
      stack,
      reason: reason,
      fatal: false,
    );
  }

  static String _format(String event, Map<String, Object?>? attrs) {
    if (attrs == null || attrs.isEmpty) return event;
    final parts = StringBuffer(event);
    for (final entry in attrs.entries) {
      if (entry.value == null) continue;
      var value = entry.value.toString();
      if (value.length > _maxValueLen) {
        value = '${value.substring(0, _maxValueLen)}…';
      }
      parts.write(' ${entry.key}=$value');
    }
    return parts.toString();
  }
}
