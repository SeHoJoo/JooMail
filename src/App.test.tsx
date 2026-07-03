import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { AppShell } from "./App";
import type { Account, Message } from "./types";

const account: Account = {
  id: "acct1",
  email: "user@example.com",
  label: "user@example.com",
  initials: "U",
  unread: 0,
  storage: "1 GB",
  mailboxes: [{ id: "inbox", label: "Inbox", kind: "inbox", unread: 0 }],
};

const staleMessage = message("stale", "Stale subject");
const freshMessage = message("fresh", "Fresh subject");

describe("AppShell search", () => {
  beforeEach(() => {
    localStorage.clear();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("ignores stale message-list responses after a newer search", async () => {
    let resolveStale: ((body: unknown) => void) | undefined;
    vi.stubGlobal(
      "fetch",
      vi.fn((input: RequestInfo | URL) => {
        const url = String(input);
        if (url.startsWith("/api/messages/")) {
          return jsonResponse({ message: freshMessage });
        }
        if (url.includes("q=fresh")) {
          return jsonResponse({ messages: [freshMessage] });
        }
        return new Promise<Response>((resolve) => {
          resolveStale = (body: unknown) => resolve(response(body));
        });
      }),
    );

    render(
      <MemoryRouter initialEntries={["/mail/acct1/inbox"]}>
        <Routes>
          <Route path="/mail/:accountId/:mailboxId" element={<AppShell initialAccounts={[account]} />} />
          <Route path="/mail/:accountId/:mailboxId/:messageId" element={<AppShell initialAccounts={[account]} />} />
        </Routes>
      </MemoryRouter>,
    );

    fireEvent.change(screen.getByPlaceholderText("메일 검색 — 발신자, 제목, 본문"), { target: { value: "fresh" } });
    await wait(350);

    await waitFor(() => expect(document.querySelector('[data-message-id="fresh"]')).toBeInTheDocument());
    resolveStale?.({ messages: [staleMessage] });

    await waitFor(() => expect(document.querySelector('[data-message-id="stale"]')).not.toBeInTheDocument());
    expect(document.querySelector('[data-message-id="fresh"]')).toBeInTheDocument();
  });
});

function message(id: string, subject: string): Message {
  return {
    id,
    accountId: "acct1",
    mailboxId: "inbox",
    sender: "Sender",
    senderEmail: "sender@example.com",
    initials: "S",
    subject,
    snippet: subject,
    time: "09:00",
    fullDate: "2026-07-03 09:00",
    unread: false,
    body: [subject],
  };
}

function jsonResponse(body: unknown) {
  return Promise.resolve(response(body));
}

function response(body: unknown) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
}

function wait(ms: number) {
  return new Promise((resolve) => window.setTimeout(resolve, ms));
}
