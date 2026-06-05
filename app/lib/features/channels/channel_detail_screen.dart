/// 📁 Куда: lib/features/channels/channel_detail_screen.dart
/// 📝 Экран канала — категории + комнаты, тап → чат в комнате
/// 🔗 Требует: api_client.dart, models/channel.dart, room_chat_screen.dart

import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../../core/api/api_client.dart';
import '../../core/models/channel.dart';
import 'room_chat_screen.dart';

class ChannelDetailScreen extends StatefulWidget {
  final String channelId;
  final String channelName;

  const ChannelDetailScreen({
    super.key,
    required this.channelId,
    required this.channelName,
  });

  @override
  State<ChannelDetailScreen> createState() => _ChannelDetailScreenState();
}

class _ChannelDetailScreenState extends State<ChannelDetailScreen> {
  List<ChannelCategory> _categories = [];
  bool _isLoading = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _loadStructure();
  }

  Future<void> _loadStructure() async {
    setState(() {
      _isLoading = true;
      _error = null;
    });

    try {
      final api = context.read<ApiClient>();
      final data = await api.getChannelStructure(widget.channelId);
      setState(() {
        _categories = data
            .map((json) =>
                ChannelCategory.fromJson(json as Map<String, dynamic>))
            .toList();
        _isLoading = false;
      });
    } catch (e) {
      setState(() {
        _error = 'Не удалось загрузить структуру';
        _isLoading = false;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Scaffold(
      appBar: AppBar(
        title: Text(widget.channelName),
      ),
      body: _isLoading
          ? const Center(child: CircularProgressIndicator())
          : _error != null
              ? Center(
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Text(_error!, style: TextStyle(color: Colors.grey[600])),
                      const SizedBox(height: 16),
                      FilledButton.tonal(
                        onPressed: _loadStructure,
                        child: const Text('Повторить'),
                      ),
                    ],
                  ),
                )
              : _categories.isEmpty
                  ? Center(
                      child: Text(
                        'Нет комнат',
                        style: TextStyle(color: Colors.grey[500]),
                      ),
                    )
                  : ListView.builder(
                      padding: const EdgeInsets.symmetric(vertical: 8),
                      itemCount: _categories.length,
                      itemBuilder: (context, index) {
                        final cat = _categories[index];
                        return _CategorySection(
                          category: cat,
                          onRoomTap: (room) => _openRoom(room),
                        );
                      },
                    ),
    );
  }

  void _openRoom(ChannelRoom room) {
    Navigator.push(
      context,
      MaterialPageRoute(
        builder: (_) => RoomChatScreen(
          roomId: room.id,
          roomName: room.name,
          roomType: room.type,
          channelName: widget.channelName,
        ),
      ),
    );
  }
}

class _CategorySection extends StatelessWidget {
  final ChannelCategory category;
  final void Function(ChannelRoom room) onRoomTap;

  const _CategorySection({
    required this.category,
    required this.onRoomTap,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 16, 16, 4),
          child: Text(
            category.name.toUpperCase(),
            style: TextStyle(
              fontSize: 12,
              fontWeight: FontWeight.w700,
              color: Colors.grey[500],
              letterSpacing: 0.8,
            ),
          ),
        ),
        ...category.rooms.map((room) => ListTile(
              leading: Icon(
                room.isText
                    ? Icons.tag
                    : room.isVoice
                        ? Icons.volume_up
                        : Icons.campaign,
                size: 20,
                color: Colors.grey[600],
              ),
              title: Text(
                room.name,
                style: const TextStyle(fontSize: 15),
              ),
              subtitle: room.topic.isNotEmpty
                  ? Text(
                      room.topic,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: TextStyle(fontSize: 12, color: Colors.grey[500]),
                    )
                  : null,
              dense: true,
              visualDensity: VisualDensity.compact,
              onTap: room.isText ? () => onRoomTap(room) : null,
              enabled: room.isText,
            )),
      ],
    );
  }
}
