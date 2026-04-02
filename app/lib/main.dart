import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import 'core/api/api_client.dart';
import 'core/auth/auth_provider.dart';
import 'features/auth/login_screen.dart';
import 'features/home/home_screen.dart';

void main() {
  runApp(const VortexApp());
}

class VortexApp extends StatelessWidget {
  const VortexApp({super.key});

  @override
  Widget build(BuildContext context) {
    // Создаём ApiClient один раз — он живёт всё время работы приложения.
    final apiClient = ApiClient();

    return MultiProvider(
      providers: [
        // ApiClient доступен везде через Provider
        Provider<ApiClient>.value(value: apiClient),

        // AuthProvider управляет авторизацией
        ChangeNotifierProvider<AuthProvider>(
          create: (_) => AuthProvider(apiClient)..init(),
        ),
      ],
      child: MaterialApp(
        title: 'Vortex',
        debugShowCheckedModeBanner: false,

        // ── Тема ────────────────────────────
        theme: ThemeData(
          colorScheme: ColorScheme.fromSeed(
            seedColor: const Color(0xFF6C5CE7), // фиолетовый акцент
            brightness: Brightness.light,
          ),
          useMaterial3: true,
          fontFamily: 'Segoe UI', // нативный шрифт Windows
        ),
        darkTheme: ThemeData(
          colorScheme: ColorScheme.fromSeed(
            seedColor: const Color(0xFF6C5CE7),
            brightness: Brightness.dark,
          ),
          useMaterial3: true,
          fontFamily: 'Segoe UI',
        ),
        themeMode: ThemeMode.system, // следуем за системой

        // ── Роутинг ─────────────────────────
        // Если пользователь залогинен — HomeScreen.
        // Если нет — LoginScreen.
        home: const AuthGate(),
      ),
    );
  }
}

/// AuthGate — решает какой экран показать.
/// Слушает AuthProvider и переключает между логином и главным экраном.
class AuthGate extends StatelessWidget {
  const AuthGate({super.key});

  @override
  Widget build(BuildContext context) {
    final auth = context.watch<AuthProvider>();

    // Показываем splash пока загружаются токены
    if (auth.user == null && auth.isLoading) {
      return const Scaffold(
        body: Center(
          child: CircularProgressIndicator(),
        ),
      );
    }

    if (auth.isLoggedIn) {
      return const HomeScreen();
    }

    return const LoginScreen();
  }
}
