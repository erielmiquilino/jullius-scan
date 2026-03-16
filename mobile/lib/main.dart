import 'package:firebase_auth/firebase_auth.dart';
import 'package:firebase_core/firebase_core.dart';
import 'package:flutter/material.dart';

import 'package:jullius_scan/screens/home_screen.dart';
import 'package:jullius_scan/screens/login_screen.dart';
import 'package:jullius_scan/services/api_client.dart';
import 'package:jullius_scan/services/auth_service.dart';

void main() async {
  WidgetsFlutterBinding.ensureInitialized();
  await Firebase.initializeApp();
  runApp(const JulliusScanApp());
}

class JulliusScanApp extends StatefulWidget {
  const JulliusScanApp({super.key});

  @override
  State<JulliusScanApp> createState() => _JulliusScanAppState();
}

class _JulliusScanAppState extends State<JulliusScanApp> {
  final _authService = AuthService();
  late final ApiClient _apiClient;

  @override
  void initState() {
    super.initState();
    _apiClient = ApiClient(authService: _authService);
  }

  @override
  void dispose() {
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
