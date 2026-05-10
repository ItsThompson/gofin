package email

import "context"

// Sender abstracts email delivery for testability.
type Sender interface {
	SendExportEmail(ctx context.Context, toEmail string, zipBytes []byte) error
}
