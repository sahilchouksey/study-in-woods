package services

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/smtp"
	"os"
	"strings"
)

// EmailService handles sending emails via SMTP
type EmailService struct {
	host     string
	port     int
	username string
	password string
	from     string
	appURL   string
}

// NewEmailService creates a new email service instance
func NewEmailService() *EmailService {
	port := 587
	if p := os.Getenv("SMTP_PORT"); p != "" {
		fmt.Sscanf(p, "%d", &port)
	}

	return &EmailService{
		host:     getEnvOrDefault("SMTP_HOST", "smtp.gmail.com"),
		port:     port,
		username: os.Getenv("SMTP_USERNAME"),
		password: os.Getenv("SMTP_PASSWORD"),
		from:     getEnvOrDefault("SMTP_FROM", "noreply@studyinwoods.app"),
		appURL:   getEnvOrDefault("APP_URL", "http://localhost:3000"),
	}
}

// getEnvOrDefault returns the environment variable value or a default
func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// IsConfigured checks if SMTP is properly configured
func (e *EmailService) IsConfigured() bool {
	return e.username != "" && e.password != ""
}

// SendPasswordResetEmail sends a password reset email to the user
func (e *EmailService) SendPasswordResetEmail(toEmail, resetToken, userName string) error {
	if !e.IsConfigured() {
		log.Printf("SMTP not configured. Reset token for %s: %s", toEmail, resetToken)
		return fmt.Errorf("SMTP not configured")
	}

	resetLink := fmt.Sprintf("%s/reset-password?token=%s", e.appURL, resetToken)

	subject := "Reset Your Password - Study in Woods"
	body := e.buildPasswordResetEmailBody(userName, resetLink)

	return e.sendEmail(toEmail, subject, body)
}

// buildPasswordResetEmailBody creates the HTML email body for password reset
func (e *EmailService) buildPasswordResetEmailBody(userName, resetLink string) string {
	if userName == "" {
		userName = "User"
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Reset Your Password - Study in Woods</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, sans-serif;
            line-height: 1.6;
            color: #333;
            max-width: 600px;
            margin: 0 auto;
            padding: 20px;
            background-color: #f5f5f5;
        }
        .container {
            background-color: #ffffff;
            border-radius: 8px;
            padding: 40px;
            box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
        }
        .logo {
            text-align: center;
            margin-bottom: 30px;
            padding-bottom: 20px;
            border-bottom: 2px solid #2d5016;
        }
        .logo h1 {
            color: #2d5016;
            font-size: 28px;
            margin: 0;
            letter-spacing: -0.5px;
        }
        .logo .domain {
            color: #666;
            font-size: 14px;
            margin-top: 5px;
        }
        h2 {
            color: #2d5016;
            margin-top: 0;
        }
        .button {
            display: inline-block;
            background-color: #2d5016;
            color: #ffffff !important;
            padding: 14px 28px;
            text-decoration: none;
            border-radius: 6px;
            font-weight: 600;
            margin: 20px 0;
        }
        .button:hover {
            background-color: #1e3a0f;
        }
        .link-text {
            word-break: break-all;
            color: #666;
            font-size: 12px;
            background-color: #f5f5f5;
            padding: 10px;
            border-radius: 4px;
            margin-top: 15px;
        }
        .footer {
            margin-top: 30px;
            padding-top: 20px;
            border-top: 1px solid #eee;
            font-size: 12px;
            color: #666;
            text-align: center;
        }
        .footer a {
            color: #2d5016;
            text-decoration: none;
        }
        .warning {
            background-color: #fff3cd;
            border: 1px solid #ffc107;
            border-radius: 4px;
            padding: 12px;
            margin-top: 20px;
            font-size: 13px;
        }
        .social-links {
            margin-top: 15px;
        }
        .social-links a {
            color: #2d5016;
            text-decoration: none;
            margin: 0 10px;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="logo">
            <h1>Study in Woods</h1>
            <div class="domain">studyinwoods.app</div>
        </div>
        
        <h2>Reset Your Password</h2>
        
        <p>Hello %s,</p>
        
        <p>We received a request to reset the password for your Study in Woods account. Click the button below to create a new password:</p>
        
        <p style="text-align: center;">
            <a href="%s" class="button">Reset Password</a>
        </p>
        
        <p>If the button doesn't work, copy and paste this link into your browser:</p>
        <div class="link-text">%s</div>
        
        <div class="warning">
            <strong>Important:</strong> This link will expire in 1 hour for security reasons. If you didn't request a password reset, please ignore this email or contact support if you have concerns.
        </div>
        
        <div class="footer">
            <p><strong>Study in Woods</strong></p>
            <p>Your AI-powered study companion</p>
            <p><a href="https://studyinwoods.app">studyinwoods.app</a></p>
            <div class="social-links">
                <a href="https://studyinwoods.app">Website</a> |
                <a href="mailto:support@studyinwoods.app">Support</a>
            </div>
            <p style="margin-top: 15px; color: #999;">
                If you didn't request this password reset, you can safely ignore this email.
            </p>
        </div>
    </div>
</body>
</html>`, userName, resetLink, resetLink)
}

// sendEmail sends an email using SMTP with TLS
func (e *EmailService) sendEmail(to, subject, htmlBody string) error {
	// Build the email message with proper headers
	headers := make(map[string]string)
	headers["From"] = fmt.Sprintf("Study in Woods | studyinwoods.app <%s>", e.from)
	headers["Reply-To"] = "support@studyinwoods.app"
	headers["To"] = to
	headers["Subject"] = subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/html; charset=UTF-8"
	headers["X-Mailer"] = "Study in Woods Mailer"

	var message strings.Builder
	for k, v := range headers {
		message.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	message.WriteString("\r\n")
	message.WriteString(htmlBody)

	// Connect to the SMTP server
	addr := fmt.Sprintf("%s:%d", e.host, e.port)

	// Use PlainAuth for Gmail
	auth := smtp.PlainAuth("", e.username, e.password, e.host)

	// TLS configuration
	tlsConfig := &tls.Config{
		InsecureSkipVerify: false,
		ServerName:         e.host,
	}

	// Connect to the server
	conn, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	defer conn.Close()

	// Start TLS
	if err := conn.StartTLS(tlsConfig); err != nil {
		return fmt.Errorf("failed to start TLS: %w", err)
	}

	// Authenticate
	if err := conn.Auth(auth); err != nil {
		return fmt.Errorf("SMTP authentication failed: %w", err)
	}

	// Set the sender
	if err := conn.Mail(e.from); err != nil {
		return fmt.Errorf("failed to set sender: %w", err)
	}

	// Set the recipient
	if err := conn.Rcpt(to); err != nil {
		return fmt.Errorf("failed to set recipient: %w", err)
	}

	// Send the email body
	w, err := conn.Data()
	if err != nil {
		return fmt.Errorf("failed to get data writer: %w", err)
	}

	_, err = w.Write([]byte(message.String()))
	if err != nil {
		return fmt.Errorf("failed to write email body: %w", err)
	}

	err = w.Close()
	if err != nil {
		return fmt.Errorf("failed to close data writer: %w", err)
	}

	// Close the connection
	conn.Quit()

	log.Printf("Password reset email sent successfully to: %s", to)
	return nil
}
