package auth

import "context"

// Этот файл решает одну задачу: как передать данные
// из JWT-токена (UserID, SessionID) от middleware к handler'у.
//
// В Go для этого используется context — объект, который
// путешествует вместе с HTTP-запросом через все слои.
// Middleware кладёт Claims в context, handler достаёт.
//
// contextKey — приватный тип, чтобы другие пакеты
// не могли случайно перезаписать наше значение в context.

type contextKey string

const claimsKey contextKey = "claims"

// SetClaimsToContext кладёт JWT claims в context.
// Вызывается в auth middleware после успешной проверки токена.
func SetClaimsToContext(ctx context.Context, claims *Claims) context.Context {
	return context.WithValue(ctx, claimsKey, claims)
}

// GetClaimsFromContext достаёт JWT claims из context.
// Вызывается в handler'ах для получения UserID текущего пользователя.
// Возвращает nil если пользователь не авторизован.
func GetClaimsFromContext(ctx context.Context) *Claims {
	claims, _ := ctx.Value(claimsKey).(*Claims)
	return claims
}
