package processor

import (
	"context"

	"github.com/EduGoGroup/edugo-worker/internal/client"
)

// NotificationDispatcher delega la entrega de notificaciones (in-app + push) al
// Notification Gateway en edugo-api-platform (plan 020 D13). Lo implementa
// client.NotificationDispatchClient; los processors dependen de esta interfaz
// para facilitar el testing con dobles.
type NotificationDispatcher interface {
	Dispatch(ctx context.Context, req client.DispatchRequest) error
}

// caller identifica al worker como origen en el contrato de dispatch.
const dispatchCaller = "edugo-worker"
