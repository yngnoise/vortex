/// 📁 Куда: lib/features/chats/chat_screen.dart
/// 📝 Экран переписки — сообщения + поле ввода
/// 🔗 Требует: chat_provider.dart, api_client.dart, auth_provider.dart

import 'dart:async';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:file_picker/file_picker.dart';
import 'package:provider/provider.dart';
import '../../core/api/api_client.dart';
import '../../core/auth/auth_provider.dart';
import '../../core/models/message.dart';
import '../../core/realtime/realtime_service.dart';
import 'chat_provider.dart';

class ChatScreen extends StatefulWidget {
  final String conversationId;
  final String title;
  final String? avatarUrl;

  const ChatScreen({
    super.key,
    required this.conversationId,
    required this.title,
    this.avatarUrl,
  });

  @override
  State<ChatScreen> createState() => _ChatScreenState();
}

class _ChatScreenState extends State<ChatScreen> {
  late final ChatProvider _chatProvider;
  late final RealtimeService _rt;
  final _controller = TextEditingController();
  final _scrollController = ScrollController();
  StreamSubscription? _realtimeSub;

  @override
  void initState() {
    super.initState();
    _chatProvider = ChatProvider(
      api: context.read<ApiClient>(),
      conversationId: widget.conversationId,
    );
    _chatProvider.loadMessages().then((_) => _scrollToBottom());
    _chatProvider.addListener(_onMessagesChanged);
    _scrollController.addListener(_onScroll);

    // Сохраняем ссылку — в dispose() context.read уже нельзя
    _rt = context.read<RealtimeService>();
    _rt.subscribeChat(widget.conversationId);
    _realtimeSub = _rt.events.listen((event) {
      if (event.channel != 'chat:${widget.conversationId}') return;
      if (event.type == 'message') {
        final msgData = event.data['data'];
        if (msgData is Map<String, dynamic>) {
          final msg = Message.fromJson(msgData);
          _chatProvider.addRealtimeMessage(msg);
        }
      }
    });
  }

  void _onMessagesChanged() {
    if (_scrollController.hasClients) {
      final pos = _scrollController.position;
      if (pos.pixels >= pos.maxScrollExtent - 150) {
        _scrollToBottom();
      }
    }
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

  void _onScroll() {
    if (_scrollController.position.pixels <=
        _scrollController.position.minScrollExtent + 100) {
      _chatProvider.loadMore();
    }
  }

  @override
  void dispose() {
    _chatProvider.removeListener(_onMessagesChanged);
    _realtimeSub?.cancel();
    _rt.unsubscribeChat(widget.conversationId);
    _chatProvider.markLastRead();
    _controller.dispose();
    _scrollController.dispose();
    _chatProvider.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return ChangeNotifierProvider.value(
      value: _chatProvider,
      child: Scaffold(
        appBar: AppBar(
          titleSpacing: 0,
          title: Row(
            children: [
              CircleAvatar(
                radius: 18,
                backgroundImage: widget.avatarUrl != null
                    ? NetworkImage(widget.avatarUrl!)
                    : null,
                child: widget.avatarUrl == null
                    ? Text(widget.title.isNotEmpty
                        ? widget.title[0].toUpperCase()
                        : '?')
                    : null,
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Text(
                  widget.title,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
              ),
            ],
          ),
        ),
        body: Column(
          children: [
            Expanded(child: _MessageList(scrollController: _scrollController)),
            const _MessageInput(),
          ],
        ),
      ),
    );
  }
}

// ── Список сообщений ─────────────────────────────────

class _MessageList extends StatelessWidget {
  final ScrollController scrollController;
  const _MessageList({required this.scrollController});

  @override
  Widget build(BuildContext context) {
    final provider = context.watch<ChatProvider>();
    final currentUserId =
        context.read<AuthProvider>().user?['id'] as String? ?? '';

    if (provider.isLoading && provider.messages.isEmpty) {
      return const Center(child: CircularProgressIndicator());
    }

    if (provider.error != null && provider.messages.isEmpty) {
      return Center(
        child: Text(provider.error!, style: TextStyle(color: Colors.grey[600])),
      );
    }

    if (provider.messages.isEmpty) {
      return Center(
        child: Text(
          'Напишите первое сообщение!',
          style: TextStyle(color: Colors.grey[500]),
        ),
      );
    }

    return ListView.builder(
      controller: scrollController,
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
      itemCount: provider.messages.length,
      itemBuilder: (context, index) {
        final msg = provider.messages[index];
        final isMine = msg.senderId == currentUserId;
        final showSender = !isMine &&
            (index == 0 ||
                provider.messages[index - 1].senderId != msg.senderId);

        return _MessageBubble(
          message: msg,
          isMine: isMine,
          showSender: showSender,
          onLongPress: isMine
              ? () => _showMessageActions(context, provider, msg)
              : null,
        );
      },
    );
  }

  void _showMessageActions(
      BuildContext context, ChatProvider provider, Message msg) {
    showModalBottomSheet(
      context: context,
      builder: (ctx) => SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            ListTile(
              leading: const Icon(Icons.edit),
              title: const Text('Редактировать'),
              onTap: () {
                Navigator.pop(ctx);
                _showEditDialog(context, provider, msg);
              },
            ),
            ListTile(
              leading: const Icon(Icons.delete, color: Colors.red),
              title:
                  const Text('Удалить', style: TextStyle(color: Colors.red)),
              onTap: () {
                Navigator.pop(ctx);
                provider.delete(msg.id);
              },
            ),
          ],
        ),
      ),
    );
  }

  void _showEditDialog(
      BuildContext context, ChatProvider provider, Message msg) {
    final controller = TextEditingController(text: msg.content);
    showDialog(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Редактировать'),
        content: TextField(
          controller: controller,
          maxLines: null,
          autofocus: true,
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx),
            child: const Text('Отмена'),
          ),
          FilledButton(
            onPressed: () {
              provider.edit(msg.id, controller.text);
              Navigator.pop(ctx);
            },
            child: const Text('Сохранить'),
          ),
        ],
      ),
    );
  }
}

