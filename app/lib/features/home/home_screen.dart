/// 📁 Куда: lib/features/home/home_screen.dart (ЗАМЕНИТЬ)
/// 📝 Главный экран с навигацией: чаты, каналы, профиль
/// 🔗 Требует: все feature-экраны, auth_provider, channel_provider

import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../../core/auth/auth_provider.dart';
import '../chats/chat_list_screen.dart';
import '../channels/channel_list_screen.dart';
import '../channels/channel_provider.dart';
import '../search/user_search_screen.dart';

class HomeScreen extends StatefulWidget {
  const HomeScreen({super.key});

  @override
  State<HomeScreen> createState() => _HomeScreenState();
}

class _HomeScreenState extends State<HomeScreen> {
  int _currentIndex = 0;

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: Text(_titles[_currentIndex]),
        actions: [
          if (_currentIndex == 0)
            IconButton(
              icon: const Icon(Icons.person_search),
              tooltip: 'Новый чат',
              onPressed: () => Navigator.push(
                context,
                MaterialPageRoute(builder: (_) => const UserSearchScreen()),
              ),
            ),
          if (_currentIndex == 1)
            IconButton(
              icon: const Icon(Icons.add),
              tooltip: 'Создать канал',
              onPressed: () => _showCreateChannel(context),
            ),
          if (_currentIndex == 2)
            IconButton(
              icon: const Icon(Icons.logout),
              tooltip: 'Выйти',
              onPressed: () => context.read<AuthProvider>().logout(),
            ),
        ],
      ),
      body: IndexedStack(
        index: _currentIndex,
        children: const [
          ChatListScreen(),
          ChannelListScreen(),
          _ProfileTab(),
        ],
      ),
      bottomNavigationBar: NavigationBar(
        selectedIndex: _currentIndex,
        onDestinationSelected: (i) => setState(() => _currentIndex = i),
        destinations: const [
          NavigationDestination(
            icon: Icon(Icons.chat_outlined),
            selectedIcon: Icon(Icons.chat),
            label: 'Чаты',
          ),
          NavigationDestination(
            icon: Icon(Icons.forum_outlined),
            selectedIcon: Icon(Icons.forum),
            label: 'Каналы',
          ),
          NavigationDestination(
            icon: Icon(Icons.person_outline),
            selectedIcon: Icon(Icons.person),
            label: 'Профиль',
          ),
        ],
      ),
    );
  }

  static const _titles = ['Чаты', 'Каналы', 'Профиль'];

  void _showCreateChannel(BuildContext context) {
    final nameController = TextEditingController();
    final descController = TextEditingController();
    bool isPublic = true;

    showDialog(
      context: context,
      builder: (ctx) => StatefulBuilder(
        builder: (ctx, setDialogState) => AlertDialog(
          title: const Text('Новый канал'),
          content: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              TextField(
                controller: nameController,
                decoration: const InputDecoration(labelText: 'Название'),
                autofocus: true,
              ),
              const SizedBox(height: 12),
              TextField(
                controller: descController,
                decoration: const InputDecoration(labelText: 'Описание'),
              ),
              const SizedBox(height: 12),
              SwitchListTile(
                title: const Text('Публичный'),
                value: isPublic,
                onChanged: (v) => setDialogState(() => isPublic = v),
                contentPadding: EdgeInsets.zero,
              ),
            ],
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(ctx),
              child: const Text('Отмена'),
            ),
            FilledButton(
              onPressed: () async {
                final name = nameController.text.trim();
                if (name.isEmpty) return;
                Navigator.pop(ctx);

                final channelProvider = context.read<ChannelProvider>();
                final id = await channelProvider.create(
                  name,
                  descController.text.trim(),
                  isPublic,
                );

                if (mounted) {
                  ScaffoldMessenger.of(context).showSnackBar(
                    SnackBar(
                      content: Text(id != null
                          ? 'Канал "$name" создан!'
                          : 'Не удалось создать канал'),
                    ),
                  );
                }
              },
              child: const Text('Создать'),
            ),
          ],
        ),
      ),
    );
  }
}

class _ProfileTab extends StatelessWidget {
  const _ProfileTab();

  @override
  Widget build(BuildContext context) {
    final auth = context.watch<AuthProvider>();
    final user = auth.user;
    final theme = Theme.of(context);

    return ListView(
      padding: const EdgeInsets.all(24),
      children: [
        Center(
          child: CircleAvatar(
            radius: 48,
            backgroundImage: user?['avatar_url'] != null
                ? NetworkImage(user!['avatar_url'])
                : null,
            child: user?['avatar_url'] == null
                ? Text(
                    (user?['display_name'] ?? 'U')[0].toUpperCase(),
                    style: const TextStyle(fontSize: 36),
                  )
                : null,
          ),
        ),
        const SizedBox(height: 16),
        Center(
          child: Text(
            user?['display_name'] ?? 'User',
            style: theme.textTheme.headlineSmall,
          ),
        ),
        Center(
          child: Text(
            '@${user?['username'] ?? ''}',
            style: TextStyle(color: Colors.grey[600], fontSize: 16),
          ),
        ),
        if (user?['bio'] != null && (user!['bio'] as String).isNotEmpty) ...[
          const SizedBox(height: 12),
          Center(
            child: Text(
              user['bio'],
              style: TextStyle(color: Colors.grey[500]),
              textAlign: TextAlign.center,
            ),
          ),
        ],
        const SizedBox(height: 32),
        const Divider(),
        ListTile(
          leading: const Icon(Icons.edit),
          title: const Text('Редактировать профиль'),
          onTap: () {
            // TODO: экран редактирования
          },
        ),
        ListTile(
          leading: const Icon(Icons.logout, color: Colors.red),
          title: const Text('Выйти', style: TextStyle(color: Colors.red)),
          onTap: () => auth.logout(),
        ),
      ],
    );
  }
}
