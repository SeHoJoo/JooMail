package httpapi

import (
	"bufio"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	manageSieveCommandTimeout = 10 * time.Second
	jooMailSieveScriptName    = "joomail"
	jooMailRulesBegin         = "# BEGIN JOOMAIL RULES"
	jooMailRulesEnd           = "# END JOOMAIL RULES"
	jooMailRulesMetadata      = "# JOOMAIL-RULES-V1:"
)

var (
	errManageSieveUnavailable      = errors.New("managesieve unavailable")
	errManageSieveUnmanagedActive  = errors.New("active sieve script is not managed by joomail")
	errManageSieveUnsupportedRules = errors.New("unsupported rule request")
)

type manageSieveClient struct {
	conn   deadlineReadWriteCloser
	reader *bufio.Reader
}

type manageSieveResponse struct {
	status  string
	lines   []string
	literal string
}

type sieveScriptInfo struct {
	name   string
	active bool
}

type saveRulesRequest struct {
	Rules []MailRule `json:"rules"`
}

func (s *Server) handleRules(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireCredential(w, r)
	if !ok {
		return
	}
	accountID := strings.TrimSpace(r.PathValue("accountID"))
	if !equalEmail(accountID, auth.credential.Email) {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	rules, err := s.loadRules(auth.credential)
	if errors.Is(err, errManageSieveUnavailable) {
		writeError(w, http.StatusServiceUnavailable, "rules are unavailable")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to load rules")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rules": rules})
}