// ── Пузырь сообщения ─────────────────────────────────

class _MessageBubble extends StatelessWidget {
  final Message message;
  final bool isMine;
  final bool showSender;
  final VoidCallback? onLongPress;

  const _MessageBubble({
    required this.message,
    required this.isMine,
    required this.showSender,
    this.onLongPress,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Align(
      alignment: isMine ? Alignment.centerRight : Alignment.centerLeft,
      child: GestureDetector(
        onLongPress: onLongPress,
        child: Container(
          constraints: BoxConstraints(
            maxWidth: MediaQuery.of(context).size.width * 0.75,
          ),
          margin: EdgeInsets.only(
            top: showSender ? 12 : 2,
            bottom: 2,
          ),
          padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 8),
          decoration: BoxDecoration(
            color: isMine
                ? theme.colorScheme.primary.withValues(alpha: 0.15)
                : theme.colorScheme.surfaceContainerHighest,
            borderRadius: BorderRadius.circular(16),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              if (showSender)
                Padding(
                  padding: const EdgeInsets.only(bottom: 4),
                  child: Text(
                    message.senderDisplayName,
                    style: TextStyle(
                      fontSize: 12,
                      fontWeight: FontWeight.w600,
                      color: theme.colorScheme.primary,
                    ),
                  ),
                ),
              if (message.hasAttachments)
                ...message.attachments.map(
                  (a) => Padding(
                    padding: const EdgeInsets.only(bottom: 6),
                    child: _AttachmentView(attachment: a),
                  ),
                ),
              if (message.content.isNotEmpty)
                Text(
                  message.content,
                  style: const TextStyle(fontSize: 15),
                ),
              const SizedBox(height: 2),
              Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Text(
                    '${message.createdAt.hour.toString().padLeft(2, '0')}:${message.createdAt.minute.toString().padLeft(2, '0')}',
                    style: TextStyle(fontSize: 11, color: Colors.grey[500]),
                  ),
                  if (message.isEdited)
                    Text(
                      ' • изм.',
                      style: TextStyle(fontSize: 11, color: Colors.grey[500]),
                    ),
                ],
              ),
            ],
          ),
        ),
      ),
    );
  }
}

// ── Поле ввода ───────────────────────────────────────

class _MessageInput extends StatefulWidget {
  const _MessageInput();

  @override
  State<_MessageInput> createState() => _MessageInputState();
}

class _MessageInputState extends State<_MessageInput> {
  final _controller = TextEditingController();
  final _focusNode = FocusNode();
  bool _hasText = false;
  bool _uploading = false;

  @override
  void initState() {
    super.initState();
    _controller.addListener(() {
      final has = _controller.text.trim().isNotEmpty;
      if (has != _hasText) setState(() => _hasText = has);
    });
  }

  @override
  void dispose() {
    _controller.dispose();
    _focusNode.dispose();
    super.dispose();
  }

  void _send() async {
    if (!_hasText) return;
    final text = _controller.text;
    _controller.clear();

    final provider = context.read<ChatProvider>();
    await provider.send(text);
    _focusNode.requestFocus();
  }

