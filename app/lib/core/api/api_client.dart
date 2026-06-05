import 'dart:convert';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:http/http.dart' as http;

/// ApiClient — единственная точка общения с Go-бэкендом.
///
/// Токены хранятся в flutter_secure_storage (зашифрованное хранилище):
/// - Android: EncryptedSharedPreferences (AES-256)
/// - iOS/macOS: Keychain
/// - Windows: DPAPI / Credential Manager
class ApiClient {
  static const String baseUrl = 'http://127.0.0.1:8080/api';

  static const _storage = FlutterSecureStorage(
    aOptions: AndroidOptions(encryptedSharedPreferences: true),
  );

  String? _accessToken;
  String? _refreshToken;
  DateTime? _expiresAt;

  // ── Tokens ──────────────────────────────────────────

  Future<void> loadTokens() async {
    _accessToken = await _storage.read(key: 'access_token');
    _refreshToken = await _storage.read(key: 'refresh_token');
    final expiresStr = await _storage.read(key: 'expires_at');
    if (expiresStr != null) {
      _expiresAt = DateTime.tryParse(expiresStr);
    }
  }

  Future<void> saveTokens(Map<String, dynamic> tokens) async {
    _accessToken = tokens['access_token'];
    _refreshToken = tokens['refresh_token'];
    _expiresAt = DateTime.tryParse(tokens['expires_at'] ?? '');

    await _storage.write(key: 'access_token', value: _accessToken!);
    await _storage.write(key: 'refresh_token', value: _refreshToken!);
    if (tokens['expires_at'] != null) {
      await _storage.write(key: 'expires_at', value: tokens['expires_at']);
    }
  }

  Future<void> clearTokens() async {
    _accessToken = null;
    _refreshToken = null;
    _expiresAt = null;
    await _storage.deleteAll();
  }

  bool get isLoggedIn => _accessToken != null;

  bool get isTokenExpired {
    if (_expiresAt == null) return true;
    return DateTime.now().isAfter(_expiresAt!.subtract(const Duration(seconds: 60)));
  }

  // ── Auth ────────────────────────────────────────────

  Future<Map<String, dynamic>> register({
    required String username,
    required String displayName,
    required String email,
    required String password,
  }) async {
    final response = await _post('/auth/register', {
      'username': username,
      'display_name': displayName,
      'email': email,
      'password': password,
      'device_name': 'Windows App',
      'device_type': 'windows',
    }, auth: false);

    await saveTokens(response['tokens']);
    return response;
  }

  Future<Map<String, dynamic>> login({
    required String username,
    required String password,
  }) async {
    final response = await _post('/auth/login', {
      'username': username,
      'password': password,
      'device_name': 'Windows App',
      'device_type': 'windows',
    }, auth: false);

    await saveTokens(response['tokens']);
    return response;
  }

  Future<bool> refreshTokens() async {
    if (_refreshToken == null) return false;

    try {
      final response = await _post('/auth/refresh', {
        'refresh_token': _refreshToken,
      }, auth: false);

      await saveTokens(response);
      return true;
    } catch (_) {
      await clearTokens();
      return false;
    }
  }

  Future<void> logout() async {
    try {
      await _post('/auth/logout', {});
    } catch (_) {}
    await clearTokens();
  }

  Future<Map<String, dynamic>> getMe() async {
    return await _get('/auth/me');
  }

  Future<Map<String, dynamic>> updateProfile({
    String? displayName,
    String? bio,
    String? avatarUrl,
  }) async {
    return await _patch('/auth/me', {
      'display_name': ?displayName,
      'bio': ?bio,
      'avatar_url': ?avatarUrl,
    });
  }

  // ── Users ───────────────────────────────────────────

  Future<List<dynamic>> searchUsers(String query) async {
    return await _getList('/users/search?q=$query');
  }

  Future<Map<String, dynamic>> getUserProfile(String userId) async {
    return await _get('/users/$userId');
  }

  // ── Conversations ───────────────────────────────────

  Future<Map<String, dynamic>> createDirectChat(String otherUserId) async {
    return await _post('/conversations/direct', {
      'other_user_id': otherUserId,
    });
  }

  Future<Map<String, dynamic>> createGroupChat(String title, List<String> memberIds) async {
    return await _post('/conversations/group', {
      'title': title,
      'member_ids': memberIds,
    });
  }

  Future<List<dynamic>> getConversations({int limit = 50, int offset = 0}) async {
    return await _getList('/conversations?limit=$limit&offset=$offset');
  }

  Future<Map<String, dynamic>> sendMessage(String conversationId, String content, {String? replyToId}) async {
    return await _post('/conversations/$conversationId/messages', {
      'content': content,
      'content_type': 'text',
      'reply_to_id': ?replyToId,
    });
  }

  Future<List<dynamic>> getMessages(String conversationId, {int limit = 50, String? before}) async {
    var url = '/conversations/$conversationId/messages?limit=$limit';
    if (before != null) url += '&before=$before';
    return await _getList(url);
  }

  Future<Map<String, dynamic>> editMessage(String conversationId, String messageId, String content) async {
    return await _patch('/conversations/$conversationId/messages/$messageId', {
      'content': content,
    });
  }

