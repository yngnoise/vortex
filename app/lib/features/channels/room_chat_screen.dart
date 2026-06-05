/// 📁 Куда: lib/features/channels/room_chat_screen.dart
/// 📝 Переписка в комнате канала (отдельные эндпоинты от direct-чатов)
/// 🔗 Требует: api_client.dart, models/channel.dart

import 'dart:async';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:provider/provider.dart';
import '../../core/api/api_client.dart';
import '../../core/auth/auth_provider.dart';
import '../../core/models/channel.dart';
import '../../core/realtime/realtime_service.dart';

class RoomChatScreen extends StatefulWidget {
  final String roomId;
  final String roomName;
  final String roomType;
  final String channelName;

  const RoomChatScreen({
    super.key,
    required this.roomId,
    required this.roomName,
    required this.roomType,
    required this.channelName,
  });

  @override
  State<RoomChatScreen> createState() => _RoomChatScreenState();
}

class _RoomChatScreenState extends State<RoomChatScreen> {
  final _controller = TextEditingController();
  final _scrollController = ScrollController();
  final _focusNode = FocusNode();
  late final RealtimeService _rt;
  List<ChannelMessage> _messages = [];
  bool _isLoading = true;
  bool _isSending = false;
  bool _hasMore = true;
  bool _hasText = false;
  StreamSubscription? _realtimeSub;

  @override
  void initState() {
    super.initState();
    _loadMessages();
    _scrollController.addListener(_onScroll);
    _controller.addListener(() {
      final has = _controller.text.trim().isNotEmpty;
      if (has != _hasText) setState(() => _hasText = has);
    });

    // Сохраняем ссылку — в dispose() context.read уже нельзя
    _rt = context.read<RealtimeService>();
    _rt.subscribeRoom(widget.roomId);
    _realtimeSub = _rt.events.listen((event) {
      if (event.channel != 'channel:${widget.roomId}') return;
      if (event.type == 'message') {
        final msgData = event.data['data'];
        if (msgData is Map<String, dynamic>) {
          final msg = ChannelMessage.fromJson(msgData);
          if (!_messages.any((m) => m.id == msg.id)) {
            setState(() => _messages.add(msg));
            _scrollToBottom();
          }
        }
      }
    });
  }

  void _onScroll() {
    if (_scrollController.position.pixels <=
        _scrollController.position.minScrollExtent + 100) {
      _loadMore();
    }
  }

  Future<void> _loadMessages() async {
    try {
      final api = context.read<ApiClient>();
      final data = await api.getRoomMessages(widget.roomId);
      setState(() {
        _messages = data
            .map((json) =>
                ChannelMessage.fromJson(json as Map<String, dynamic>))
            .toList()
            .reversed
            .toList();
        _hasMore = data.length >= 50;
        _isLoading = false;
      });
      _scrollToBottom();
    } catch (_) {
      setState(() => _isLoading = false);
    }
  }

  Future<void> _loadMore() async {
    if (_isLoading || !_hasMore || _messages.isEmpty) return;
    setState(() => _isLoading = true);

    try {
      final api = context.read<ApiClient>();
      final data = await api.getRoomMessages(
        widget.roomId,
        before: _messages.first.id,
      );
      final older = data
          .map((json) =>
              ChannelMessage.fromJson(json as Map<String, dynamic>))
          .toList()
          .reversed
          .toList();
      setState(() {
        _messages = [...older, ..._messages];
        _hasMore = data.length >= 50;
        _isLoading = false;
      });
    } catch (_) {
      setState(() => _isLoading = false);
    }
  }

  Future<void> _send() async {
    if (!_hasText) return;
    final text = _controller.text.trim();
    _controller.clear();

    setState(() => _isSending = true);
    try {
      final api = context.read<ApiClient>();
      final data = await api.sendRoomMessage(widget.roomId, text);
      final msg = ChannelMessage.fromJson(data);
      setState(() {
        _messages.add(msg);
        _isSending = false;
      });
      _focusNode.requestFocus();
      _scrollToBottom();
    } catch (_) {
      setState(() => _isSending = false);
    }
  }

  @override
  void dispose() {
    _realtimeSub?.cancel();
    _rt.unsubscribeRoom(widget.roomId);
    _controller.dispose();
    _scrollController.dispose();
    _focusNode.dispose();
    super.dispose();
  }

