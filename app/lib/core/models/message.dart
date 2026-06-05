/// 📁 Куда: lib/core/models/message.dart
/// 📝 Сообщение в чате — зеркало Go messaging.Message
/// 🔗 Требует: ничего

class Message {
  final String id;
  final String conversationId;
  final String senderId;
  final String senderUsername;
  final String senderDisplayName;
  final String content;
  final String contentType; // "text", "image", "file" и т.д.
  final String? replyToId;
  final DateTime createdAt;
  final DateTime? editedAt;

  const Message({
    required this.id,
    required this.conversationId,
    required this.senderId,
    required this.senderUsername,
    required this.senderDisplayName,
    required this.content,
    this.contentType = 'text',
    this.replyToId,
    required this.createdAt,
    this.editedAt,
  });

  bool get isEdited => editedAt != null;

  factory Message.fromJson(Map<String, dynamic> json) {
    return Message(
      id: json['id'] as String,
      conversationId: json['conversation_id'] as String,
      senderId: json['sender_id'] as String,
      senderUsername: json['sender_username'] as String,
      senderDisplayName: json['sender_display_name'] as String? ?? '',
      content: json['content'] as String? ?? '',
      contentType: json['content_type'] as String? ?? 'text',
      replyToId: json['reply_to_id'] as String?,
      createdAt: json['created_at'] != null
          ? DateTime.parse(json['created_at'] as String)
          : DateTime.now(),
      editedAt: json['edited_at'] != null
          ? DateTime.parse(json['edited_at'] as String)
          : null,
    );
  }

  Map<String, dynamic> toJson() => {
        'id': id,
        'conversation_id': conversationId,
        'sender_id': senderId,
        'sender_username': senderUsername,
        'sender_display_name': senderDisplayName,
        'content': content,
        'content_type': contentType,
        'reply_to_id': replyToId,
        'created_at': createdAt.toIso8601String(),
        'edited_at': editedAt?.toIso8601String(),
      };
}
