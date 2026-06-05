/// 📁 Куда: lib/core/presence.dart
/// 📝 Утилиты присутствия: «в сети», если last_seen свежее 60 секунд.

bool isOnline(DateTime? lastSeen) =>
    lastSeen != null && DateTime.now().difference(lastSeen).inSeconds < 60;

/// Человеко-читаемая подпись присутствия для шапки чата.
String presenceLabel(DateTime? lastSeen) {
  if (lastSeen == null) return '';
  final diff = DateTime.now().difference(lastSeen);
  if (diff.inSeconds < 60) return 'в сети';
  if (diff.inMinutes < 60) return 'был(а) в сети ${diff.inMinutes} мин назад';
  if (diff.inHours < 24) return 'был(а) в сети ${diff.inHours} ч назад';
  if (diff.inDays == 1) return 'был(а) в сети вчера';
  return 'был(а) в сети ${diff.inDays} дн назад';
}
