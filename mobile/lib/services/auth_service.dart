import 'package:firebase_auth/firebase_auth.dart';
import 'package:flutter/foundation.dart';

/// Wraps Firebase Auth for sign-in, sign-out, and token retrieval.
class AuthService extends ChangeNotifier {
  final FirebaseAuth? _auth;

  AuthService({FirebaseAuth? auth}) : _auth = auth ?? FirebaseAuth.instance;

  AuthService.test() : _auth = null;

  User? get currentUser => _auth?.currentUser;
  bool get isSignedIn => currentUser != null;

  Stream<User?> get authStateChanges =>
      _auth?.authStateChanges() ?? const Stream<User?>.empty();

  /// Sign in with email and password.
  /// Returns the signed-in [User] or throws on failure.
  Future<User> signInWithEmail(String email, String password) async {
    if (_auth == null) {
      throw StateError('AuthService.test does not support sign in');
    }

    final credential = await _auth.signInWithEmailAndPassword(
      email: email,
      password: password,
    );
    notifyListeners();
    return credential.user!;
  }

  /// Get the current Firebase ID token for API requests.
  /// Returns null if not signed in.
  Future<String?> getIdToken({bool forceRefresh = false}) async {
    return currentUser?.getIdToken(forceRefresh);
  }

  /// Sign out the current user.
  Future<void> signOut() async {
    if (_auth != null) {
      await _auth.signOut();
    }
    notifyListeners();
  }
}
