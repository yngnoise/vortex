/// 📁 Куда: lib/core/models/auth_response.dart
/// 📝 Ответы auth-эндпоинтов — токены и связка user+tokens
/// 🔗 Требует: user.dart

import 'user.dart';

/// Пара токенов. Go: auth.TokenPair
class TokenPair {
  final String accessToken;
  final String refreshToken;
  final DateTime expiresAt;

  const TokenPair({
    required this.accessToken,
    required this.refreshToken,
    required this.expiresAt,
  });

  bool get isExpired => DateTime.now().isAfter(expiresAt);

  factory TokenPair.fromJson(Map<String, dynamic> json) {
    return TokenPair(
      accessToken: json['access_token'] as String,
      refreshToken: json['refresh_token'] as String,
      expiresAt: DateTime.parse(json['expires_at'] as String),
    );
  }
}

/// Ответ на register/login. Go: auth.authResponse
class AuthResponse {
  final User user;
  final TokenPair tokens;

  const AuthResponse({required this.user, required this.tokens});

  factory AuthResponse.fromJson(Map<String, dynamic> json) {
    return AuthResponse(
      user: User.fromJson(json['user'] as Map<String, dynamic>),
      tokens: TokenPair.fromJson(json['tokens'] as Map<String, dynamic>),
    );
  }
}
