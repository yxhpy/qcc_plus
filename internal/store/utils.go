package store

import "context"

func withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, defaultTimeout)
}

func normalizeAccount(accountID string) string {
	if accountID == "" {
		return DefaultAccountID
	}
	return accountID
}

func nullOrString(v string) interface{} {
	if v == "" {
		return nil
	}
	return v
}
