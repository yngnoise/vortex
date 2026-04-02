import 'package:flutter/material.dart';
import '../api/api_client.dart';

/// AuthProvider — управляет состоянием авторизации.
///
/// Хранит текущего пользователя, обрабатывает логин/регистрацию/logout.
/// Виджеты подписываются через Provider и автоматически
/// перестраиваются при изменении состояния.
class AuthProvider extends ChangeNotifier {
  final ApiClient api;

  AuthProvider(this.api);

  Map<String, dynamic>? _user;
  bool _isLoading = false;
  String? _error;

  /// Текущий пользователь (null если не залогинен).
  Map<String, dynamic>? get user => _user;

  /// Идёт ли загрузка (логин, регистрация и т.д.).
  bool get isLoading => _isLoading;

  /// Последняя ошибка (null если всё ок).
  String? get error => _error;

  /// Залогинен ли пользователь.
  bool get isLoggedIn => _user != null;

  /// Инициализация при старте приложения.
  /// Загружает токены из хранилища и пробует получить профиль.
  Future<void> init() async {
    await api.loadTokens();
    if (api.isLoggedIn) {
      try {
        _user = await api.getMe();
      } catch (_) {
        // Токен невалидный — пробуем refresh
        final refreshed = await api.refreshTokens();
        if (refreshed) {
          try {
            _user = await api.getMe();
          } catch (_) {
            await api.clearTokens();
          }
        }
      }
      notifyListeners();
    }
  }

  /// Регистрация.
  Future<bool> register({
    required String username,
    required String displayName,
    required String email,
    required String password,
  }) async {
    _isLoading = true;
    _error = null;
    notifyListeners();

    try {
      final response = await api.register(
        username: username,
        displayName: displayName,
        email: email,
        password: password,
      );
      _user = response['user'];
      _isLoading = false;
      notifyListeners();
      return true;
    } on ApiException catch (e) {
      _error = _translateError(e.code);
      _isLoading = false;
      notifyListeners();
      return false;
    } catch (e) {
      _error = 'Нет соединения с сервером';
      _isLoading = false;
      notifyListeners();
      return false;
    }
  }

  /// Логин.
  Future<bool> login({
    required String username,
    required String password,
  }) async {
    _isLoading = true;
    _error = null;
    notifyListeners();

    try {
      final response = await api.login(
        username: username,
        password: password,
      );
      _user = response['user'];
      _isLoading = false;
      notifyListeners();
      return true;
    } on ApiException catch (e) {
      _error = _translateError(e.code);
      _isLoading = false;
      notifyListeners();
      return false;
    } catch (e) {
      _error = 'Нет соединения с сервером';
      _isLoading = false;
      notifyListeners();
      return false;
    }
  }

  /// Logout.
  Future<void> logout() async {
    await api.logout();
    _user = null;
    notifyListeners();
  }

  /// Очистить ошибку.
  void clearError() {
    _error = null;
    notifyListeners();
  }

  /// Перевод кодов ошибок в человекочитаемый текст.
  String _translateError(String code) {
    switch (code) {
      case 'INVALID_CREDENTIALS':
        return 'Неверный логин или пароль';
      case 'USER_EXISTS':
        return 'Это имя пользователя уже занято';
      case 'EMAIL_EXISTS':
        return 'Этот email уже зарегистрирован';
      case 'INVALID_USERNAME':
        return 'Имя пользователя: 3-32 символа';
      case 'WEAK_PASSWORD':
        return 'Пароль минимум 8 символов';
      case 'INVALID_EMAIL':
        return 'Введите корректный email';
      case 'SESSION_EXPIRED':
        return 'Сессия истекла, войдите заново';
      default:
        return 'Произошла ошибка, попробуйте позже';
    }
  }
}