  Future<void> deleteMessage(String conversationId, String messageId) async {
    await _delete('/conversations/$conversationId/messages/$messageId');
  }

  Future<void> markAsRead(String conversationId, String messageId) async {
    await _post('/conversations/$conversationId/read', {
      'message_id': messageId,
    });
  }

  // ── Channels ────────────────────────────────────────

  Future<Map<String, dynamic>> createChannel(String name, String description, bool isPublic) async {
    return await _post('/channels', {
      'name': name,
      'description': description,
      'is_public': isPublic,
    });
  }

  Future<List<dynamic>> getMyChannels() async {
    return await _getList('/channels/my');
  }

  Future<List<dynamic>> discoverChannels({int limit = 50}) async {
    return await _getList('/channels/discover?limit=$limit');
  }

  Future<List<dynamic>> getChannelStructure(String channelId) async {
    return await _getList('/channels/$channelId/structure');
  }

  Future<void> joinChannel(String channelId) async {
    await _post('/channels/$channelId/join', {});
  }

  Future<void> leaveChannel(String channelId) async {
    await _post('/channels/$channelId/leave', {});
  }

  Future<Map<String, dynamic>> sendRoomMessage(String roomId, String content) async {
    return await _post('/rooms/$roomId/messages', {
      'content': content,
    });
  }

  Future<List<dynamic>> getRoomMessages(String roomId, {int limit = 50, String? before}) async {
    var url = '/rooms/$roomId/messages?limit=$limit';
    if (before != null) url += '&before=$before';
    return await _getList(url);
  }

  // ── Realtime ────────────────────────────────────────

  Future<String> getRealtimeToken() async {
    final response = await _get('/realtime/token');
    return response['token'];
  }

  Future<String> getSubscriptionToken(String channel) async {
    final response = await _get('/realtime/subscribe?channel=$channel');
    return response['token'];
  }

  // ── HTTP helpers ────────────────────────────────────

  Map<String, String> _headers({bool auth = true}) {
    final headers = {'Content-Type': 'application/json'};
    if (auth && _accessToken != null) {
      headers['Authorization'] = 'Bearer $_accessToken';
    }
    return headers;
  }

  Future<Map<String, dynamic>> _handleResponse(http.Response response) async {
    if (response.statusCode == 401 && _refreshToken != null) {
      final refreshed = await refreshTokens();
      if (!refreshed) {
        throw ApiException('Session expired', 'SESSION_EXPIRED');
      }
      throw RetryException();
    }

    final body = jsonDecode(response.body);

    if (response.statusCode >= 400) {
      throw ApiException(
        body['error'] ?? 'Unknown error',
        body['code'] ?? 'UNKNOWN',
      );
    }

    return body;
  }

  Future<List<dynamic>> _handleListResponse(http.Response response) async {
    if (response.statusCode == 401 && _refreshToken != null) {
      final refreshed = await refreshTokens();
      if (!refreshed) {
        throw ApiException('Session expired', 'SESSION_EXPIRED');
      }
      throw RetryException();
    }

    final body = jsonDecode(response.body);

    if (response.statusCode >= 400) {
      throw ApiException(
        body['error'] ?? 'Unknown error',
        body['code'] ?? 'UNKNOWN',
      );
    }

    return body as List<dynamic>;
  }

  Future<void> _ensureValidToken() async {
    if (isTokenExpired && _refreshToken != null) {
      await refreshTokens();
    }
  }

  Future<Map<String, dynamic>> _get(String path) async {
    await _ensureValidToken();
    final response = await http.get(
      Uri.parse('$baseUrl$path'),
      headers: _headers(),
    );
    return _handleResponse(response);
  }

  Future<List<dynamic>> _getList(String path) async {
    await _ensureValidToken();
    final response = await http.get(
      Uri.parse('$baseUrl$path'),
      headers: _headers(),
    );
    return _handleListResponse(response);
  }

  Future<Map<String, dynamic>> _post(String path, Map<String, dynamic> body, {bool auth = true}) async {
    if (auth) await _ensureValidToken();
    final response = await http.post(
      Uri.parse('$baseUrl$path'),
      headers: _headers(auth: auth),
      body: jsonEncode(body),
    );
    return _handleResponse(response);
  }

  Future<Map<String, dynamic>> _patch(String path, Map<String, dynamic> body) async {
    await _ensureValidToken();
    final response = await http.patch(
      Uri.parse('$baseUrl$path'),
      headers: _headers(),
      body: jsonEncode(body),
    );
    return _handleResponse(response);
  }

  Future<void> _delete(String path) async {
    await _ensureValidToken();
    final response = await http.delete(
      Uri.parse('$baseUrl$path'),
      headers: _headers(),
    );
    if (response.statusCode >= 400) {
      final body = jsonDecode(response.body);
      throw ApiException(body['error'] ?? 'Unknown error', body['code'] ?? 'UNKNOWN');
    }
  }
}

class ApiException implements Exception {
  final String message;
  final String code;
  ApiException(this.message, this.code);

  @override
  String toString() => 'ApiException($code): $message';
}

class RetryException implements Exception {}
