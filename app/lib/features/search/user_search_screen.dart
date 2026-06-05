/// 📁 Куда: lib/features/search/user_search_screen.dart
/// 📝 Поиск пользователей → начать чат
/// 🔗 Требует: api_client.dart, chat_list_provider.dart, chat_screen.dart

import 'dart:async';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../../core/api/api_client.dart';
import '../../core/models/user.dart';
import '../chats/chat_list_provider.dart';
import '../chats/chat_screen.dart';

class UserSearchScreen extends StatefulWidget {
  const UserSearchScreen({super.key});

  @override
  State<UserSearchScreen> createState() => _UserSearchScreenState();
}

class _UserSearchScreenState extends State<UserSearchScreen> {
  final _searchController = TextEditingController();
  List<User> _results = [];
  bool _isLoading = false;
  String? _error;
  Timer? _debounce;

  @override
  void dispose() {
    _searchController.dispose();
    _debounce?.cancel();
    super.dispose();
  }

  void _onSearchChanged(String query) {
    _debounce?.cancel();
    if (query.length < 2) {
      setState(() {
        _results = [];
        _error = null;
      });
      return;
    }
    _debounce = Timer(const Duration(milliseconds: 400), () {
      _search(query);
    });
  }

  Future<void> _search(String query) async {
    setState(() {
      _isLoading = true;
      _error = null;
    });

    try {
      final api = context.read<ApiClient>();
      final data = await api.searchUsers(query);
      setState(() {
        _results = data
            .map((json) => User.fromJson(json as Map<String, dynamic>))
            .toList();
        _isLoading = false;
      });
    } on ApiException catch (e) {
      setState(() {
        _error = e.message;
        _isLoading = false;
      });
    } catch (_) {
      setState(() {
        _error = 'Ошибка поиска';
        _isLoading = false;
      });
    }
  }

  Future<void> _startChat(User user) async {
    final chatList = context.read<ChatListProvider>();
    final convId = await chatList.createDirect(user.id);

    if (convId != null && mounted) {
      Navigator.pushReplacement(
        context,
        MaterialPageRoute(
          builder: (_) => ChatScreen(
            conversationId: convId,
            title: user.nameToShow,
            avatarUrl: user.avatarUrl,
          ),
        ),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: TextField(
          controller: _searchController,
          autofocus: true,
          decoration: const InputDecoration(
            hintText: 'Поиск по имени...',
            border: InputBorder.none,
          ),
          onChanged: _onSearchChanged,
        ),
        actions: [
          if (_searchController.text.isNotEmpty)
            IconButton(
              icon: const Icon(Icons.clear),
              onPressed: () {
                _searchController.clear();
                setState(() {
                  _results = [];
                  _error = null;
                });
              },
            ),
        ],
      ),
      body: _buildBody(),
    );
  }

  Widget _buildBody() {
    if (_isLoading) {
      return const Center(child: CircularProgressIndicator());
    }

    if (_error != null) {
      return Center(
          child: Text(_error!, style: TextStyle(color: Colors.grey[600])));
    }

    if (_searchController.text.length < 2) {
      return Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.search, size: 64, color: Colors.grey[400]),
            const SizedBox(height: 16),
            Text(
              'Введите минимум 2 символа',
              style: TextStyle(color: Colors.grey[500]),
            ),
          ],
        ),
      );
    }

    if (_results.isEmpty) {
      return Center(
        child: Text(
          'Никого не нашлось',
          style: TextStyle(color: Colors.grey[600]),
        ),
      );
    }

    return ListView.builder(
      itemCount: _results.length,
      itemBuilder: (context, index) {
        final user = _results[index];
        return ListTile(
          leading: CircleAvatar(
            backgroundImage:
                user.avatarUrl != null ? NetworkImage(user.avatarUrl!) : null,
            child: user.avatarUrl == null
                ? Text(user.nameToShow[0].toUpperCase())
                : null,
          ),
          title: Text(user.nameToShow),
          subtitle: Text('@${user.username}'),
          trailing: const Icon(Icons.chat_bubble_outline, size: 20),
          onTap: () => _startChat(user),
        );
      },
    );
  }
}
