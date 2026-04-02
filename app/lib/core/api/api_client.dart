import 'dart:convert';
import 'package:http/http.dart' as http;
import 'package:shared_preferences/shared_preferences.dart';

/// ApiClient — единственная точка общения с Go-бэкендом.
///
/// Хранит токены в SharedPreferences (локальное хранилище).
/// Автоматически подставляет Authorization header.
/// При 401 пытается обновить токен через refresh.
class ApiClient {
  // Для Windows localhost работает напрямую.
  // Для Android-эмулятора нужно 10.0.2.2 вместо localhost.
  static const String baseUrl = 'http://127.0.0.1:8080/api';

  String? _accessToken;
  String? _refreshToken;
  DateTime? _expiresAt;

  // ── Tokens ──────────────────────────────────────────

  /// Загружает токены из локального хранилища при старте.
  Future<void> loadTokens() async {
    final prefs = await SharedPreferences.getInstance();
    _accessToken = prefs.getString('access_token');
    _refreshToken = prefs.getString('refresh_token');
    final expiresStr = prefs.getString('expires_at');
    if (expiresStr != null) {
      _expiresAt = DateTime.tryParse(expiresStr);
    }
  }

  /// Сохраняет токены в локальное хранилище.
  Future<void> saveTokens(Map<String, dynamic> tokens) async {
    _accessToken = tokens['access_token'];
    _refreshToken = tokens['refresh_token'];
    _expiresAt = DateTime.tryParse(tokens['expires_at'] ?? '');

    final prefs = await SharedPreferences.getInstance();
    await prefs.setString('access_token', _accessToken!);
    await prefs.setString('refresh_token', _refreshToken!);
    if (tokens['expires_at'] != null) {
      await prefs.setString('expires_at', tokens['expires_at']);
    }
  }

  /// Очищает токены (logout).
  Future<void> clearTokens() async {
    _accessToken = null;
    _refreshToken = null;
    _expiresAt = null;
    final prefs = await SharedPreferences.getInstance();
    await prefs.remove('access_token');
    await prefs.remove('refresh_token');
    await prefs.remove('expires_at');
  }

  /// Проверяет залогинен ли пользователь.
  bool get isLoggedIn => _accessToken != null;

  /// Проверяет истёк ли access token.
  bool get isTokenExpired {
    if (_expiresAt == null) return true;
    // Обновляем за 60 секунд до истечения
    return DateTime.now().isAfter(_expiresAt!.subtract(const Duration(seconds: 60)));
  }

  // ── Auth ────────────────────────────────────────────

  /// Регистрация нового пользователя.
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

  /// Логин.
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

  /// Обновление токенов.
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

  /// Logout.
  Future<void> logout() async {
    try {
      await _post('/auth/logout', {});
    } catch (_) {
      // Даже если запрос не прошёл — чистим токены локально
    }
    await clearTokens();
  }

  /// Профиль текущего пользователя.
  Future<Map<String, dynamic>> getMe() async {
    return await _get('/auth/me');
  }

  /// Обновить профиль.
  Future<Map<String, dynamic>> updateProfile({
    String? displayName,
    String? bio,
    String? avatarUrl,
  }) async {
    return await _patch('/auth/me', {
      if (displayName != null) 'display_name': displayName,
      if (bio != null) 'bio': bio,
      if (avatarUrl != null) 'avatar_url': avatarUrl,
    });
  }

  // ── Users ───────────────────────────────────────────

  /// Поиск пользователей.
  Future<List<dynamic>> searchUsers(String query) async {
    return await _getList('/users/search?q=$query');
  }

  /// Публичный профиль пользователя.
  Future<Map<String, dynamic>> getUserProfile(String userId) async {
    return await _get('/users/$userId');
  }

  // ── Conversations ───────────────────────────────────

  /// Создать личный чат.
  Future<Map<String, dynamic>> createDirectChat(String otherUserId) async {
    return await _post('/conversations/direct', {
      'other_user_id': otherUserId,
    });
  }

  /// Создать групповой чат.
  Future<Map<String, dynamic>> createGroupChat(String title, List<String> memberIds) async {
    return await _post('/conversations/group', {
      'title': title,
      'member_ids': memberIds,
    });
  }

