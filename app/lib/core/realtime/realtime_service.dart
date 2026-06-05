import 'dart:async';
import 'dart:convert';
import 'package:centrifuge/centrifuge.dart' as centrifuge;
import '../api/api_client.dart';

class RealtimeEvent {
  final String channel;
  final String type;
  final Map<String, dynamic> data;

  const RealtimeEvent({
    required this.channel,
    required this.type,
    required this.data,
  });
}

class RealtimeService {
  final ApiClient api;

  static const String _wsUrl = 'ws://127.0.0.1:8001/connection/websocket';

  centrifuge.Client? _client;
  final Map<String, centrifuge.Subscription> _subscriptions = {};
  final _eventController = StreamController<RealtimeEvent>.broadcast();
  Stream<RealtimeEvent> get events => _eventController.stream;

  bool _isConnected = false;
  bool get isConnected => _isConnected;

  // Каналы, запрошенные до того как _client был создан — выполним при подключении
  final Set<String> _pendingSubscriptions = {};

  RealtimeService(this.api);

  Future<void> connect() async {
    if (_client != null) return;

    try {
      final token = await api.getRealtimeToken();

      _client = centrifuge.createClient(
        _wsUrl,
        centrifuge.ClientConfig(
          token: token,
          getToken: (centrifuge.ConnectionTokenEvent event) async {
            return await api.getRealtimeToken();
          },
        ),
      );

      _client!.connected.listen((event) {
        _isConnected = true;
        _flushPending();
      });

      _client!.disconnected.listen((event) {
        _isConnected = false;
      });

      _client!.connect();
    } catch (e) {
      _isConnected = false;
    }
  }

  void disconnect() {
    _pendingSubscriptions.clear();
    for (final sub in _subscriptions.values) {
      sub.unsubscribe();
    }
    _subscriptions.clear();
    _client?.disconnect();
    _client = null;
    _isConnected = false;
  }

  Future<void> subscribeUser(String userId) async {
    await _subscribe('user:$userId');
  }

  Future<void> subscribeChat(String conversationId) async {
    await _subscribe('chat:$conversationId');
  }

  void unsubscribeChat(String conversationId) {
    _unsubscribe('chat:$conversationId');
  }

  Future<void> subscribeRoom(String roomId) async {
    await _subscribe('channel:$roomId');
  }

  void unsubscribeRoom(String roomId) {
    _unsubscribe('channel:$roomId');
  }

  // Если клиент ещё не создан — откладываем подписку до подключения
  Future<void> _subscribe(String channel) async {
    if (_subscriptions.containsKey(channel)) return;

    if (_client == null) {
      _pendingSubscriptions.add(channel);
      return;
    }

    try {
      final token = await api.getSubscriptionToken(channel);

      // После await _client мог стать null (disconnect вызвали пока ждали токен)
      if (_client == null) return;

      final sub = _client!.newSubscription(
        channel,
        centrifuge.SubscriptionConfig(
          token: token,
          getToken: (centrifuge.SubscriptionTokenEvent event) async {
            return await api.getSubscriptionToken(channel);
          },
        ),
      );

      sub.publication.listen((event) => _onPublication(channel, event));

      sub.subscribe();
      _subscriptions[channel] = sub;
    } catch (_) {
      // Молча — подписка не удалась, сообщения придут при следующем входе в чат
    }
  }

  void _unsubscribe(String channel) {
    _pendingSubscriptions.remove(channel);
    final sub = _subscriptions.remove(channel);
    sub?.unsubscribe();
  }

  // Выполняем все отложенные подписки после установки соединения
  void _flushPending() {
    final pending = Set<String>.from(_pendingSubscriptions);
    _pendingSubscriptions.clear();
    for (final channel in pending) {
      _subscribe(channel);
    }
  }

  void _onPublication(String channel, centrifuge.PublicationEvent event) {
    try {
      final raw = utf8.decode(event.data);
      final data = jsonDecode(raw) as Map<String, dynamic>;
      final type = data['type'] as String? ?? 'unknown';

      _eventController.add(RealtimeEvent(
        channel: channel,
        type: type,
        data: data,
      ));
    } catch (_) {
      // Невалидный JSON — игнорируем
    }
  }

  void dispose() {
    disconnect();
    _eventController.close();
  }
}
