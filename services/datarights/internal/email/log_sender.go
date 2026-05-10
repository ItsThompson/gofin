package email

import (
	"context"
	"log/slog"
)

// LogSender implements Sender by logging email content instead of sending.
// Used when EMAIL_ENABLED=false for development and testing.
type LogSender struct {
	logger *slog.Logger
}

// Compile-time check that LogSender implements Sender.
var _ Sender = (*LogSender)(nil)

// NewLogSender creates a Sender that logs email content instead of delivering it.
func NewLogSender(logger *slog.Logger) *LogSender {
	return &LogSender{logger: logger}
}

// SendExportEmail logs the email details instead of sending via Resend.
func (s *LogSender) SendExportEmail(_ context.Context, toEmail string, zipBytes []byte) error {
	s.logger.Info("email delivery disabled: logging email content",
		slog.String("to", toEmail),
		slog.String("subject", "Your gofin data export is ready"),
		slog.Int("zip_size_bytes", len(zipBytes)),
		slog.String("mode", "dev_log_only"),
	)
	return nil
}
