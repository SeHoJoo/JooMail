package httpapi

import (
	"bufio"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func TestRulesRouteUsesManageSieveCredentialAndWritesManagedScript(t *testing.T) {
	imapHost, imapPort := startFakeIMAPServer(t, fakeIMAPScript{
		onLogin: func(username, password string) string {
			if username != "jooseho" || password != "correct-password" {
				t.Fatalf("imap login credentials = %q/%q", username, password)
			}
			return "OK LOGIN completed"
		},
	})
	var commands []string
	var authLine string
	var writtenScript string
	manageSieveHost, manageSievePort := startFakeManageSieveServer(t, &fakeManageSieveScript{
		commands: &commands,
		authLine: &authLine,
		scripts:  map[string]string{},
		active:   "",
		onPutScript: func(name string, script string) {
			if name != jooMailSieveScriptName {
				t.Fatalf("put script name = %q, want %q", name, jooMailSieveScriptName)
			}
			writtenScript = script
		},
	})
	config := testConfig(t, imapHost, imapPort)
	config.ManageSieveHost = manageSieveHost
	config.ManageSievePort = manageSievePort
	server, cookie := loginTestSession(t, config)

	body := `{"rules":[{"name":"Clients","condition":{"field":"senderDomain","match":"contains","value":"client.example"},"action":{"type":"move","mailboxId":"Work/Clients"}}]}`
	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/accounts/jooseho@good-night.co.kr/rules", strings.NewReader(body))
	req.AddCookie(cookie)
	server.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if len(commands) < 5 || commands[0] != "CAPABILITY" || !strings.HasPrefix(commands[1], "AUTHENTICATE \"PLAIN\" ") {
		t.Fatalf("commands = %#v, want capability then authenticate", commands)
	}
	authPayload := decodeManageSievePlainAuth(t, authLine)
	if authPayload != "\x00jooseho\x00correct-password" {
		t.Fatalf("auth payload = %q, want stored session credential", authPayload)
	}
	for _, want := range []string{
		jooMailRulesBegin,
		`address :domain :contains "from" "client.example"`,
		`fileinto "Work/Clients";`,
		jooMailRulesEnd,
	} {
		if !strings.Contains(writtenScript, want) {
			t.Fatalf("written script missing %q:\n%s", want, writtenScript)
		}
	}
}

func TestRulesRouteReturnsUnavailableWhenManageSieveDisabled(t *testing.T) {
	imapHost, imapPort := startFakeIMAPServer(t, fakeIMAPScript{
		onLogin: func(username, password string) string { return "OK LOGIN completed" },
	})
	server, cookie := loginTestSession(t, testConfig(t, imapHost, imapPort))

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/accounts/jooseho@good-night.co.kr/rules", nil)
	req.AddCookie(cookie)
	server.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d; body = %s", recorder.Code, http.StatusServiceUnavailable, recorder.Body.String())
	}
	var response map[string]string
	decode(t, recorder, &response)
	if response["error"] != "rules are unavailable" {
		t.Fatalf("error = %q, want rules unavailable", response["error"])
	}
}

func TestReplaceJooMailRulesBlockPreservesUserScriptContent(t *testing.T) {
	original := strings.Join([]string{
		`require "fileinto";`,
		`if header :contains "subject" "keep" { keep; }`,
		jooMailRulesBegin,
		`old managed content`,
		jooMailRulesEnd,
		`if header :contains "subject" "after" { keep; }`,
		"",
	}, "\n")
	updated := replaceJooMailRulesBlock(original, jooMailRulesBegin+"\nnew managed content\n"+jooMailRulesEnd+"\n")

	for _, want := range []string{
		`if header :contains "subject" "keep" { keep; }`,
		`new managed content`,
		`if header :contains "subject" "after" { keep; }`,
	} {
		if !strings.Contains(updated, want) {
			t.Fatalf("updated script missing %q:\n%s", want, updated)
		}
	}
	if strings.Contains(updated, "old managed content") {
		t.Fatalf("updated script kept old managed block:\n%s", updated)
	}
}

func TestBuildJooMailRulesBlockGeneratesFolderClassificationSieve(t *testing.T) {
	block, err := buildJooMailRulesBlock([]MailRule{
		{
			Condition: RuleCondition{Field: "senderEmail", Match: "equals", Value: "boss@example.com"},
			Action:    RuleAction{Type: "move", MailboxID: "Work/Boss"},
		},
		{
			Condition: RuleCondition{Field: "senderDomain", Match: "contains", Value: "vendor.example"},
			Action:    RuleAction{Type: "moveSpam"},
		},
		{
			Condition: RuleCondition{Field: "subject", Match: "contains", Value: "invoice"},
			Action:    RuleAction{Type: "moveTrash"},
		},
	})
	if err != nil {
		t.Fatalf("build rules block: %v", err)
	}
	for _, want := range []string{
		`require "fileinto";`,
		`if address :is "from" "boss@example.com" {`,
		`fileinto "Work/Boss";`,
		`if address :domain :contains "from" "vendor.example" {`,
		`fileinto "Spam";`,
		`if header :contains "subject" "invoice" {`,
		`fileinto "Trash";`,
	} {
		if !strings.Contains(block, want) {
			t.Fatalf("generated block missing %q:\n%s", want, block)
		}
	}
}

