/// 📁 Куда: lib/core/models/user.dart
/// 📝 Модель пользователя — точное зеркало Go auth.User
/// 🔗 Требует: ничего

class User {
  final String id;
  final String username;
  final String displayName;
  final String? email;
  final String? phone;
  final String? avatarUrl;
  final String? publicKey;
  final String status;
  final String bio;
  final DateTime lastSeenAt;
  final DateTime createdAt;

  const User({
    required this.id,
    required this.username,
    required this.displayName,
    this.email,
    this.phone,
    this.avatarUrl,
    this.publicKey,
    this.status = 'active',
    this.bio = '',
    required this.lastSeenAt,
    required this.createdAt,
  });

  String get nameToShow =>
      displayName.isNotEmpty ? displayName : username;

  bool get isOnline =>
      DateTime.now().difference(lastSeenAt).inMinutes < 5;

  factory User.fromJson(Map<String, dynamic> json) {
    return User(
      id: json['id'] as String,
      username: json['username'] as String,
      displayName: json['display_name'] as String? ?? '',
      email: json['email'] as String?,
      phone: json['phone'] as String?,
      avatarUrl: json['avatar_url'] as String?,
      publicKey: json['public_key'] as String?,
      status: json['status'] as String? ?? 'active',
      bio: json['bio'] as String? ?? '',
      lastSeenAt: json['last_seen_at'] != null
          ? DateTime.parse(json['last_seen_at'] as String)
          : DateTime.now(),
      createdAt: json['created_at'] != null
          ? DateTime.parse(json['created_at'] as String)
          : DateTime.now(),
    );
  }

  Map<String, dynamic> toJson() => {
        'id': id,
        'username': username,
        'display_name': displayName,
        'email': email,
        'phone': phone,
        'avatar_url': avatarUrl,
        'public_key': publicKey,
        'status': status,
        'bio': bio,
        'last_seen_at': lastSeenAt.toIso8601String(),
        'created_at': createdAt.toIso8601String(),
      };

  @override
  bool operator ==(Object other) =>
      identical(this, other) || other is User && other.id == id;

  @override
  int get hashCode => id.hashCode;
}
