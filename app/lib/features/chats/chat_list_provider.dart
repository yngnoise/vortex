/// 📁 Куда: lib/features/chats/chat_list_provider.dart
/// 📝 Управляет списком чатов (главный экран мессенджера)
/// 🔗 Требует: api_client.dart, models/conversation.dart

import 'package:flutter/material.dart';
import '../../core/api/api_client.dart';
import '../../core/models/conversation.dart';

class ChatListProvider extends ChangeNotifier {
  final ApiClient api;

  ChatListProvider(this.api);

  List<ConversationPreview> _conversations = [];
  bool _isLoading = false;
  String? _error;

  List<ConversationPreview> get conversations => _conversations;
  bool get isLoading => _isLoading;
  String? get error => _error;

  /// Загрузить список чатов.
  Future<void> load() async {
    _isLoading = true;
    _error = null;
    notifyListeners();

    try {
      final data = await api.getConversations();
      _conversations = data
          .map((json) =>
              ConversationPreview.fromJson(json as Map<String, dynamic>))
          .toList();
      _error = null;
    } on ApiException catch (e) {
      _error = e.message;
    } catch (e) {
      _error = 'Не удалось загрузить чаты';
    }

    _isLoading = false;
    notifyListeners();
  }

  /// Создать личный чат и перейти к нему.
  /// Возвращает conversation_id (нужен для навигации).
  Future<String?> createDirect(String otherUserId) async {
    try {
      final data = await api.createDirectChat(otherUserId);
      final convId = data['id'] as String;
      // Обновим список чтобы новый чат появился
      await load();
      return convId;
    } on ApiException catch (e) {
      _error = e.message;
      notifyListeners();
      return null;
    } catch (_) {
      _error = 'Не удалось создать чат';
      notifyListeners();
      return null;
    }
  }

  /// Обновляет непрочитанные (вызывается при возврате из чата).
  Future<void> refresh() => load();
}