func (s *Server) handleSaveRules(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireCredential(w, r)
	if !ok {
		return
	}
	accountID := strings.TrimSpace(r.PathValue("accountID"))
	if !equalEmail(accountID, auth.credential.Email) {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	var request saveRulesRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid rules request")
		return
	}
	if err := s.saveRules(auth.credential, request.Rules); errors.Is(err, errManageSieveUnavailable) {
		writeError(w, http.StatusServiceUnavailable, "rules are unavailable")
		return
	} else if errors.Is(err, errManageSieveUnsupportedRules) {
		writeError(w, http.StatusBadRequest, "unsupported rules request")
		return
	} else if errors.Is(err, errManageSieveUnmanagedActive) {
		writeError(w, http.StatusConflict, "active sieve script is not managed by JooMail")
		return
	} else if err != nil {
		writeError(w, http.StatusBadGateway, "failed to save rules")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rules": request.Rules})
}

func (s *Server) loadRules(credential storedCredential) ([]MailRule, error) {
	client, err := openManageSieveSession(s.config, credential)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	scripts, err := client.listScripts()
	if err != nil {
		return nil, err
	}
	for _, script := range scripts {
		if !script.active {
			continue
		}
		body, err := client.getScript(script.name)
		if err != nil {
			return nil, err
		}
		return parseJooMailRules(body), nil
	}
	for _, script := range scripts {
		if script.name != jooMailSieveScriptName {
			continue
		}
		body, err := client.getScript(script.name)
		if err != nil {
			return nil, err
		}
		return parseJooMailRules(body), nil
	}
	return []MailRule{}, nil
}

func (s *Server) saveRules(credential storedCredential, rules []MailRule) error {
	block, err := buildJooMailRulesBlock(rules)
	if err != nil {
		return err
	}

	client, err := openManageSieveSession(s.config, credential)
	if err != nil {
		return err
	}
	defer client.Close()

	scripts, err := client.listScripts()
	if err != nil {
		return err
	}
	targetName := jooMailSieveScriptName
	currentScript := ""
	for _, script := range scripts {
		if !script.active {
			continue
		}
		targetName = script.name
		currentScript, err = client.getScript(script.name)
		if err != nil {
			return err
		}
		if !containsJooMailRulesBlock(currentScript) && strings.TrimSpace(currentScript) != "" {
			return errManageSieveUnmanagedActive
		}
		break
	}
	if currentScript == "" {
		for _, script := range scripts {
			if script.name != jooMailSieveScriptName {
				continue
			}
			currentScript, err = client.getScript(script.name)
			if err != nil {
				return err
			}
			if !containsJooMailRulesBlock(currentScript) && strings.TrimSpace(currentScript) != "" {
				return errManageSieveUnmanagedActive
			}
			break
		}
	}
	updatedScript := replaceJooMailRulesBlock(currentScript, block)
	if err := client.putScript(targetName, updatedScript); err != nil {
		return err
	}
	return client.setActive(targetName)
}

func openManageSieveSession(config Config, credential storedCredential) (*manageSieveClient, error) {
	if config.ManageSieveHost == "" || config.ManageSievePort == "" {
		return nil, errManageSieveUnavailable
	}
	conn, err := dialManageSieve(config)
	if err != nil {
		return nil, err
	}
	client := &manageSieveClient{conn: conn, reader: bufio.NewReader(conn)}
	if _, err := client.readResponse(); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if _, err := client.command("CAPABILITY"); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if err := client.authenticatePlain(credential.IMAPUsername, credential.Password); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return client, nil
}

func dialManageSieve(config Config) (net.Conn, error) {
	address := net.JoinHostPort(config.ManageSieveHost, config.ManageSievePort)
	dialer := &net.Dialer{Timeout: manageSieveCommandTimeout}
	if config.ManageSieveTLS {
		return tls.DialWithDialer(dialer, "tcp", address, &tls.Config{
			ServerName: config.ManageSieveHost,
			MinVersion: tls.VersionTLS12,
		})
	}
	return dialer.Dial("tcp", address)
}

func (c *manageSieveClient) Close() error {
	_ = c.conn.SetDeadline(time.Now().Add(manageSieveCommandTimeout))
	_, _ = c.command("LOGOUT")
	return c.conn.Close()
}

func (c *manageSieveClient) authenticatePlain(username string, password string) error {
	payload := base64.StdEncoding.EncodeToString([]byte("\x00" + username + "\x00" + password))
	response, err := c.command("AUTHENTICATE \"PLAIN\" %s", quoteSieveString(payload))
	if err != nil {
		return err
	}
	if response.status != "OK" {
		return errInvalidCredentials
	}
	return nil
}

func (c *manageSieveClient) listScripts() ([]sieveScriptInfo, error) {
	response, err := c.command("LISTSCRIPTS")
	if err != nil {
		return nil, err
	}
	if response.status != "OK" {
		return nil, fmt.Errorf("listscripts failed")
	}
	scripts := make([]sieveScriptInfo, 0, len(response.lines))
	for _, line := range response.lines {
		name, ok := parseSieveQuotedPrefix(line)
		if !ok {
			continue
		}
		scripts = append(scripts, sieveScriptInfo{
			name:   name,
			active: strings.Contains(strings.ToUpper(line), "ACTIVE"),
		})
	}
	return scripts, nil
}

func (c *manageSieveClient) getScript(name string) (string, error) {
	response, err := c.command("GETSCRIPT %s", quoteSieveString(name))
	if err != nil {
		return "", err
	}
	if response.status != "OK" {
		return "", fmt.Errorf("getscript failed")
	}
	return response.literal, nil
}

func (c *manageSieveClient) putScript(name string, script string) error {
	response, err := c.commandWithLiteral("PUTSCRIPT "+quoteSieveString(name), script)
	if err != nil {
		return err
	}
	if response.status != "OK" {
		return fmt.Errorf("putscript failed")
	}
	return nil
}

func (c *manageSieveClient) setActive(name string) error {
	response, err := c.command("SETACTIVE %s", quoteSieveString(name))
	if err != nil {
		return err
	}
	if response.status != "OK" {
		return fmt.Errorf("setactive failed")
	}
	return nil
}

func (c *manageSieveClient) command(format string, args ...any) (manageSieveResponse, error) {
	if err := c.conn.SetDeadline(time.Now().Add(manageSieveCommandTimeout)); err != nil {
		return manageSieveResponse{}, err
	}
	command := fmt.Sprintf(format, args...)
	if _, err := fmt.Fprintf(c.conn, "%s\r\n", command); err != nil {
		return manageSieveResponse{}, err
	}
	return c.readResponse()
}

func (c *manageSieveClient) commandWithLiteral(command string, literal string) (manageSieveResponse, error) {
	if err := c.conn.SetDeadline(time.Now().Add(manageSieveCommandTimeout)); err != nil {
		return manageSieveResponse{}, err
	}
	if _, err := fmt.Fprintf(c.conn, "%s {%d+}\r\n%s\r\n", command, len(literal), literal); err != nil {
		return manageSieveResponse{}, err
	}
	return c.readResponse()
}

func (c *manageSieveClient) readResponse() (manageSieveResponse, error) {
	var response manageSieveResponse
	for {
		line, err := c.reader.ReadString('\n')
		if err != nil {
			return response, err
		}
		line = strings.TrimRight(line, "\r\n")
		if strings.HasPrefix(line, "{") {
			size, ok := literalSize(line)
			if !ok {
				return response, fmt.Errorf("invalid managesieve literal")
			}
			data := make([]byte, size)
			if _, err := io.ReadFull(c.reader, data); err != nil {
				return response, err
			}
			response.literal = string(data)
			_, _ = c.reader.ReadString('\n')
			continue
		}
		upper := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(upper, "OK"):
			response.status = "OK"
			return response, nil
		case strings.HasPrefix(upper, "NO"):
			response.status = "NO"
			return response, nil
		case strings.HasPrefix(upper, "BYE"):
			response.status = "BYE"
			return response, nil
		default:
			response.lines = append(response.lines, line)
		}
	}
}

