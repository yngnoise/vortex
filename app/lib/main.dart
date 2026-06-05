/// 📁 Куда: lib/main.dart (ЗАМЕНИТЬ)
/// 📝 Точка входа — добавлен RealtimeService
/// 🔗 Изменения: +RealtimeService, автоподключение после авторизации

import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import 'core/api/api_client.dart';
import 'core/auth/auth_provider.dart';
import 'core/realtime/realtime_service.dart';
import 'features/auth/login_screen.dart';
import 'features/chats/chat_list_provider.dart';
import 'features/channels/channel_provider.dart';
import 'features/home/home_screen.dart';

void main() {
  runApp(const VortexApp());
}

class VortexApp extends StatelessWidget {
  const VortexApp({super.key});

  @override
  Widget build(BuildContext context) {
    final apiClient = ApiClient();
    final realtimeService = RealtimeService(apiClient);

    return MultiProvider(
      providers: [
        Provider<ApiClient>.value(value: apiClient),
        Provider<RealtimeService>.value(value: realtimeService),

        ChangeNotifierProvider<AuthProvider>(
          create: (_) => AuthProvider(apiClient)..init(),
        ),
        ChangeNotifierProvider<ChatListProvider>(
          create: (_) => ChatListProvider(apiClient),
        ),
        ChangeNotifierProvider<ChannelProvider>(
          create: (_) => ChannelProvider(apiClient),
        ),
      ],
      child: MaterialApp(
        title: 'Vortex',
        debugShowCheckedModeBanner: false,
        theme: ThemeData(
          colorScheme: ColorScheme.fromSeed(
            seedColor: const Color(0xFF6C5CE7),
            brightness: Brightness.light,
          ),
          useMaterial3: true,
          fontFamily: 'Segoe UI',
        ),
        darkTheme: ThemeData(
          colorScheme: ColorScheme.fromSeed(
            seedColor: const Color(0xFF6C5CE7),
            brightness: Brightness.dark,
          ),
          useMaterial3: true,
          fontFamily: 'Segoe UI',
        ),
        themeMode: ThemeMode.system,
        home: const AuthGate(),
      ),
    );
  }
}

class AuthGate extends StatefulWidget {
  const AuthGate({super.key});

  @override
  State<AuthGate> createState() => _AuthGateState();
}

class _AuthGateState extends State<AuthGate> {
  bool _realtimeConnected = false;

  @override
  Widget build(BuildContext context) {
    final auth = context.watch<AuthProvider>();

    if (auth.user == null && auth.isLoading) {
      return const Scaffold(
        body: Center(child: CircularProgressIndicator()),
      );
    }

    if (auth.isLoggedIn) {
      // Подключаем realtime один раз при авторизации
      if (!_realtimeConnected) {
        _realtimeConnected = true;
        final rt = context.read<RealtimeService>();
        rt.connect().then((_) {
          final userId = auth.user?['id'] as String?;
          if (userId != null) {
            rt.subscribeUser(userId);
          }
        });
      }
      return const HomeScreen();
    }

    // Если вышли — отключаем realtime
    if (_realtimeConnected) {
      _realtimeConnected = false;
      context.read<RealtimeService>().disconnect();
    }

    return const LoginScreen();
  }
}
