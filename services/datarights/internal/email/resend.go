package email

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"text/template"
	"time"
)

//go:embed templates/export_ready.html
var htmlTemplateContent string

//go:embed templates/export_ready.txt
var textTemplateContent string

// templateData holds values passed to email templates. The brand token groups
// are used directly (BrandColors/BrandTypography/BrandSpacing) rather than
// mirrored field by field into parallel types; the templates access the same
// field names either way.
type templateData struct {
	Colors     BrandColors
	Typography BrandTypography
	Spacing    BrandSpacing
	ExportDate string
	FileName   string
}

// resendAttachment represents an email attachment for the Resend API.
type resendAttachment struct {
	Content  string `json:"content"`
	Filename string `json:"filename"`
}

// resendRequest represents the Resend API send email request body.
type resendRequest struct {
	From        string              `json:"from"`
	To          []string            `json:"to"`
	Subject     string              `json:"subject"`
	HTML        string              `json:"html"`
	Text        string              `json:"text"`
	Attachments []*resendAttachment `json:"attachments"`
}

// resendResponse represents the Resend API response.
type resendResponse struct {
	ID string `json:"id"`
}

// ResendSender implements Sender using the Resend API.
type ResendSender struct {
	apiKey     string
	from       string
	httpClient *http.Client
	htmlTmpl   *template.Template
	textTmpl   *template.Template
	tokens     BrandTokens
	logger     *slog.Logger
}

// Compile-time check that ResendSender implements Sender.
var _ Sender = (*ResendSender)(nil)

// NewResendSender creates a ResendSender with the given API key, sender address, and brand tokens.
func NewResendSender(apiKey, from string, tokens BrandTokens, logger *slog.Logger) (*ResendSender, error) {
	htmlTmpl, err := template.New("export_ready.html").Parse(htmlTemplateContent)
	if err != nil {
		return nil, fmt.Errorf("parsing HTML template: %w", err)
	}

	textTmpl, err := template.New("export_ready.txt").Parse(textTemplateContent)
	if err != nil {
		return nil, fmt.Errorf("parsing text template: %w", err)
	}

	return &ResendSender{
		apiKey:     apiKey,
		from:       from,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		htmlTmpl:   htmlTmpl,
		textTmpl:   textTmpl,
		tokens:     tokens,
		logger:     logger,
	}, nil
}

// SendExportEmail sends a branded export email with the ZIP file attached.
func (s *ResendSender) SendExportEmail(ctx context.Context, toEmail string, zipBytes []byte) error {
	if toEmail == "" {
		return fmt.Errorf("recipient email address is empty")
	}
	now := time.Now()
	exportDate := now.Format("2006-01-02")
	fileName := fmt.Sprintf("gofin-export-%s.zip", exportDate)

	data := s.buildTemplateData(exportDate, fileName)

	htmlBody, err := s.renderHTML(data)
	if err != nil {
		return fmt.Errorf("rendering email HTML: %w", err)
	}

	textBody, err := s.renderText(data)
	if err != nil {
		return fmt.Errorf("rendering email text: %w", err)
	}

	encoded := base64.StdEncoding.EncodeToString(zipBytes)

	reqBody := resendRequest{
		From:    s.from,
		To:      []string{toEmail},
		Subject: "Your gofin data export is ready",
		HTML:    htmlBody,
		Text:    textBody,
		Attachments: []*resendAttachment{
			{
				Content:  encoded,
				Filename: fileName,
			},
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshaling email request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("creating email request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending email via Resend: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return fmt.Errorf("reading Resend response body: %w", readErr)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("resend API error (status %d): %s", resp.StatusCode, string(body))
	}

	var resendResp resendResponse
	if err := json.Unmarshal(body, &resendResp); err != nil {
		return fmt.Errorf("parsing Resend response: %w", err)
	}

	s.logger.Info("export email sent",
		slog.String("resend_id", resendResp.ID),
		slog.String("to", toEmail),
		slog.String("filename", fileName),
	)

	return nil
}

// buildTemplateData constructs the template rendering context from brand tokens.
func (s *ResendSender) buildTemplateData(exportDate, fileName string) templateData {
	return templateData{
		Colors:     s.tokens.Colors,
		Typography: s.tokens.Typography,
		Spacing:    s.tokens.Spacing,
		ExportDate: exportDate,
		FileName:   fileName,
	}
}

func (s *ResendSender) renderHTML(data templateData) (string, error) {
	var buf bytes.Buffer
	if err := s.htmlTmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (s *ResendSender) renderText(data templateData) (string, error) {
	var buf bytes.Buffer
	if err := s.textTmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// LoadBrandTokens parses brand tokens from JSON bytes.
func LoadBrandTokens(data []byte) (BrandTokens, error) {
	var tokens BrandTokens
	if err := json.Unmarshal(data, &tokens); err != nil {
		return BrandTokens{}, fmt.Errorf("parsing brand tokens: %w", err)
	}
	return tokens, nil
}
