// Package notify sends push notifications through Firebase Cloud Messaging.
// When Firebase is not configured it logs instead of failing.
package notify

import (
	"context"
	"log/slog"
	"sync"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"

	"github.com/granborg/wcs_workshops/apps/api/internal/store"
)

// Notifier delivers notifications to every device registered to a user.
type Notifier struct {
	st     *store.Store
	log    *slog.Logger
	once   sync.Once
	client *messaging.Client
	err    error
}

// New creates a Notifier. Firebase initialises lazily on first use.
func New(st *store.Store, log *slog.Logger) *Notifier {
	return &Notifier{st: st, log: log}
}

func (n *Notifier) init(ctx context.Context) {
	app, err := firebase.NewApp(ctx, nil)
	if err != nil {
		n.err = err
		return
	}
	n.client, n.err = app.Messaging(ctx)
}

// NotifyUser sends to all of the user's devices. Errors are logged, never returned.
func (n *Notifier) NotifyUser(ctx context.Context, userID, title, body string, data map[string]string) {
	tokens, err := n.st.ListDeviceTokens(ctx, userID)
	if err != nil {
		n.log.Error("list device tokens", "err", err)
		return
	}
	if len(tokens) == 0 {
		return
	}
	n.once.Do(func() { n.init(ctx) })
	if n.err != nil {
		n.log.Info("push disabled (Firebase not configured)", "user", userID, "title", title, "reason", n.err.Error())
		return
	}

	res, err := n.client.SendEachForMulticast(ctx, &messaging.MulticastMessage{
		Tokens:       tokens,
		Notification: &messaging.Notification{Title: title, Body: body},
		Data:         data,
	})
	if err != nil {
		n.log.Error("push send failed", "err", err)
		return
	}
	stale := make([]string, 0)
	for i, r := range res.Responses {
		if !r.Success && r.Error != nil && messaging.IsRegistrationTokenNotRegistered(r.Error) {
			stale = append(stale, tokens[i])
		}
	}
	if len(stale) > 0 {
		if err := n.st.DeleteDevicesByTokens(ctx, stale); err != nil {
			n.log.Error("delete stale tokens", "err", err)
		}
	}
}