  /// Выбрать файл, загрузить и отправить как вложение
  /// (текст в поле ввода становится подписью).
  Future<void> _pickAndSend() async {
    final result = await FilePicker.platform.pickFiles(withData: true);
    if (result == null || result.files.isEmpty) return;
    final picked = result.files.first;
    final bytes = picked.bytes;
    if (bytes == null) return;

    if (!mounted) return;
    setState(() => _uploading = true);

    final api = context.read<ApiClient>();
    final provider = context.read<ChatProvider>();
    final caption = _controller.text.trim();

    try {
      final info = await api.uploadMedia(bytes, picked.name);
      final ok = await provider.send(
        caption,
        attachments: [
          {'key': info['key'], 'file_name': picked.name},
        ],
      );
      if (ok) _controller.clear();
    } catch (_) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Не удалось отправить файл')),
        );
      }
    } finally {
      if (mounted) setState(() => _uploading = false);
    }
  }

  /// Enter → отправить, Shift+Enter → перенос строки.
  KeyEventResult _handleKey(FocusNode node, KeyEvent event) {
    if (event is! KeyDownEvent) return KeyEventResult.ignored;

    final isEnter = event.logicalKey == LogicalKeyboardKey.enter ||
        event.logicalKey == LogicalKeyboardKey.numpadEnter;

    if (!isEnter) return KeyEventResult.ignored;

    final isShift = HardwareKeyboard.instance.isShiftPressed;

    if (isShift) {
      // Shift+Enter → вставляем перенос строки вручную
      final text = _controller.text;
      final sel = _controller.selection;
      final newText = text.replaceRange(sel.start, sel.end, '\n');
      _controller.value = TextEditingValue(
        text: newText,
        selection: TextSelection.collapsed(offset: sel.start + 1),
      );
      return KeyEventResult.handled;
    }

    // Enter без Shift → отправить
    _send();
    return KeyEventResult.handled;
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final sending = context.select<ChatProvider, bool>((p) => p.isSending);

    return Container(
      padding: EdgeInsets.only(
        left: 12,
        right: 8,
        top: 8,
        bottom: MediaQuery.of(context).padding.bottom + 8,
      ),
      decoration: BoxDecoration(
        color: theme.colorScheme.surface,
        border: Border(
          top: BorderSide(color: theme.dividerColor.withValues(alpha: 0.3)),
        ),
      ),
      child: Row(
        children: [
          IconButton(
            onPressed: (sending || _uploading) ? null : _pickAndSend,
            tooltip: 'Прикрепить файл',
            icon: _uploading
                ? const SizedBox(
                    width: 20,
                    height: 20,
                    child: CircularProgressIndicator(strokeWidth: 2),
                  )
                : const Icon(Icons.attach_file),
          ),
          Expanded(
            child: TextField(
              controller: _controller,
              focusNode: _focusNode..onKeyEvent = _handleKey,
              maxLines: 4,
              minLines: 1,
              textInputAction: TextInputAction.newline,
              decoration: InputDecoration(
                hintText: 'Сообщение...',
                border: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(24),
                  borderSide: BorderSide.none,
                ),
                filled: true,
                fillColor: theme.colorScheme.surfaceContainerHighest,
                contentPadding:
                    const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
              ),
            ),
          ),
          const SizedBox(width: 8),
          IconButton.filled(
            onPressed: sending ? null : (_hasText ? _send : null),
            icon: sending
                ? const SizedBox(
                    width: 20,
                    height: 20,
                    child: CircularProgressIndicator(strokeWidth: 2),
                  )
                : const Icon(Icons.send_rounded, size: 20),
          ),
        ],
      ),
    );
  }
}

// ── Вложение в пузыре сообщения ──────────────────────

class _AttachmentView extends StatelessWidget {
  final Attachment attachment;
  const _AttachmentView({required this.attachment});

  @override
  Widget build(BuildContext context) {
    if (attachment.isImage) {
      return ClipRRect(
        borderRadius: BorderRadius.circular(10),
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxHeight: 240, maxWidth: 260),
          child: Image.network(
            attachment.fileUrl,
            fit: BoxFit.cover,
            errorBuilder: (_, _, _) => _fileChip(context),
            loadingBuilder: (ctx, child, progress) {
              if (progress == null) return child;
              return const SizedBox(
                width: 120,
                height: 120,
                child: Center(child: CircularProgressIndicator(strokeWidth: 2)),
              );
            },
          ),
        ),
      );
    }
    return _fileChip(context);
  }

  Widget _fileChip(BuildContext context) {
    final theme = Theme.of(context);
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 8),
      decoration: BoxDecoration(
        color: theme.colorScheme.surface,
        borderRadius: BorderRadius.circular(10),
        border: Border.all(color: theme.dividerColor.withValues(alpha: 0.4)),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(_iconForType(attachment.fileType),
              size: 28, color: theme.colorScheme.primary),
          const SizedBox(width: 8),
          Flexible(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  attachment.fileName ?? 'Файл',
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(fontSize: 13, fontWeight: FontWeight.w500),
                ),
                Text(
                  _formatSize(attachment.fileSize),
                  style: TextStyle(fontSize: 11, color: Colors.grey[600]),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  IconData _iconForType(String type) {
    switch (type) {
      case 'video':
        return Icons.videocam;
      case 'audio':
        return Icons.audiotrack;
      case 'document':
        return Icons.description;
      default:
        return Icons.insert_drive_file;
    }
  }

  static String _formatSize(int bytes) {
    if (bytes < 1024) return '$bytes B';
    if (bytes < 1024 * 1024) return '${(bytes / 1024).toStringAsFixed(1)} KB';
    return '${(bytes / (1024 * 1024)).toStringAsFixed(1)} MB';
  }
}
