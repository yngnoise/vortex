/// 📁 Куда: lib/core/models/channel.dart
/// 📝 Каналы (аналог Discord-серверов) — все модели
/// 🔗 Требует: ничего

/// Канал. Go: channels.Channel
class Channel {
  final String id;
  final String name;
  final String slug;
  final String description;
  final String? avatarUrl;
  final String? bannerUrl;
  final bool isPublic;
  final String? inviteCode;
  final String? createdBy;
  final int memberCount;
  final DateTime createdAt;

  const Channel({
    required this.id,
    required this.name,
    required this.slug,
    this.description = '',
    this.avatarUrl,
    this.bannerUrl,
    this.isPublic = true,
    this.inviteCode,
    this.createdBy,
    this.memberCount = 0,
    required this.createdAt,
  });

  factory Channel.fromJson(Map<String, dynamic> json) {
    return Channel(
      id: json['id'] as String,
      name: json['name'] as String,
      slug: json['slug'] as String,
      description: json['description'] as String? ?? '',
      avatarUrl: json['avatar_url'] as String?,
      bannerUrl: json['banner_url'] as String?,
      isPublic: json['is_public'] as bool? ?? true,
      inviteCode: json['invite_code'] as String?,
      createdBy: json['created_by'] as String?,
      memberCount: json['member_count'] as int? ?? 0,
      createdAt: json['created_at'] != null
          ? DateTime.parse(json['created_at'] as String)
          : DateTime.now(),
    );
  }
}

/// Канал с ролью текущего пользователя. Go: channels.ChannelPreview
class ChannelPreview extends Channel {
  final String? myRoleName;

  const ChannelPreview({
    required super.id,
    required super.name,
    required super.slug,
    super.description,
    super.avatarUrl,
    super.bannerUrl,
    super.isPublic,
    super.inviteCode,
    super.createdBy,
    super.memberCount,
    required super.createdAt,
    this.myRoleName,
  });

  bool get isAdmin =>
      myRoleName == 'admin' || myRoleName == 'owner';

  factory ChannelPreview.fromJson(Map<String, dynamic> json) {
    return ChannelPreview(
      id: json['id'] as String,
      name: json['name'] as String,
      slug: json['slug'] as String,
      description: json['description'] as String? ?? '',
      avatarUrl: json['avatar_url'] as String?,
      bannerUrl: json['banner_url'] as String?,
      isPublic: json['is_public'] as bool? ?? true,
      inviteCode: json['invite_code'] as String?,
      createdBy: json['created_by'] as String?,
      memberCount: json['member_count'] as int? ?? 0,
      createdAt: json['created_at'] != null
          ? DateTime.parse(json['created_at'] as String)
          : DateTime.now(),
      myRoleName: json['my_role_name'] as String?,
    );
  }
}

/// Комната внутри канала. Go: channels.ChannelRoom
class ChannelRoom {
  final String id;
  final String channelId;
  final String? categoryId;
  final String name;
  final String topic;
  final String type; // "text", "voice", "announcement"
  final int position;
  final bool isNsfw;
  final int slowmodeSec;

  const ChannelRoom({
    required this.id,
    required this.channelId,
    this.categoryId,
    required this.name,
    this.topic = '',
    required this.type,
    this.position = 0,
    this.isNsfw = false,
    this.slowmodeSec = 0,
  });

  bool get isText => type == 'text';
  bool get isVoice => type == 'voice';

  factory ChannelRoom.fromJson(Map<String, dynamic> json) {
    return ChannelRoom(
      id: json['id'] as String,
      channelId: json['channel_id'] as String,
      categoryId: json['category_id'] as String?,
      name: json['name'] as String,
      topic: json['topic'] as String? ?? '',
      type: json['type'] as String,
      position: json['position'] as int? ?? 0,
      isNsfw: json['is_nsfw'] as bool? ?? false,
      slowmodeSec: json['slowmode_sec'] as int? ?? 0,
    );
  }
}

/// Категория комнат. Go: channels.ChannelCategory
class ChannelCategory {
  final String id;
  final String channelId;
  final String name;
  final int position;
  final List<ChannelRoom> rooms;

  const ChannelCategory({
    required this.id,
    required this.channelId,
    required this.name,
    this.position = 0,
    this.rooms = const [],
  });

  factory ChannelCategory.fromJson(Map<String, dynamic> json) {
    return ChannelCategory(
      id: json['id'] as String,
      channelId: json['channel_id'] as String,
      name: json['name'] as String,
      position: json['position'] as int? ?? 0,
      rooms: (json['rooms'] as List<dynamic>?)
              ?.map((e) => ChannelRoom.fromJson(e as Map<String, dynamic>))
              .toList() ??
          [],
    );
  }
}

/// Участник канала. Go: channels.ChannelMember
class ChannelMember {
  final String channelId;
  final String userId;
  final String username;
  final String displayName;
  final String? avatarUrl;
  final String? nickname;
  final String? roleId;
  final String? roleName;
  final String? roleColor;
  final DateTime joinedAt;

  const ChannelMember({
    required this.channelId,
    required this.userId,
    required this.username,
    required this.displayName,
    this.avatarUrl,
    this.nickname,
    this.roleId,
    this.roleName,
    this.roleColor,
    required this.joinedAt,
  });

  String get nameToShow => nickname ?? displayName;

  factory ChannelMember.fromJson(Map<String, dynamic> json) {
    return ChannelMember(
      channelId: json['channel_id'] as String,
      userId: json['user_id'] as String,
      username: json['username'] as String,
      displayName: json['display_name'] as String? ?? '',
      avatarUrl: json['avatar_url'] as String?,
      nickname: json['nickname'] as String?,
      roleId: json['role_id'] as String?,
      roleName: json['role_name'] as String?,
      roleColor: json['role_color'] as String?,
      joinedAt: DateTime.parse(json['joined_at'] as String),
    );
  }
}

/// Сообщение в комнате канала. Go: channels.ChannelMessage
class ChannelMessage {
  final String id;
  final String roomId;
  final String senderId;
  final String senderUsername;
  final String senderDisplayName;
  final String? senderAvatar;
  final String content;
  final String? threadId;
  final bool isPinned;
  final DateTime createdAt;
  final DateTime? editedAt;

  const ChannelMessage({
    required this.id,
    required this.roomId,
    required this.senderId,
    required this.senderUsername,
    required this.senderDisplayName,
    this.senderAvatar,
    required this.content,
    this.threadId,
    this.isPinned = false,
    required this.createdAt,
    this.editedAt,
  });

  bool get isEdited => editedAt != null;

  factory ChannelMessage.fromJson(Map<String, dynamic> json) {
    return ChannelMessage(
      id: json['id'] as String,
      roomId: json['room_id'] as String,
      senderId: json['sender_id'] as String,
      senderUsername: json['sender_username'] as String,
      senderDisplayName: json['sender_display_name'] as String? ?? '',
      senderAvatar: json['sender_avatar'] as String?,
      content: json['content'] as String? ?? '',
      threadId: json['thread_id'] as String?,
      isPinned: json['is_pinned'] as bool? ?? false,
      createdAt: json['created_at'] != null
          ? DateTime.parse(json['created_at'] as String)
          : DateTime.now(),
      editedAt: json['edited_at'] != null
          ? DateTime.parse(json['edited_at'] as String)
          : null,
    );
  }
}
