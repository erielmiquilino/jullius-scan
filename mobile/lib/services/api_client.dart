import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:http/http.dart' as http;

import 'package:jullius_scan/config/api_config.dart';
import 'package:jullius_scan/models/api_error.dart';
import 'package:jullius_scan/models/job.dart';
import 'package:jullius_scan/models/receipt.dart';
import 'package:jullius_scan/models/submit_receipt.dart';
import 'package:jullius_scan/services/auth_service.dart';

/// HTTP client for the Jullius Scan backend API.
///
/// All requests attach the Firebase Bearer token from [AuthService].
class ApiClient {
  final AuthService _authService;
  final http.Client _http;

  ApiClient({required AuthService authService, http.Client? httpClient})
      : _authService = authService,
        _http = httpClient ?? http.Client();

  String get _baseUrl => ApiConfig.baseUrl;

  /// Build authorization headers with the current Firebase token.
  Future<Map<String, String>> _headers() async {
    final token = await _authService.getIdToken();
    return {
      HttpHeaders.contentTypeHeader: 'application/json',
      if (token != null) HttpHeaders.authorizationHeader: 'Bearer $token',
    };
  }

  /// Perform a GET request and return the decoded JSON.
  Future<dynamic> _get(String path) async {
    final uri = Uri.parse('$_baseUrl$path');
    final response = await _http.get(uri, headers: await _headers());
    return _handleResponse(response);
  }

  /// Perform a POST request with a JSON body.
  Future<dynamic> _post(String path, String body) async {
    final uri = Uri.parse('$_baseUrl$path');
    final response = await _http.post(
      uri,
      headers: await _headers(),
      body: body,
    );
    return _handleResponse(response);
  }

  dynamic _handleResponse(http.Response response) {
    if (response.statusCode >= 200 && response.statusCode < 300) {
      if (response.body.isEmpty) return null;
      return jsonDecode(response.body);
    }
    throw ApiError.fromResponse(response.statusCode, response.body);
  }

  // -- Receipts --

  /// Submit a fiscal URL for scraping.
  /// POST /api/v1/receipts
  Future<SubmitReceiptResponse> submitReceipt(String fiscalUrl) async {
    final request = SubmitReceiptRequest(fiscalUrl: fiscalUrl);
    final json = await _post('/api/v1/receipts', request.toJsonString());
    return SubmitReceiptResponse.fromJson(json as Map<String, dynamic>);
  }

  /// List all receipts for the current user's house.
  /// GET /api/v1/receipts
  Future<List<Receipt>> listReceipts() async {
    final json = await _get('/api/v1/receipts');
    final list = json as List<dynamic>;
    return list
        .map((e) => Receipt.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  /// Get a single receipt with store and items.
  /// GET /api/v1/receipts/{id}
  Future<Receipt> getReceipt(int id) async {
    final json = await _get('/api/v1/receipts/$id');
    return Receipt.fromJson(json as Map<String, dynamic>);
  }

  // -- Jobs --

  /// Get the status of a scraping job.
  /// GET /api/v1/jobs/{id}
  Future<Job> getJob(int id) async {
    final json = await _get('/api/v1/jobs/$id');
    return Job.fromJson(json as Map<String, dynamic>);
  }

  /// Poll a job until it reaches a terminal state (completed or failed).
  ///
  /// Returns the final [Job]. Throws [TimeoutException] if polling exceeds
  /// [ApiConfig.pollTimeout].
  Future<Job> pollJobUntilTerminal(int jobId) async {
    final deadline = DateTime.now().add(ApiConfig.pollTimeout);

    while (DateTime.now().isBefore(deadline)) {
      final job = await getJob(jobId);
      if (job.status.isTerminal) return job;
      await Future.delayed(ApiConfig.pollInterval);
    }

    throw TimeoutException(
      'Job $jobId did not complete within ${ApiConfig.pollTimeout.inSeconds}s',
    );
  }

  void dispose() {
    _http.close();
  }
}