func buildJooMailRulesBlock(rules []MailRule) (string, error) {
	normalized := make([]MailRule, len(rules))
	for i, rule := range rules {
		normalizedRule, err := normalizeMailRule(rule)
		if err != nil {
			return "", err
		}
		normalized[i] = normalizedRule
	}
	metadata, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	var builder strings.Builder
	builder.WriteString(jooMailRulesBegin)
	builder.WriteString("\n")
	builder.WriteString("# Managed by JooMail. Do not edit this block manually.\n")
	builder.WriteString(jooMailRulesMetadata)
	builder.WriteString(" ")
	builder.WriteString(base64.RawStdEncoding.EncodeToString(metadata))
	builder.WriteString("\n")
	builder.WriteString("require \"fileinto\";\n")
	for _, rule := range normalized {
		builder.WriteString(sieveRule(rule))
	}
	builder.WriteString(jooMailRulesEnd)
	builder.WriteString("\n")
	return builder.String(), nil
}

func normalizeMailRule(rule MailRule) (MailRule, error) {
	rule.Name = strings.TrimSpace(rule.Name)
	rule.Condition.Field = strings.TrimSpace(rule.Condition.Field)
	rule.Condition.Match = strings.TrimSpace(rule.Condition.Match)
	rule.Condition.Value = strings.TrimSpace(rule.Condition.Value)
	rule.Action.Type = strings.TrimSpace(rule.Action.Type)
	rule.Action.MailboxID = strings.TrimSpace(rule.Action.MailboxID)
	if rule.Condition.Value == "" {
		return MailRule{}, errManageSieveUnsupportedRules
	}
	switch rule.Condition.Field {
	case "senderEmail", "senderDomain", "subject":
	default:
		return MailRule{}, errManageSieveUnsupportedRules
	}
	switch rule.Condition.Match {
	case "contains", "equals":
	default:
		return MailRule{}, errManageSieveUnsupportedRules
	}
	if rule.Condition.Field == "subject" && rule.Condition.Match != "contains" {
		return MailRule{}, errManageSieveUnsupportedRules
	}
	switch rule.Action.Type {
	case "move":
		if rule.Action.MailboxID == "" {
			return MailRule{}, errManageSieveUnsupportedRules
		}
	case "moveSpam":
		rule.Action.MailboxID = "Spam"
	case "moveTrash":
		rule.Action.MailboxID = "Trash"
	default:
		return MailRule{}, errManageSieveUnsupportedRules
	}
	return rule, nil
}

func sieveRule(rule MailRule) string {
	match := ":contains"
	if rule.Condition.Match == "equals" {
		match = ":is"
	}
	var test string
	switch rule.Condition.Field {
	case "senderDomain":
		test = fmt.Sprintf("address :domain %s \"from\" %s", match, quoteSieveString(rule.Condition.Value))
	case "senderEmail":
		test = fmt.Sprintf("address %s \"from\" %s", match, quoteSieveString(rule.Condition.Value))
	default:
		test = fmt.Sprintf("header %s \"subject\" %s", match, quoteSieveString(rule.Condition.Value))
	}
	return fmt.Sprintf("if %s {\n  fileinto %s;\n  stop;\n}\n", test, quoteSieveString(rule.Action.MailboxID))
}

func parseJooMailRules(script string) []MailRule {
	block, ok := jooMailRulesBlock(script)
	if !ok {
		return []MailRule{}
	}
	for _, line := range strings.Split(block, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, jooMailRulesMetadata) {
			continue
		}
		encoded := strings.TrimSpace(strings.TrimPrefix(line, jooMailRulesMetadata))
		data, err := base64.RawStdEncoding.DecodeString(encoded)
		if err != nil {
			return []MailRule{}
		}
		var rules []MailRule
		if err := json.Unmarshal(data, &rules); err != nil {
			return []MailRule{}
		}
		return rules
	}
	return []MailRule{}
}

func replaceJooMailRulesBlock(script string, block string) string {
	start := strings.Index(script, jooMailRulesBegin)
	end := strings.Index(script, jooMailRulesEnd)
	if start >= 0 && end >= start {
		end += len(jooMailRulesEnd)
		return strings.TrimRight(script[:start], "\n") + "\n" + strings.TrimRight(block, "\n") + "\n" + strings.TrimLeft(script[end:], "\n")
	}
	if strings.TrimSpace(script) == "" {
		return block
	}
	return strings.TrimRight(script, "\n") + "\n\n" + block
}

func containsJooMailRulesBlock(script string) bool {
	_, ok := jooMailRulesBlock(script)
	return ok
}

func jooMailRulesBlock(script string) (string, bool) {
	start := strings.Index(script, jooMailRulesBegin)
	end := strings.Index(script, jooMailRulesEnd)
	if start < 0 || end < start {
		return "", false
	}
	end += len(jooMailRulesEnd)
	return script[start:end], true
}

func quoteSieveString(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return `"` + value + `"`
}

func parseSieveQuotedPrefix(line string) (string, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, `"`) {
		return "", false
	}
	escaped := false
	for i := 1; i < len(line); i++ {
		if escaped {
			escaped = false
			continue
		}
		if line[i] == '\\' {
			escaped = true
			continue
		}
		if line[i] != '"' {
			continue
		}
		value, err := strconv.Unquote(line[:i+1])
		return value, err == nil
	}
	return "", false
}
