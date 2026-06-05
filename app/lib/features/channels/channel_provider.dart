/// 📁 Куда: lib/features/channels/channel_provider.dart
/// 📝 Управляет каналами: мои + discover + вступление/выход
/// 🔗 Требует: api_client.dart, models/channel.dart

import 'package:flutter/material.dart';
import '../../core/api/api_client.dart';
import '../../core/models/channel.dart';

class ChannelProvider extends ChangeNotifier {
  final ApiClient api;

  ChannelProvider(this.api);

  List<ChannelPreview> _myChannels = [];
  List<Channel> _discover = [];
  bool _isLoading = false;
  String? _error;

  List<ChannelPreview> get myChannels => _myChannels;
  List<Channel> get discover => _discover;
  bool get isLoading => _isLoading;
  String? get error => _error;

  /// Загрузить мои каналы.
  Future<void> loadMyChannels() async {
    _isLoading = true;
    _error = null;
    notifyListeners();

    try {
      final data = await api.getMyChannels();
      _myChannels = data
          .map((json) =>
              ChannelPreview.fromJson(json as Map<String, dynamic>))
          .toList();
    } on ApiException catch (e) {
      _error = e.message;
    } catch (_) {
      _error = 'Не удалось загрузить каналы';
    }

    _isLoading = false;
    notifyListeners();
  }

  /// Загрузить публичные каналы для вступления.
  Future<void> loadDiscover() async {
    try {
      final data = await api.discoverChannels();
      _discover = data
          .map((json) => Channel.fromJson(json as Map<String, dynamic>))
          .toList();
      notifyListeners();
    } catch (_) {}
  }

  /// Вступить в канал.
  Future<bool> join(String channelId) async {
    try {
      await api.joinChannel(channelId);
      await loadMyChannels();
      return true;
    } on ApiException catch (e) {
      _error = e.message;
      notifyListeners();
      return false;
    } catch (_) {
      return false;
    }
  }

  /// Покинуть канал.
  Future<bool> leave(String channelId) async {
    try {
      await api.leaveChannel(channelId);
      _myChannels.removeWhere((c) => c.id == channelId);
      notifyListeners();
      return true;
    } catch (_) {
      return false;
    }
  }

  /// Создать канал.
  Future<String?> create(
      String name, String description, bool isPublic) async {
    try {
      final data = await api.createChannel(name, description, isPublic);
      await loadMyChannels();
      return data['id'] as String;
    } on ApiException catch (e) {
      _error = e.message;
      notifyListeners();
      return null;
    } catch (_) {
      _error = 'Не удалось создать канал';
      notifyListeners();
      return null;
    }
  }

  Future<void> refresh() => loadMyChannels();
}
