/// 📁 Куда: lib/features/chats/chat_provider.dart
/// 📝 Управляет сообщениями внутри одного чата
/// 🔗 Требует: api_client.dart, models/message.dart

import 'package:flutter/material.dart';
import '../../core/api/api_client.dart';
import '../../core/models/message.dart';

class ChatProvider extends ChangeNotifier {
  final ApiClient api;
  final String conversationId;
  final String? currentUserId;

  ChatProvider({
    required this.api,
    required this.conversationId,
    this.currentUserId,
  });

  List<Message> _messages = [];
  bool _isLoading = false;
  bool _isSending = false;
  bool _hasMore = true;
  String? _error;
  DateTime? _lastTyping;
  DateTime? _otherLastReadAt;

  List<Message> get messages => _messages;
  bool get isLoading => _isLoading;
  bool get isSending => _isSending;
  bool get hasMore => _hasMore;
  String? get error => _error;
  DateTime? get otherLastReadAt => _otherLastReadAt;

  /// Загрузить последние сообщения.
  Future<void> loadMessages() async {
    _isLoading = true;
    _error = null;
    notifyListeners();

    try {
      final data = await api.getMessages(conversationId);
      _messages = data
          .map((json) => Message.fromJson(json as Map<String, dynamic>))
          .toList();
      // API возвращает DESC, разворачиваем для отображения
      _messages = _messages.reversed.toList();
      _hasMore = data.length >= 50;
    } on ApiException catch (e) {
      _error = e.message;
    } catch (_) {
      _error = 'Не удалось загрузить сообщения';
    }

    _isLoading = false;
    notifyListeners();
  }

  /// Подгрузить старые сообщения (пагинация вверх).
  Future<void> loadMore() async {
    if (_isLoading || !_hasMore || _messages.isEmpty) return;

    _isLoading = true;
    notifyListeners();

    try {
      final oldestId = _messages.first.id;
      final data =
          await api.getMessages(conversationId, before: oldestId);
      final older = data
          .map((json) => Message.fromJson(json as Map<String, dynamic>))
          .toList()
          .reversed
          .toList();

      _messages = [...older, ..._messages];
      _hasMore = data.length >= 50;
    } catch (_) {
      // Молча — пользователь увидит что загрузка остановилась
    }

    _isLoading = false;
    notifyListeners();
  }

  /// Отправить сообщение. Можно с вложениями (attachments) и/или текстом.
  Future<bool> send(
    String content, {
    String? replyToId,
    List<Map<String, dynamic>>? attachments,
  }) async {
    final hasAttachments = attachments != null && attachments.isNotEmpty;
    if (content.trim().isEmpty && !hasAttachments) return false;

    _isSending = true;
    notifyListeners();

    try {
      final data = await api.sendMessage(
        conversationId,
        content.trim(),
        replyToId: replyToId,
        attachments: attachments,
      );
      addRealtimeMessage(Message.fromJson(data));
      _isSending = false;
      notifyListeners();
      return true;
    } on ApiException catch (e) {
      _error = e.message;
      _isSending = false;
      notifyListeners();
      return false;
    } catch (_) {
      _error = 'Не удалось отправить';
      _isSending = false;
      notifyListeners();
      return false;
    }
  }

  /// Сообщить серверу, что пользователь печатает (не чаще раза в 3 секунды).
  void notifyTyping() {
    final now = DateTime.now();
    if (_lastTyping != null && now.difference(_lastTyping!).inSeconds < 3) {
      return;
    }
    _lastTyping = now;
    api.sendTyping(conversationId);
  }

  /// Редактировать сообщение.
  Future<bool> edit(String messageId, String newContent) async {
    try {
      final data =
          await api.editMessage(conversationId, messageId, newContent);
      final updated = Message.fromJson(data);
      final idx = _messages.indexWhere((m) => m.id == messageId);
      if (idx != -1) {
        _messages[idx] = updated;
        notifyListeners();
      }
      return true;
    } catch (_) {
      return false;
    }
  }

  /// Удалить сообщение.
  Future<bool> delete(String messageId) async {
    try {
      await api.deleteMessage(conversationId, messageId);
      _messages.removeWhere((m) => m.id == messageId);
      notifyListeners();
      return true;
    } catch (_) {
      return false;
    }
  }

  /// Отметить последнее сообщение прочитанным.
  Future<void> markLastRead() async {
    if (_messages.isEmpty) return;
    try {
      await api.markAsRead(conversationId, _messages.last.id);
    } catch (_) {}
  }

  /// Загрузить позиции прочтения собеседника (галочки) при открытии чата.
  Future<void> loadReadState() async {
    try {
      final data = await api.getReadState(conversationId);
      DateTime? maxOther;
      for (final s in data) {
        final m = s as Map<String, dynamic>;
        if (m['user_id'] == currentUserId) continue;
        final ls = m['last_read_at'];
        if (ls == null) continue;
        final dt = DateTime.tryParse(ls as String);
        if (dt != null && (maxOther == null || dt.isAfter(maxOther))) {
          maxOther = dt;
        }
      }
      if (maxOther != null) {
        _otherLastReadAt = maxOther;
        notifyListeners();
      }
    } catch (_) {}
  }

  /// Применить событие прочтения от собеседника (realtime).
  void applyRead(String userId, DateTime lastReadAt) {
    if (userId == currentUserId) return;
    if (_otherLastReadAt == null || lastReadAt.isAfter(_otherLastReadAt!)) {
      _otherLastReadAt = lastReadAt;
      notifyListeners();
    }
  }

  /// Добавить сообщение из realtime (когда подключим Centrifugo).
  void addRealtimeMessage(Message msg) {
    if (_messages.any((m) => m.id == msg.id)) return;
    _messages.add(msg);
    notifyListeners();
    // Чужое сообщение, пока чат открыт — сразу помечаем прочитанным.
    if (msg.senderId != currentUserId) markLastRead();
  }
}
