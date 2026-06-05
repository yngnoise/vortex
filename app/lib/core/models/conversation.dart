/// 📁 Куда: lib/core/models/conversation.dart
/// 📝 Чат (direct/group) + превью для списка чатов
/// 🔗 Требует: message.dart

import 'message.dart';

class Conversation {
  final String id;
  final String type; // "direct" или "group"
  final String? title; // null для direct
  final String? avatarUrl;
  final String? createdBy;
  final DateTime createdAt;

  const Conversation({
    required this.id,
    required this.type,
    this.title,
    this.avatarUrl,
    this.createdBy,
    required this.createdAt,
  });

  bool get isDirect => type == 'direct';
  bool get isGroup => type == 'group';

  factory Conversation.fromJson(Map<String, dynamic> json) {
    return Conversation(
      id: json['id'] as String,
      type: json['type'] as String,
      title: json['title'] as String?,
      avatarUrl: json['avatar_url'] as String?,
      createdBy: json['created_by'] as String?,
      createdAt: json['created_at'] != null
          ? DateTime.parse(json['created_at'] as String)
          : DateTime.now(),
    );
  }
}

/// ConversationPreview — элемент списка чатов на главном экране.
/// Go: messaging.ConversationPreview
class ConversationPreview {
  final String id;
  final String type;
  final String? title;
  final String? avatarUrl;
  final String? createdBy;
  final DateTime createdAt;
  final Message? lastMessage;
  final int unreadCount;
  final int memberCount;
  // Данные собеседника в direct-чатах
  final String? otherUserId;
  final String? otherUsername;
  final String? otherDisplayName;
  final String? otherAvatarUrl;
  final DateTime? otherLastSeen;

  const ConversationPreview({
    required this.id,
    required this.type,
    this.title,
    this.avatarUrl,
    this.createdBy,
    required this.createdAt,
    this.lastMessage,
    this.unreadCount = 0,
    this.memberCount = 0,
    this.otherUserId,
    this.otherUsername,
    this.otherDisplayName,
    this.otherAvatarUrl,
    this.otherLastSeen,
  });

  bool get isDirect => type == 'direct';
  bool get isGroup => type == 'group';
  bool get hasUnread => unreadCount > 0;

  /// Название для отображения: для direct — имя собеседника, для group — title
  String get displayName {
    if (isDirect) {
      return otherDisplayName ?? otherUsername ?? 'Пользователь';
    }
    return title ?? 'Групповой чат';
  }

  /// Аватар для отображения
  String? get displayAvatar {
    if (isDirect) return otherAvatarUrl;
    return avatarUrl;
  }

  factory ConversationPreview.fromJson(Map<String, dynamic> json) {
    return ConversationPreview(
      id: json['id'] as String,
      type: json['type'] as String,
      title: json['title'] as String?,
      avatarUrl: json['avatar_url'] as String?,
      createdBy: json['created_by'] as String?,
      createdAt: json['created_at'] != null
          ? DateTime.parse(json['created_at'] as String)
          : DateTime.now(),
      lastMessage: json['last_message'] != null
          ? Message.fromJson(json['last_message'] as Map<String, dynamic>)
          : null,
      unreadCount: json['unread_count'] as int? ?? 0,
      memberCount: json['member_count'] as int? ?? 0,
      otherUserId: json['other_user_id'] as String?,
      otherUsername: json['other_username'] as String?,
      otherDisplayName: json['other_display_name'] as String?,
      otherAvatarUrl: json['other_avatar_url'] as String?,
      otherLastSeen: json['other_last_seen'] != null
          ? DateTime.parse(json['other_last_seen'] as String)
          : null,
    );
  }
}

/// Участник чата. Go: messaging.ConversationMember
class ConversationMember {
  final String conversationId;
  final String userId;
  final String username;
  final String displayName;
  final String? avatarUrl;
  final String role; // "owner", "admin", "member"
  final DateTime joinedAt;

  const ConversationMember({
    required this.conversationId,
    required this.userId,
    required this.username,
    required this.displayName,
    this.avatarUrl,
    required this.role,
    required this.joinedAt,
  });

  factory ConversationMember.fromJson(Map<String, dynamic> json) {
    return ConversationMember(
      conversationId: json['conversation_id'] as String,
      userId: json['user_id'] as String,
      username: json['username'] as String,
      displayName: json['display_name'] as String? ?? '',
      avatarUrl: json['avatar_url'] as String?,
      role: json['role'] as String,
      joinedAt: DateTime.parse(json['joined_at'] as String),
    );
  }
}
