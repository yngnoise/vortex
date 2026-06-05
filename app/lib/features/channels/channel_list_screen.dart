/// 📁 Куда: lib/features/channels/channel_list_screen.dart (ЗАМЕНИТЬ)
/// 📝 Список каналов: мои + discover. Тап → ChannelDetailScreen
/// 🔗 Требует: channel_provider.dart, channel_detail_screen.dart

import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../../core/models/channel.dart';
import 'channel_provider.dart';
import 'channel_detail_screen.dart';

class ChannelListScreen extends StatefulWidget {
  const ChannelListScreen({super.key});

  @override
  State<ChannelListScreen> createState() => _ChannelListScreenState();
}

class _ChannelListScreenState extends State<ChannelListScreen>
    with SingleTickerProviderStateMixin {
  late final TabController _tabController;

  @override
  void initState() {
    super.initState();
    _tabController = TabController(length: 2, vsync: this);
    final provider = context.read<ChannelProvider>();
    Future.microtask(() {
      provider.loadMyChannels();
      provider.loadDiscover();
    });
  }

  @override
  void dispose() {
    _tabController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Column(
      children: [
        TabBar(
          controller: _tabController,
          tabs: const [
            Tab(text: 'Мои каналы'),
            Tab(text: 'Обзор'),
          ],
        ),
        Expanded(
          child: TabBarView(
            controller: _tabController,
            children: const [
              _MyChannelsTab(),
              _DiscoverTab(),
            ],
          ),
        ),
      ],
    );
  }
}

class _MyChannelsTab extends StatelessWidget {
  const _MyChannelsTab();

  @override
  Widget build(BuildContext context) {
    final provider = context.watch<ChannelProvider>();

    if (provider.isLoading && provider.myChannels.isEmpty) {
      return const Center(child: CircularProgressIndicator());
    }

    if (provider.myChannels.isEmpty) {
      return Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.forum_outlined, size: 64, color: Colors.grey[400]),
            const SizedBox(height: 16),
            Text(
              'Вы пока не в каналах',
              style: TextStyle(fontSize: 18, color: Colors.grey[600]),
            ),
            const SizedBox(height: 8),
            Text(
              'Найдите интересный во вкладке «Обзор»',
              style: TextStyle(color: Colors.grey[500]),
            ),
          ],
        ),
      );
    }

    return RefreshIndicator(
      onRefresh: () => provider.refresh(),
      child: ListView.builder(
        itemCount: provider.myChannels.length,
        itemBuilder: (context, index) {
          final ch = provider.myChannels[index];
          return _ChannelTile(
            name: ch.name,
            description: ch.description,
            memberCount: ch.memberCount,
            avatarUrl: ch.avatarUrl,
            trailing: ch.isAdmin
                ? Chip(
                    label: Text(ch.myRoleName ?? 'admin',
                        style: const TextStyle(fontSize: 11)),
                    padding: EdgeInsets.zero,
                    visualDensity: VisualDensity.compact,
                  )
                : null,
            onTap: () {
              Navigator.push(
                context,
                MaterialPageRoute(
                  builder: (_) => ChannelDetailScreen(
                    channelId: ch.id,
                    channelName: ch.name,
                  ),
                ),
              );
            },
          );
        },
      ),
    );
  }
}

class _DiscoverTab extends StatelessWidget {
  const _DiscoverTab();

  @override
  Widget build(BuildContext context) {
    final provider = context.watch<ChannelProvider>();

    if (provider.discover.isEmpty) {
      return Center(
        child: Text(
          'Нет публичных каналов',
          style: TextStyle(color: Colors.grey[500]),
        ),
      );
    }

    final myIds = provider.myChannels.map((c) => c.id).toSet();

    return ListView.builder(
      itemCount: provider.discover.length,
      itemBuilder: (context, index) {
        final ch = provider.discover[index];
        final joined = myIds.contains(ch.id);

        return _ChannelTile(
          name: ch.name,
          description: ch.description,
          memberCount: ch.memberCount,
          avatarUrl: ch.avatarUrl,
          trailing: joined
              ? const Chip(
                  label: Text('Вы участник', style: TextStyle(fontSize: 11)),
                  padding: EdgeInsets.zero,
                  visualDensity: VisualDensity.compact,
                )
              : FilledButton.tonal(
                  onPressed: () => provider.join(ch.id),
                  child: const Text('Вступить'),
                ),
          onTap: joined
              ? () {
                  Navigator.push(
                    context,
                    MaterialPageRoute(
                      builder: (_) => ChannelDetailScreen(
                        channelId: ch.id,
                        channelName: ch.name,
                      ),
                    ),
                  );
                }
              : () {},
        );
      },
    );
  }
}

class _ChannelTile extends StatelessWidget {
  final String name;
  final String description;
  final int memberCount;
  final String? avatarUrl;
  final Widget? trailing;
  final VoidCallback onTap;

  const _ChannelTile({
    required this.name,
    required this.description,
    required this.memberCount,
    this.avatarUrl,
    this.trailing,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return ListTile(
      leading: CircleAvatar(
        radius: 24,
        backgroundImage:
            avatarUrl != null ? NetworkImage(avatarUrl!) : null,
        child: avatarUrl == null
            ? Text(name.isNotEmpty ? name[0].toUpperCase() : '#',
                style: const TextStyle(fontSize: 18))
            : null,
      ),
      title: Text(name, maxLines: 1, overflow: TextOverflow.ellipsis),
      subtitle: Text(
        description.isNotEmpty
            ? description
            : '$memberCount участник${_plural(memberCount)}',
        maxLines: 1,
        overflow: TextOverflow.ellipsis,
        style: TextStyle(color: Colors.grey[600]),
      ),
      trailing: trailing,
      onTap: onTap,
    );
  }

  String _plural(int n) {
    if (n % 10 == 1 && n % 100 != 11) return '';
    if (n % 10 >= 2 && n % 10 <= 4 && (n % 100 < 10 || n % 100 >= 20)) {
      return 'а';
    }
    return 'ов';
  }
}