  /// Список чатов текущего пользователя.
  Future<List<dynamic>> getConversations({int limit = 50, int offset = 0}) async {
    return await _getList('/conversations?limit=$limit&offset=$offset');
  }

  /// Отправить сообщение.
  Future<Map<String, dynamic>> sendMessage(String conversationId, String content, {String? replyToId}) async {
    return await _post('/conversations/$conversationId/messages', {
      'content': content,
      'content_type': 'text',
      if (replyToId != null) 'reply_to_id': replyToId,
    });
  }

  /// История сообщений.
  Future<List<dynamic>> getMessages(String conversationId, {int limit = 50, String? before}) async {
    var url = '/conversations/$conversationId/messages?limit=$limit';
    if (before != null) url += '&before=$before';
    return await _getList(url);
  }

  /// Редактировать сообщение.
  Future<Map<String, dynamic>> editMessage(String conversationId, String messageId, String content) async {
    return await _patch('/conversations/$conversationId/messages/$messageId', {
      'content': content,
    });
  }

  /// Удалить сообщение.
  Future<void> deleteMessage(String conversationId, String messageId) async {
    await _delete('/conversations/$conversationId/messages/$messageId');
  }

  /// Отметить прочитанным.
  Future<void> markAsRead(String conversationId, String messageId) async {
    await _post('/conversations/$conversationId/read', {
      'message_id': messageId,
    });
  }

  // ── Channels ────────────────────────────────────────

  /// Создать канал.
  Future<Map<String, dynamic>> createChannel(String name, String description, bool isPublic) async {
    return await _post('/channels', {
      'name': name,
      'description': description,
      'is_public': isPublic,
    });
  }

  /// Мои каналы.
  Future<List<dynamic>> getMyChannels() async {
    return await _getList('/channels/my');
  }

  /// Публичные каналы (discovery).
  Future<List<dynamic>> discoverChannels({int limit = 50}) async {
    return await _getList('/channels/discover?limit=$limit');
  }

  /// Структура канала (категории + комнаты).
  Future<List<dynamic>> getChannelStructure(String channelId) async {
    return await _getList('/channels/$channelId/structure');
  }

  /// Вступить в канал.
  Future<void> joinChannel(String channelId) async {
    await _post('/channels/$channelId/join', {});
  }

  /// Покинуть канал.
  Future<void> leaveChannel(String channelId) async {
    await _post('/channels/$channelId/leave', {});
  }

  /// Отправить сообщение в комнату канала.
  Future<Map<String, dynamic>> sendRoomMessage(String roomId, String content) async {
    return await _post('/rooms/$roomId/messages', {
      'content': content,
    });
  }

  /// История сообщений комнаты.
  Future<List<dynamic>> getRoomMessages(String roomId, {int limit = 50, String? before}) async {
    var url = '/rooms/$roomId/messages?limit=$limit';
    if (before != null) url += '&before=$before';
    return await _getList(url);
  }

  // ── Realtime ────────────────────────────────────────

  /// Получить токен для подключения к Centrifugo.
  Future<String> getRealtimeToken() async {
    final response = await _get('/realtime/token');
    return response['token'];
  }

  // ── HTTP helpers ────────────────────────────────────

  /// Подготавливает headers с auth-токеном.
  Map<String, String> _headers({bool auth = true}) {
    final headers = {'Content-Type': 'application/json'};
    if (auth && _accessToken != null) {
      headers['Authorization'] = 'Bearer $_accessToken';
    }
    return headers;
  }

  /// Обрабатывает ответ: при 401 пробует refresh, при ошибке — бросает.
  Future<Map<String, dynamic>> _handleResponse(http.Response response) async {
    if (response.statusCode == 401 && _refreshToken != null) {
      final refreshed = await refreshTokens();
      if (!refreshed) {
        throw ApiException('Session expired', 'SESSION_EXPIRED');
      }
      // Нужно повторить оригинальный запрос — пробросим ошибку
      // чтобы вызывающий код мог retry
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

  /// Обрабатывает ответ-список.
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

  /// Автоматический refresh при необходимости.
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

/// Ошибка API с кодом (для обработки в UI).
class ApiException implements Exception {
  final String message;
  final String code;
  ApiException(this.message, this.code);

  @override
  String toString() => 'ApiException($code): $message';
}

/// Сигнал что нужно повторить запрос после refresh.
class RetryException implements Exception {}
