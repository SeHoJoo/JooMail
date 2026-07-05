package httpapi

import (
	"os"
	"strings"
)

type Config struct {
	Addr            string
	StaticDir       string
	IMAPHost        string
	IMAPPort        string
	IMAPTLS         bool
	IMAPUserFormat  string
	LoginDomain     string
	SMTPHost        string
	SMTPPort        string
	SMTPTLS         bool
	SMTPStartTLS    bool
	SMTPUserFormat  string
	ManageSieveHost string
	ManageSievePort string
	ManageSieveTLS  bool
	SessionSecret   string
	CredentialKey   string
	CredentialDir   string
}

func LoadConfig() Config {
	addr := os.Getenv("JOOMAIL_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}

	return Config{
		Addr:            addr,
		StaticDir:       os.Getenv("JOOMAIL_STATIC_DIR"),
		IMAPHost:        os.Getenv("JOOMAIL_IMAP_HOST"),
		IMAPPort:        os.Getenv("JOOMAIL_IMAP_PORT"),
		IMAPTLS:         envBool("JOOMAIL_IMAP_TLS"),
		IMAPUserFormat:  os.Getenv("JOOMAIL_IMAP_USER_FORMAT"),
		LoginDomain:     os.Getenv("JOOMAIL_LOGIN_DOMAIN"),
		SMTPHost:        os.Getenv("JOOMAIL_SMTP_HOST"),
		SMTPPort:        os.Getenv("JOOMAIL_SMTP_PORT"),
		SMTPTLS:         envBool("JOOMAIL_SMTP_TLS"),
		SMTPStartTLS:    envBool("JOOMAIL_SMTP_STARTTLS"),
		SMTPUserFormat:  os.Getenv("JOOMAIL_SMTP_USER_FORMAT"),
		ManageSieveHost: os.Getenv("JOOMAIL_MANAGESIEVE_HOST"),
		ManageSievePort: os.Getenv("JOOMAIL_MANAGESIEVE_PORT"),
		ManageSieveTLS:  envBool("JOOMAIL_MANAGESIEVE_TLS"),
		// Empty secrets are allowed at startup so deploys do not crash;
		// handleLogin fails closed until JOOMAIL_SESSION_SECRET is set.
		SessionSecret: os.Getenv("JOOMAIL_SESSION_SECRET"),
		CredentialKey: os.Getenv("JOOMAIL_CREDENTIAL_KEY"),
		CredentialDir: os.Getenv("JOOMAIL_CREDENTIAL_DIR"),
	}
}

func envBool(name string) bool {
	return strings.EqualFold(os.Getenv(name), "true")
}
