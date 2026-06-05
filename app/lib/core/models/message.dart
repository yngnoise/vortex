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
  final List<Attachment> attachments;

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
    this.attachments = const [],
  });

  bool get isEdited => editedAt != null;
  bool get hasAttachments => attachments.isNotEmpty;

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
      attachments: (json['attachments'] as List<dynamic>?)
              ?.map((a) => Attachment.fromJson(a as Map<String, dynamic>))
              .toList() ??
          const [],
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
        'attachments': attachments.map((a) => a.toJson()).toList(),
      };
}

/// Вложение сообщения — зеркало Go messaging.Attachment.
class Attachment {
  final String id;
  final String fileType; // image, video, audio, document, other
  final String fileUrl;
  final int fileSize;
  final String? fileName;
  final String? mimeType;

  const Attachment({
    required this.id,
    required this.fileType,
    required this.fileUrl,
    required this.fileSize,
    this.fileName,
    this.mimeType,
  });

  bool get isImage => fileType == 'image';

  factory Attachment.fromJson(Map<String, dynamic> json) {
    return Attachment(
      id: json['id'] as String? ?? '',
      fileType: json['file_type'] as String? ?? 'other',
      fileUrl: json['file_url'] as String? ?? '',
      fileSize: (json['file_size'] as num?)?.toInt() ?? 0,
      fileName: json['file_name'] as String?,
      mimeType: json['mime_type'] as String?,
    );
  }

  Map<String, dynamic> toJson() => {
        'id': id,
        'file_type': fileType,
        'file_url': fileUrl,
        'file_size': fileSize,
        'file_name': fileName,
        'mime_type': mimeType,
      };
}
