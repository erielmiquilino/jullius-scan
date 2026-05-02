import 'dart:async';

import 'package:firebase_auth/firebase_auth.dart';
import 'package:firebase_core/firebase_core.dart';
import 'package:firebase_crashlytics/firebase_crashlytics.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';

import 'package:jullius_scan/screens/home_screen.dart';
import 'package:jullius_scan/screens/login_screen.dart';
import 'package:jullius_scan/services/api_client.dart';
import 'package:jullius_scan/services/auth_service.dart';

void main() {
  runZonedGuarded(() async {
    WidgetsFlutterBinding.ensureInitialized();
    await Firebase.initializeApp();

    await FirebaseCrashlytics.instance.setCrashlyticsCollectionEnabled(
      !kDebugMode,
    );

    FlutterError.onError = FirebaseCrashlytics.instance.recordFlutterFatalError;
    PlatformDispatcher.instance.onError = (error, stack) {
      FirebaseCrashlytics.instance.recordError(error, stack, fatal: true);
      return true;
    };

    runApp(const JulliusScanApp());
  }, (error, stack) {
    FirebaseCrashlytics.instance.recordError(error, stack, fatal: true);
  });
}

class JulliusScanApp extends StatefulWidget {
  const JulliusScanApp({super.key});

  @override
  State<JulliusScanApp> createState() => _JulliusScanAppState();
}

class _JulliusScanAppState extends State<JulliusScanApp> {
  final _authService = AuthService();
  late final ApiClient _apiClient;
  late final StreamSubscription<User?> _authSub;

  @override
  void initState() {
    super.initState();
    _apiClient = ApiClient(authService: _authService);
    _authSub = _authService.authStateChanges.listen(_onAuthStateChanged);
  }

  Future<void> _onAuthStateChanged(User? user) async {
    final crashlytics = FirebaseCrashlytics.instance;
    if (user == null) {
      await crashlytics.setUserIdentifier('');
      return;
    }
    await crashlytics.setUserIdentifier(user.uid);
    final email = user.email;
    if (email != null) {
      await crashlytics.setCustomKey('email', email);
    }
  }

  @override
  void dispose() {
    _authSub.cancel();
    _apiClient.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'Jullius Scan',
      debugShowCheckedModeBanner: false,
      theme: ThemeData(
        colorScheme: ColorScheme.fromSeed(seedColor: Colors.teal),
        useMaterial3: true,
      ),
      home: StreamBuilder<User?>(
        stream: _authService.authStateChanges,
        builder: (context, snapshot) {
          if (snapshot.connectionState == ConnectionState.waiting) {
            return const Scaffold(
              body: Center(child: CircularProgressIndicator()),
            );
          }

          if (snapshot.hasData) {
            return HomeScreen(
              apiClient: _apiClient,
              authService: _authService,
            );
          }

          return LoginScreen(authService: _authService);
        },
      ),
    );
  }
}
