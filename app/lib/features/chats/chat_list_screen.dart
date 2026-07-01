/// 📁 Куда: lib/features/chats/chat_list_screen.dart
/// 📝 Список чатов — главная вкладка мессенджера
/// 🔗 Требует: chat_list_provider.dart, chat_screen.dart, models

import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../../core/models/conversation.dart';
import '../../core/presence.dart';
import 'chat_list_provider.dart';
import 'chat_screen.dart';

class ChatListScreen extends StatefulWidget {
  const ChatListScreen({super.key});

  @override
  State<ChatListScreen> createState() => _ChatListScreenState();
}

class _ChatListScreenState extends State<ChatListScreen> {
  @override
  void initState() {
    super.initState();
    // Загружаем чаты при первом показе
    Future.microtask(() => context.read<ChatListProvider>().load());
  }

  @override
  Widget build(BuildContext context) {
    final provider = context.watch<ChatListProvider>();

    if (provider.isLoading && provider.conversations.isEmpty) {
      return const Center(child: CircularProgressIndicator());
    }

    if (provider.error != null && provider.conversations.isEmpty) {
      return Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(provider.error!, style: TextStyle(color: Colors.grey[600])),
            const SizedBox(height: 16),
            FilledButton.tonal(
              onPressed: () => provider.load(),
              child: const Text('Повторить'),
            ),
          ],
        ),
      );
    }

    if (provider.conversations.isEmpty) {
      return Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.chat_bubble_outline, size: 64, color: Colors.grey[400]),
            const SizedBox(height: 16),
            Text(
              'Пока нет чатов',
              style: TextStyle(fontSize: 18, color: Colors.grey[600]),
            ),
            const SizedBox(height: 8),
            Text(
              'Найдите пользователя через поиск',
              style: TextStyle(color: Colors.grey[500]),
            ),
          ],
        ),
      );
    }

    return RefreshIndicator(
      onRefresh: () => provider.refresh(),
      child: ListView.builder(
        itemCount: provider.conversations.length,
        itemBuilder: (context, index) {
          final conv = provider.conversations[index];
          return _ConversationTile(
            conv: conv,
            onTap: () => _openChat(context, conv),
          );
        },
      ),
    );
  }

  void _openChat(BuildContext context, ConversationPreview conv) async {
    await Navigator.push(
      context,
      MaterialPageRoute(
        builder: (_) => ChatScreen(
          conversationId: conv.id,
          title: conv.displayName,
          avatarUrl: conv.displayAvatar,
          otherUserId: conv.isDirect ? conv.otherUserId : null,
          otherLastSeen: conv.isDirect ? conv.otherLastSeen : null,
          isGroup: conv.isGroup,
        ),
      ),
    );
    // Обновляем список при возврате (непрочитанные)
    if (mounted) {
      context.read<ChatListProvider>().refresh();
    }
  }
}

class _ConversationTile extends StatelessWidget {
  final ConversationPreview conv;
  final VoidCallback onTap;

  const _ConversationTile({required this.conv, required this.onTap});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final lastMsg = conv.lastMessage;

    return ListTile(
      leading: Stack(
        children: [
          CircleAvatar(
            radius: 24,
            backgroundImage: conv.displayAvatar != null
                ? NetworkImage(conv.displayAvatar!)
                : null,
            child: conv.displayAvatar == null
                ? Text(
                    conv.displayName.isNotEmpty
                        ? conv.displayName[0].toUpperCase()
                        : '?',
                    style: const TextStyle(fontSize: 18),
                  )
                : null,
          ),
          if (conv.isDirect && isOnline(conv.otherLastSeen))
            Positioned(
              right: 0,
              bottom: 0,
              child: Container(
                width: 14,
                height: 14,
                decoration: BoxDecoration(
                  color: Colors.green,
                  shape: BoxShape.circle,
                  border:
                      Border.all(color: theme.scaffoldBackgroundColor, width: 2),
                ),
              ),
            ),
        ],
      ),
      title: Row(
        children: [
          Expanded(
            child: Text(
              conv.displayName,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: TextStyle(
                fontWeight:
                    conv.hasUnread ? FontWeight.w600 : FontWeight.normal,
              ),
            ),
          ),
          if (lastMsg != null)
            Text(
              _formatTime(lastMsg.createdAt),
              style: TextStyle(
                fontSize: 12,
                color: conv.hasUnread
                    ? theme.colorScheme.primary
                    : Colors.grey[500],
              ),
            ),
        ],
      ),
      subtitle: Row(
        children: [
          Expanded(
            child: Text(
              lastMsg != null
                  ? '${lastMsg.senderDisplayName}: ${lastMsg.content}'
                  : 'Нет сообщений',
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: TextStyle(
                color: Colors.grey[600],
                fontWeight:
                    conv.hasUnread ? FontWeight.w500 : FontWeight.normal,
              ),
            ),
          ),
          if (conv.hasUnread)
            Container(
              margin: const EdgeInsets.only(left: 8),
              padding: const EdgeInsets.symmetric(horizontal: 7, vertical: 2),
              decoration: BoxDecoration(
                color: theme.colorScheme.primary,
                borderRadius: BorderRadius.circular(12),
              ),
              child: Text(
                conv.unreadCount > 99 ? '99+' : '${conv.unreadCount}',
                style: const TextStyle(
                  color: Colors.white,
                  fontSize: 11,
                  fontWeight: FontWeight.w600,
                ),
              ),
            ),
        ],
      ),
      onTap: onTap,
    );
  }

  String _formatTime(DateTime dt) {
    final now = DateTime.now();
    final diff = now.difference(dt);

    if (diff.inDays == 0) {
      return '${dt.hour.toString().padLeft(2, '0')}:${dt.minute.toString().padLeft(2, '0')}';
    }
    if (diff.inDays == 1) return 'вчера';
    if (diff.inDays < 7) {
      const days = ['пн', 'вт', 'ср', 'чт', 'пт', 'сб', 'вс'];
      return days[dt.weekday - 1];
    }
    return '${dt.day}.${dt.month.toString().padLeft(2, '0')}';
  }
}
