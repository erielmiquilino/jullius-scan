import 'dart:convert';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart';
import 'package:http/testing.dart';
import 'package:jullius_scan/screens/home_screen.dart';
import 'package:jullius_scan/screens/receipt_detail_screen.dart';
import 'package:jullius_scan/services/api_client.dart';
import 'package:jullius_scan/services/auth_service.dart';

Map<String, dynamic> _receiptSummaryJson() {
  return {
    'id': 1,
    'house_id': 1,
    'fiscal_key': '12345678901234567890123456789012345678901234',
    'fiscal_url': 'https://sefaz.example/receipt/1',
    'issued_at': DateTime.utc(2026, 4, 22, 12, 0, 0).toIso8601String(),
    'total_amount': 10.5,
    'created_at': DateTime.utc(2026, 4, 22, 12, 5, 0).toIso8601String(),
  };
}

Map<String, dynamic> _receiptDetailJson() {
  return {
    ..._receiptSummaryJson(),
    'store': {
      'id': 1,
      'cnpj': '75492694000201',
      'name': 'SUPERMERCADOS MYATA',
      'address': 'Rua de Teste, 123',
    },
    'items': [
      {
        'id': 1,
        'description': 'PAO FRANCES',
        'quantity': 1.0,
        'unit': 'UN',
        'unit_price': 10.5,
        'total_price': 10.5,
      },
    ],
  };
}

Response _jsonResponse(Object body, int statusCode) {
  return Response(
    jsonEncode(body),
    statusCode,
    headers: {HttpHeaders.contentTypeHeader: 'application/json'},
  );
}

ApiClient _buildApiClient(MockClient client) {
  return ApiClient(authService: AuthService.test(), httpClient: client);
}

void main() {
  testWidgets('asks for confirmation and cancels without deleting', (
    tester,
  ) async {
    var deleteCalls = 0;

    final apiClient = _buildApiClient(
      MockClient((request) async {
        if (request.method == 'GET' && request.url.path == '/api/v1/receipts/1') {
          return _jsonResponse(_receiptDetailJson(), 200);
        }

        if (request.method == 'DELETE' && request.url.path == '/api/v1/receipts/1') {
          deleteCalls += 1;
          return Response('', 204);
        }

        throw StateError('Unexpected request: ${request.method} ${request.url}');
      }),
    );

    await tester.pumpWidget(
      MaterialApp(
        home: ReceiptDetailScreen(apiClient: apiClient, receiptId: 1),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byTooltip('Remover lançamento'));
    await tester.pumpAndSettle();

    expect(find.text('Deseja mesmo remover este lançamento?'), findsOneWidget);

    await tester.tap(find.text('Cancelar'));
    await tester.pumpAndSettle();

    expect(deleteCalls, 0);
    expect(find.text('Detalhes do Recibo'), findsOneWidget);
  });

  testWidgets('returns to refreshed list after successful deletion', (
    tester,
  ) async {
    var deleted = false;
    var listCalls = 0;
    final authService = AuthService.test();

    final apiClient = ApiClient(
      authService: authService,
      httpClient: MockClient((request) async {
        if (request.method == 'GET' && request.url.path == '/api/v1/receipts') {
          listCalls += 1;
          return _jsonResponse(deleted ? [] : [_receiptSummaryJson()], 200);
        }

        if (request.method == 'GET' && request.url.path == '/api/v1/receipts/1') {
          return _jsonResponse(_receiptDetailJson(), 200);
        }

        if (request.method == 'DELETE' && request.url.path == '/api/v1/receipts/1') {
          deleted = true;
          return Response('', 204);
        }

        throw StateError('Unexpected request: ${request.method} ${request.url}');
      }),
    );

    await tester.pumpWidget(
      MaterialApp(
        home: HomeScreen(apiClient: apiClient, authService: authService),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.byType(ListTile), findsOneWidget);

    await tester.tap(find.byType(ListTile));
    await tester.pumpAndSettle();

    await tester.tap(find.byTooltip('Remover lançamento'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Remover'));
    await tester.pump();
    await tester.pumpAndSettle();

    expect(listCalls, greaterThanOrEqualTo(2));
    expect(find.text('Nenhum recibo ainda'), findsOneWidget);
    expect(find.byType(ListTile), findsNothing);
  });

  testWidgets('keeps detail screen open when deletion fails', (tester) async {
    var deleteCalls = 0;

    final apiClient = _buildApiClient(
      MockClient((request) async {
        if (request.method == 'GET' && request.url.path == '/api/v1/receipts/1') {
          return _jsonResponse(_receiptDetailJson(), 200);
        }

        if (request.method == 'DELETE' && request.url.path == '/api/v1/receipts/1') {
          deleteCalls += 1;
          return _jsonResponse({'error': 'Falha ao remover no servidor.', 'code': 'INTERNAL'}, 500);
        }

        throw StateError('Unexpected request: ${request.method} ${request.url}');
      }),
    );

    await tester.pumpWidget(
      MaterialApp(
        home: ReceiptDetailScreen(apiClient: apiClient, receiptId: 1),
      ),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.byTooltip('Remover lançamento'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Remover'));
    await tester.pump();
    await tester.pumpAndSettle();

    expect(deleteCalls, 1);
    expect(find.text('Detalhes do Recibo'), findsOneWidget);
    expect(find.text('Falha ao remover no servidor.'), findsOneWidget);
    expect(find.byTooltip('Remover lançamento'), findsOneWidget);
  });
}
