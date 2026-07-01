/// 📁 Куда: lib/features/chats/group_info_screen.dart
/// 📝 Управление группой: название, участники, добавить/исключить, выйти
/// 🔗 Требует: api_client.dart, auth_provider.dart, models/conversation.dart

import 'dart:async';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../../core/api/api_client.dart';
import '../../core/auth/auth_provider.dart';
import '../../core/models/conversation.dart';

class GroupInfoScreen extends StatefulWidget {
  final String conversationId;
  final String title;

  const GroupInfoScreen({
    super.key,
    required this.conversationId,
    required this.title,
  });

  @override
  State<GroupInfoScreen> createState() => _GroupInfoScreenState();
}

class _GroupInfoScreenState extends State<GroupInfoScreen> {
  late String _title;
  List<ConversationMember> _members = [];
  bool _loading = true;
  String _myRole = 'member';
  String _myId = '';

  bool get _isManager => _myRole == 'owner' || _myRole == 'admin';

  @override
  void initState() {
    super.initState();
    _title = widget.title;
    _myId = context.read<AuthProvider>().user?['id'] as String? ?? '';
    _load();
  }

  Future<void> _load() async {
    try {
      final data =
          await context.read<ApiClient>().getConversationMembers(widget.conversationId);
      final members = data
          .map((j) => ConversationMember.fromJson(j as Map<String, dynamic>))
          .toList();
      var myRole = 'member';
      for (final m in members) {
        if (m.userId == _myId) myRole = m.role;
      }
      if (!mounted) return;
      setState(() {
        _members = members;
        _myRole = myRole;
        _loading = false;
      });
    } catch (_) {
      if (mounted) setState(() => _loading = false);
    }
  }

  Future<void> _rename() async {
    final api = context.read<ApiClient>();
    final controller = TextEditingController(text: _title);
    final newTitle = await showDialog<String>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Название группы'),
        content: TextField(controller: controller, autofocus: true, maxLength: 128),
        actions: [
          TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('Отмена')),
          FilledButton(
            onPressed: () => Navigator.pop(ctx, controller.text.trim()),
            child: const Text('Сохранить'),
          ),
        ],
      ),
    );
    if (newTitle == null || newTitle.isEmpty || newTitle == _title) return;
    try {
      await api.renameGroup(widget.conversationId, newTitle);
      if (mounted) setState(() => _title = newTitle);
    } catch (_) {
      _snack('Не удалось переименовать');
    }
  }

  Future<void> _remove(ConversationMember m) async {
    final api = context.read<ApiClient>();
    if (!await _confirm('Исключить ${m.displayName}?')) return;
    try {
      await api.removeGroupMember(widget.conversationId, m.userId);
      await _load();
    } catch (_) {
      _snack('Не удалось исключить');
    }
  }

  Future<void> _leave() async {
    final api = context.read<ApiClient>();
    final nav = Navigator.of(context);
    if (!await _confirm('Покинуть группу?')) return;
    try {
      await api.leaveConversation(widget.conversationId);
      if (mounted) nav.pop('left');
    } catch (_) {
      _snack('Не удалось выйти');
    }
  }

  Future<void> _addMembers() async {
    final existing = _members.map((m) => m.userId).toSet();
    await showModalBottomSheet(
      context: context,
      isScrollControlled: true,
      builder: (_) => _AddMemberSheet(
        conversationId: widget.conversationId,
        excludeIds: existing,
      ),
    );
    await _load();
  }

  void _snack(String msg) {
    if (mounted) {
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(msg)));
    }
  }

  Future<bool> _confirm(String msg) async {
    return await showDialog<bool>(
          context: context,
          builder: (ctx) => AlertDialog(
            content: Text(msg),
            actions: [
              TextButton(
                  onPressed: () => Navigator.pop(ctx, false),
                  child: const Text('Отмена')),
              FilledButton(
                  onPressed: () => Navigator.pop(ctx, true),
                  child: const Text('Да')),
            ],
          ),
        ) ??
        false;
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Группа')),
      body: _loading
          ? const Center(child: CircularProgressIndicator())
          : ListView(
              children: [
                ListTile(
                  leading: CircleAvatar(
                    radius: 24,
                    child: Text(_title.isNotEmpty ? _title[0].toUpperCase() : '?'),
                  ),
                  title: Text(
                    _title,
                    style: const TextStyle(fontWeight: FontWeight.w600, fontSize: 18),
                  ),
                  subtitle: Text('${_members.length} участников'),
                  trailing: _isManager
                      ? IconButton(icon: const Icon(Icons.edit), onPressed: _rename)
                      : null,
                ),
                const Divider(),
                if (_isManager)
                  ListTile(
                    leading: const Icon(Icons.person_add),
                    title: const Text('Добавить участников'),
                    onTap: _addMembers,
                  ),
                ..._members.map(_memberTile),
                const Divider(),
                ListTile(
                  leading: Icon(Icons.exit_to_app, color: Colors.red[400]),
                  title: Text('Покинуть группу',
                      style: TextStyle(color: Colors.red[400])),
                  onTap: _leave,
                ),
              ],
            ),
    );
  }

  Widget _memberTile(ConversationMember m) {
    return ListTile(
      leading: CircleAvatar(
        backgroundImage: m.avatarUrl != null ? NetworkImage(m.avatarUrl!) : null,
        child: m.avatarUrl == null
            ? Text(m.displayName.isNotEmpty ? m.displayName[0].toUpperCase() : '?')
            : null,
      ),
      title: Text(m.displayName),
      subtitle: Text('@${m.username}'),
      trailing: _memberTrailing(m),
    );
  }

  Widget? _memberTrailing(ConversationMember m) {
    if (m.role == 'owner') {
      return const Chip(
          label: Text('владелец'), visualDensity: VisualDensity.compact);
    }
    if (m.role == 'admin') {
      return const Chip(label: Text('админ'), visualDensity: VisualDensity.compact);
    }
    if (_isManager && m.userId != _myId) {
      return IconButton(
        icon: Icon(Icons.person_remove, color: Colors.red[300]),
        onPressed: () => _remove(m),
      );
    }
    return null;
  }
}