  void _scrollToBottom() {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (_scrollController.hasClients) {
        _scrollController.animateTo(
          _scrollController.position.maxScrollExtent,
          duration: const Duration(milliseconds: 200),
          curve: Curves.easeOut,
        );
      }
    });
  }

  /// Enter → отправить, Shift+Enter → перенос строки.
  KeyEventResult _handleKey(FocusNode node, KeyEvent event) {
    if (event is! KeyDownEvent) return KeyEventResult.ignored;

    final isEnter = event.logicalKey == LogicalKeyboardKey.enter ||
        event.logicalKey == LogicalKeyboardKey.numpadEnter;

    if (!isEnter) return KeyEventResult.ignored;

    final isShift = HardwareKeyboard.instance.isShiftPressed;

    if (isShift) {
      final text = _controller.text;
      final sel = _controller.selection;
      final newText = text.replaceRange(sel.start, sel.end, '\n');
      _controller.value = TextEditingValue(
        text: newText,
        selection: TextSelection.collapsed(offset: sel.start + 1),
      );
      return KeyEventResult.handled;
    }

    _send();
    return KeyEventResult.handled;
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final currentUserId =
        context.read<AuthProvider>().user?['id'] as String? ?? '';

    return Scaffold(
      appBar: AppBar(
        title: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text('# ${widget.roomName}', style: const TextStyle(fontSize: 16)),
            Text(
              widget.channelName,
              style: TextStyle(fontSize: 12, color: Colors.grey[500]),
            ),
          ],
        ),
      ),
      body: Column(
        children: [
          // Сообщения
          Expanded(
            child: _isLoading && _messages.isEmpty
                ? const Center(child: CircularProgressIndicator())
                : _messages.isEmpty
                    ? Center(
                        child: Text(
                          'Начните обсуждение в #${widget.roomName}',
                          style: TextStyle(color: Colors.grey[500]),
                        ),
                      )
                    : ListView.builder(
                        controller: _scrollController,
                        padding: const EdgeInsets.symmetric(
                            horizontal: 12, vertical: 8),
                        itemCount: _messages.length,
                        itemBuilder: (context, index) {
                          final msg = _messages[index];
                          final isMine = msg.senderId == currentUserId;
                          final showHeader = index == 0 ||
                              _messages[index - 1].senderId != msg.senderId;

                          return _RoomMessageTile(
                            message: msg,
                            showHeader: showHeader,
                            isMine: isMine,
                          );
                        },
                      ),
          ),
          // Ввод
          Container(
            padding: EdgeInsets.only(
              left: 12,
              right: 8,
              top: 8,
              bottom: MediaQuery.of(context).padding.bottom + 8,
            ),
            decoration: BoxDecoration(
              color: theme.colorScheme.surface,
              border: Border(
                top: BorderSide(
                    color: theme.dividerColor.withValues(alpha: 0.3)),
              ),
            ),
            child: Row(
              children: [
                Expanded(
                  child: TextField(
                    controller: _controller,
                    focusNode: _focusNode..onKeyEvent = _handleKey,
                    maxLines: 4,
                    minLines: 1,
                    decoration: InputDecoration(
                      hintText: 'Написать в #${widget.roomName}...',
                      border: OutlineInputBorder(
                        borderRadius: BorderRadius.circular(24),
                        borderSide: BorderSide.none,
                      ),
                      filled: true,
                      fillColor: theme.colorScheme.surfaceContainerHighest,
                      contentPadding: const EdgeInsets.symmetric(
                          horizontal: 16, vertical: 10),
                    ),
                  ),
                ),
                const SizedBox(width: 8),
                IconButton.filled(
                  onPressed: _isSending ? null : (_hasText ? _send : null),
                  icon: _isSending
                      ? const SizedBox(
                          width: 20,
                          height: 20,
                          child: CircularProgressIndicator(strokeWidth: 2),
                        )
                      : const Icon(Icons.send_rounded, size: 20),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

/// Сообщение в стиле Discord — аватар слева, имя + время + текст.
class _RoomMessageTile extends StatelessWidget {
  final ChannelMessage message;
  final bool showHeader;
  final bool isMine;

  const _RoomMessageTile({
    required this.message,
    required this.showHeader,
    required this.isMine,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    if (!showHeader) {
      // Продолжение от того же автора — только текст с отступом
      return Padding(
        padding: const EdgeInsets.only(left: 52, bottom: 2),
        child: Text(message.content, style: const TextStyle(fontSize: 14)),
      );
    }

    return Padding(
      padding: const EdgeInsets.only(top: 12, bottom: 2),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          CircleAvatar(
            radius: 18,
            backgroundImage: message.senderAvatar != null
                ? NetworkImage(message.senderAvatar!)
                : null,
            child: message.senderAvatar == null
                ? Text(
                    message.senderDisplayName.isNotEmpty
                        ? message.senderDisplayName[0].toUpperCase()
                        : '?',
                    style: const TextStyle(fontSize: 14),
                  )
                : null,
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Row(
                  children: [
                    Text(
                      message.senderDisplayName,
                      style: TextStyle(
                        fontWeight: FontWeight.w600,
                        fontSize: 14,
                        color: theme.colorScheme.primary,
                      ),
                    ),
                    const SizedBox(width: 8),
                    Text(
                      _formatTime(message.createdAt),
                      style:
                          TextStyle(fontSize: 11, color: Colors.grey[500]),
                    ),
                    if (message.isEdited)
                      Text(
                        ' (изм.)',
                        style:
                            TextStyle(fontSize: 11, color: Colors.grey[500]),
                      ),
                  ],
                ),
                const SizedBox(height: 2),
                Text(message.content, style: const TextStyle(fontSize: 14)),
              ],
            ),
          ),
        ],
      ),
    );
  }

  String _formatTime(DateTime dt) {
    final now = DateTime.now();
    final time =
        '${dt.hour.toString().padLeft(2, '0')}:${dt.minute.toString().padLeft(2, '0')}';
    if (now.difference(dt).inDays == 0) return time;
    if (now.difference(dt).inDays == 1) return 'вчера $time';
    return '${dt.day}.${dt.month.toString().padLeft(2, '0')} $time';
  }
}