func TestBuildJooMailRulesBlockRejectsUnsupportedSubjectEquals(t *testing.T) {
	_, err := buildJooMailRulesBlock([]MailRule{
		{
			Condition: RuleCondition{Field: "subject", Match: "equals", Value: "invoice"},
			Action:    RuleAction{Type: "moveTrash"},
		},
	})
	if !errors.Is(err, errManageSieveUnsupportedRules) {
		t.Fatalf("err = %v, want unsupported rules", err)
	}
}

type fakeManageSieveScript struct {
	commands    *[]string
	authLine    *string
	scripts     map[string]string
	active      string
	onPutScript func(name string, script string)
	mu          sync.Mutex
}

func startFakeManageSieveServer(t *testing.T, script *fakeManageSieveScript) (string, string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen fake managesieve: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go serveFakeManageSieveConn(conn, script)
		}
	}()
	host, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatalf("split fake managesieve address: %v", err)
	}
	return host, port
}

func serveFakeManageSieveConn(conn net.Conn, script *fakeManageSieveScript) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	_, _ = conn.Write([]byte("\"IMPLEMENTATION\" \"fake managesieve\"\r\n\"SASL\" \"PLAIN\"\r\nOK \"ready\"\r\n"))
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		if script.commands != nil {
			*script.commands = append(*script.commands, line)
		}
		upper := strings.ToUpper(line)
		switch {
		case upper == "CAPABILITY":
			_, _ = conn.Write([]byte("\"IMPLEMENTATION\" \"fake managesieve\"\r\n\"SASL\" \"PLAIN\"\r\nOK \"capability\"\r\n"))
		case strings.HasPrefix(upper, "AUTHENTICATE "):
			if script.authLine != nil {
				*script.authLine = line
			}
			_, _ = conn.Write([]byte("OK \"authenticated\"\r\n"))
		case upper == "LISTSCRIPTS":
			script.mu.Lock()
			for name := range script.scripts {
				active := ""
				if name == script.active {
					active = " ACTIVE"
				}
				_, _ = fmt.Fprintf(conn, "%s%s\r\n", quoteSieveString(name), active)
			}
			script.mu.Unlock()
			_, _ = conn.Write([]byte("OK \"listscripts\"\r\n"))
		case strings.HasPrefix(upper, "GETSCRIPT "):
			name, _ := parseSieveQuotedPrefix(strings.TrimSpace(line[len("GETSCRIPT "):]))
			script.mu.Lock()
			body := script.scripts[name]
			script.mu.Unlock()
			_, _ = fmt.Fprintf(conn, "{%d}\r\n%s\r\nOK \"getscript\"\r\n", len(body), body)
		case strings.HasPrefix(upper, "PUTSCRIPT "):
			nameAndLiteral := strings.TrimSpace(line[len("PUTSCRIPT "):])
			name, _ := parseSieveQuotedPrefix(nameAndLiteral)
			size, ok := literalSize(line)
			if !ok {
				_, _ = conn.Write([]byte("NO \"missing literal\"\r\n"))
				continue
			}
			data := make([]byte, size)
			if _, err := io.ReadFull(reader, data); err != nil {
				return
			}
			_, _ = reader.ReadString('\n')
			script.mu.Lock()
			script.scripts[name] = string(data)
			if script.onPutScript != nil {
				script.onPutScript(name, string(data))
			}
			script.mu.Unlock()
			_, _ = conn.Write([]byte("OK \"putscript\"\r\n"))
		case strings.HasPrefix(upper, "SETACTIVE "):
			name, _ := parseSieveQuotedPrefix(strings.TrimSpace(line[len("SETACTIVE "):]))
			script.mu.Lock()
			script.active = name
			script.mu.Unlock()
			_, _ = conn.Write([]byte("OK \"setactive\"\r\n"))
		case upper == "LOGOUT":
			_, _ = conn.Write([]byte("BYE \"logout\"\r\n"))
			return
		default:
			_, _ = conn.Write([]byte("NO \"unsupported\"\r\n"))
		}
	}
}

func decodeManageSievePlainAuth(t *testing.T, line string) string {
	t.Helper()
	fields := strings.Fields(line)
	if len(fields) < 3 {
		t.Fatalf("invalid auth line: %q", line)
	}
	token, err := strconv.Unquote(fields[2])
	if err != nil {
		t.Fatalf("unquote auth token: %v", err)
	}
	payload, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		t.Fatalf("decode auth token: %v", err)
	}
	return string(payload)
}