// ── Лист добавления участников ───────────────────────

class _AddMemberSheet extends StatefulWidget {
  final String conversationId;
  final Set<String> excludeIds;

  const _AddMemberSheet({required this.conversationId, required this.excludeIds});

  @override
  State<_AddMemberSheet> createState() => _AddMemberSheetState();
}

class _AddMemberSheetState extends State<_AddMemberSheet> {
  final _controller = TextEditingController();
  List<dynamic> _results = [];
  bool _searching = false;
  Timer? _debounce;

  @override
  void dispose() {
    _debounce?.cancel();
    _controller.dispose();
    super.dispose();
  }

  void _onChanged(String q) {
    _debounce?.cancel();
    _debounce = Timer(const Duration(milliseconds: 350), () => _search(q.trim()));
  }

  Future<void> _search(String q) async {
    if (q.length < 2) {
      setState(() => _results = []);
      return;
    }
    setState(() => _searching = true);
    try {
      final data = await context.read<ApiClient>().searchUsers(q);
      if (!mounted) return;
      setState(() {
        _results = data
            .where((u) => !widget.excludeIds.contains((u as Map)['id']))
            .toList();
        _searching = false;
      });
    } catch (_) {
      if (mounted) setState(() => _searching = false);
    }
  }

  Future<void> _add(Map<String, dynamic> u) async {
    final api = context.read<ApiClient>();
    final nav = Navigator.of(context);
    final messenger = ScaffoldMessenger.of(context);
    try {
      await api.addGroupMembers(widget.conversationId, [u['id'] as String]);
      nav.pop();
    } catch (_) {
      messenger.showSnackBar(const SnackBar(content: Text('Не удалось добавить')));
    }
  }

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: EdgeInsets.only(bottom: MediaQuery.of(context).viewInsets.bottom),
      child: DraggableScrollableSheet(
        expand: false,
        initialChildSize: 0.6,
        builder: (_, scroll) => Column(
          children: [
            Padding(
              padding: const EdgeInsets.all(12),
              child: TextField(
                controller: _controller,
                autofocus: true,
                onChanged: _onChanged,
                decoration: const InputDecoration(
                  hintText: 'Поиск пользователей…',
                  prefixIcon: Icon(Icons.search),
                ),
              ),
            ),
            if (_searching) const LinearProgressIndicator(),
            Expanded(
              child: ListView.builder(
                controller: scroll,
                itemCount: _results.length,
                itemBuilder: (_, i) {
                  final u = _results[i] as Map<String, dynamic>;
                  final name = (u['display_name'] ?? u['username'] ?? '?') as String;
                  return ListTile(
                    leading: CircleAvatar(
                      child: Text(name.isNotEmpty ? name[0].toUpperCase() : '?'),
                    ),
                    title: Text(name),
                    subtitle: Text('@${u['username'] ?? ''}'),
                    trailing: const Icon(Icons.add),
                    onTap: () => _add(u),
                  );
                },
              ),
            ),
          ],
        ),
      ),
    );
  }
}
