import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
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
  mailboxes: [
    { id: "inbox", label: "Inbox", kind: "inbox", unread: 0 },
    { id: "sent", label: "Sent", kind: "sent", unread: 0 },
  ],
};

const staleMessage = message("stale", "Stale subject");
const freshMessage = message("fresh", "Fresh subject");
const inboxMessage = message("inbox-1", "Inbox subject");
const unreadInboxMessage = { ...inboxMessage, unread: true };
const sentMessage = { ...message("sent-1", "Sent subject"), mailboxId: "sent" };
const sendWarning = "전송은 완료됐지만 보낸편지함에 저장하지 못했습니다";

describe("AppShell live mail behavior", () => {
  beforeEach(() => {
    localStorage.clear();
  });

  afterEach(() => {
    cleanup();
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

    renderApp();

    fireEvent.change(screen.getByPlaceholderText("메일 검색 — 발신자, 제목, 본문"), { target: { value: "fresh" } });
    await wait(350);

    await waitFor(() => expect(messageRow("fresh")).toBeInTheDocument());
    resolveStale?.({ messages: [staleMessage] });

    await waitFor(() => expect(messageRow("stale")).not.toBeInTheDocument());
    expect(messageRow("fresh")).toBeInTheDocument();
  });

  it("keeps a mailbox-list route unselected after messages load", async () => {
    const fetchMock = vi.fn((input: RequestInfo | URL) => {
      const url = String(input);
      if (url.startsWith("/api/accounts/acct1/mailboxes/inbox/messages")) {
        return jsonResponse({ messages: [inboxMessage] });
      }
      return jsonResponse({ message: { ...inboxMessage, textBody: ["Loaded inbox body"] } });
    });
    vi.stubGlobal("fetch", fetchMock);

    renderApp();

    await waitFor(() => expect(messageRow("inbox-1")).toBeInTheDocument());
    expect(screen.getByText("메일을 선택하세요")).toBeInTheDocument();
    expect(requestsTo(fetchMock, "/api/messages/inbox-1")).toHaveLength(0);
  });

  it("does not restore a persisted message over an explicit mailbox-list route", async () => {
    localStorage.setItem(
      "joomail:mail-state",
      JSON.stringify({
        activeAccountId: "acct1",
        searchScope: "mailbox",
        byAccount: { acct1: { mailboxId: "inbox", messageId: "inbox-1" } },
      }),
    );
    const fetchMock = vi.fn((input: RequestInfo | URL) => {
      const url = String(input);
      if (url.startsWith("/api/accounts/acct1/mailboxes/inbox/messages")) {
        return jsonResponse({ messages: [inboxMessage] });
      }
      return jsonResponse({ message: { ...inboxMessage, textBody: ["Persisted body"] } });
    });
    vi.stubGlobal("fetch", fetchMock);

    renderApp();

    await waitFor(() => expect(messageRow("inbox-1")).toBeInTheDocument());
    expect(screen.getByText("메일을 선택하세요")).toBeInTheDocument();
    expect(requestsTo(fetchMock, "/api/messages/inbox-1")).toHaveLength(0);
  });

  it("clears a stale selected id without selecting the first returned message", async () => {
    const staleDetail = deferred<Response>();
    const fetchMock = vi.fn((input: RequestInfo | URL) => {
      const url = String(input);
      if (url.startsWith("/api/accounts/acct1/mailboxes/inbox/messages")) {
        return jsonResponse({ messages: [freshMessage] });
      }
      if (url === "/api/messages/stale") return staleDetail.promise;
      if (url === "/api/messages/fresh") {
        return jsonResponse({ message: { ...freshMessage, textBody: ["Unexpected fresh body"] } });
      }
      return jsonResponse({});
    });
    vi.stubGlobal("fetch", fetchMock);

    renderApp("/mail/acct1/inbox/stale");

    await waitFor(() => expect(messageRow("fresh")).toBeInTheDocument());
    await waitFor(() => expect(screen.getByText("메일을 선택하세요")).toBeInTheDocument());
    expect(requestsTo(fetchMock, "/api/messages/fresh")).toHaveLength(0);
  });

  it("marks an unread row seen only after its detail opens", async () => {
    const detail = deferred<Response>();
    const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      if (url.startsWith("/api/accounts/acct1/mailboxes/inbox/messages")) {
        return jsonResponse({ messages: [unreadInboxMessage] });
      }
      if (url === "/api/messages/inbox-1") return detail.promise;
      if (url === "/api/messages/inbox-1/seen") return jsonResponse({ message: unreadInboxMessage });
      return jsonResponse({ init });
    });
    vi.stubGlobal("fetch", fetchMock);

    renderApp();
    await waitFor(() => expect(messageRow("inbox-1")).toBeInTheDocument());
    fireEvent.click(messageRow("inbox-1")!);

    await waitFor(() => expect(requestsTo(fetchMock, "/api/messages/inbox-1")).toHaveLength(1));
    expect(requestsTo(fetchMock, "/api/messages/inbox-1/seen")).toHaveLength(0);

    detail.resolve(response({ message: { ...unreadInboxMessage, textBody: ["Unread body"] } }));

    await waitFor(() => expect(screen.getByText("Unread body")).toBeInTheDocument());
    await waitFor(() => expect(requestsTo(fetchMock, "/api/messages/inbox-1/seen")).toHaveLength(1));
    expect(seenRequestBody(fetchMock)).toEqual({ seen: true });
    expect(unreadMarker("inbox-1")).toHaveAttribute("data-show", "false");
  });

  it("restores unread when the seen patch fails and leaves detail open", async () => {
    const seenPatch = deferred<Response>();
    const fetchMock = vi.fn((input: RequestInfo | URL) => {
      const url = String(input);
      if (url.startsWith("/api/accounts/acct1/mailboxes/inbox/messages")) {
        return jsonResponse({ messages: [unreadInboxMessage] });
      }
      if (url === "/api/messages/inbox-1") {
        return jsonResponse({ message: { ...unreadInboxMessage, textBody: ["Rollback body"] } });
      }
      if (url === "/api/messages/inbox-1/seen") return seenPatch.promise;
      return jsonResponse({});
    });
    vi.stubGlobal("fetch", fetchMock);

    renderApp();
    await waitFor(() => expect(messageRow("inbox-1")).toBeInTheDocument());
    fireEvent.click(messageRow("inbox-1")!);

    await waitFor(() => expect(requestsTo(fetchMock, "/api/messages/inbox-1/seen")).toHaveLength(1));
    expect(unreadMarker("inbox-1")).toHaveAttribute("data-show", "false");
    seenPatch.resolve(response({}, 500));

    await waitFor(() => expect(unreadMarker("inbox-1")).toHaveAttribute("data-show", "true"));
    expect(screen.getByText("Rollback body")).toBeInTheDocument();
    expect(screen.queryByText("메일을 불러오지 못했습니다")).not.toBeInTheDocument();
  });

  it("uses the shared detail-then-seen flow for keyboard selection", async () => {
    const fetchMock = seenFlowFetch();
    vi.stubGlobal("fetch", fetchMock);

    renderApp();
    await waitFor(() => expect(messageRow("inbox-1")).toBeInTheDocument());
    fireEvent.keyDown(window, { key: "j" });

    await waitFor(() => expect(screen.getByText("Opened body")).toBeInTheDocument());
    expect(requestsTo(fetchMock, "/api/messages/inbox-1/seen")).toHaveLength(1);
    expect(seenRequestBody(fetchMock)).toEqual({ seen: true });
  });

  it("uses the shared detail-then-seen flow for a direct message route", async () => {
    const fetchMock = seenFlowFetch();
    vi.stubGlobal("fetch", fetchMock);

    renderApp("/mail/acct1/inbox/inbox-1");

    await waitFor(() => expect(screen.getByText("Opened body")).toBeInTheDocument());
    expect(requestsTo(fetchMock, "/api/messages/inbox-1/seen")).toHaveLength(1);
    expect(seenRequestBody(fetchMock)).toEqual({ seen: true });
  });

  it("closes compose and shows a dismissible warning when the sent copy is not stored", async () => {
    const fetchMock = vi.fn((input: RequestInfo | URL) => {
      const url = String(input);
      if (url === "/api/send") {
        return jsonResponse({ status: "sent", sentCopyStored: false });
      }
      if (url.startsWith("/api/accounts/acct1/mailboxes/inbox/messages")) {
        return jsonResponse({ messages: [] });
      }
      return jsonResponse({});
    });
    vi.stubGlobal("fetch", fetchMock);

    renderApp();
    await waitFor(() => expect(screen.getAllByText("받은편지함이 비어 있습니다").length).toBeGreaterThan(0));
    fireEvent.click(screen.getAllByRole("button", { name: "새 메일 쓰기" })[0]);
    fireEvent.change(screen.getByLabelText("받는사람"), { target: { value: "friend@example.com" } });
    fireEvent.change(screen.getByLabelText("제목"), { target: { value: "Hello" } });
    fireEvent.click(screen.getByRole("button", { name: /^보내기/ }));

    const status = await screen.findByRole("status");
    expect(status).toHaveTextContent(sendWarning);
    expect(screen.queryByLabelText("받는사람")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "전송 알림 닫기" }));
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
  });

  it("keeps a sidebar mailbox click on the selected mailbox without opening its first row", async () => {
    const fetchMock = vi.fn((input: RequestInfo | URL) => {
      const url = String(input);
      if (url.startsWith("/api/accounts/acct1/mailboxes/sent/messages")) {
        return jsonResponse({ messages: [sentMessage] });
      }
      if (url.startsWith("/api/accounts/acct1/mailboxes/inbox/messages")) {
        return jsonResponse({ messages: [inboxMessage] });
      }
      if (url === "/api/messages/inbox-1") {
        return jsonResponse({ message: { ...inboxMessage, textBody: ["Loaded inbox body"] } });
      }
      if (url === "/api/messages/sent-1") {
        return jsonResponse({ message: { ...sentMessage, textBody: ["Loaded sent body"] } });
      }
      return jsonResponse({});
    });
    vi.stubGlobal("fetch", fetchMock);

    renderApp();
    await waitFor(() => expect(messageRow("inbox-1")).toBeInTheDocument());
    fireEvent.click(screen.getAllByRole("button", { name: "Sent" })[1]);

    await waitFor(() => expect(messageRow("sent-1")).toBeInTheDocument());
    expect(screen.getByText("메일을 선택하세요")).toBeInTheDocument();
    expect(requestsTo(fetchMock, "/api/messages/sent-1")).toHaveLength(0);
  });
});

function renderApp(initialEntry = "/mail/acct1/inbox") {
  return render(
    <MemoryRouter initialEntries={[initialEntry]}>
      <Routes>
        <Route path="/mail/:accountId/:mailboxId" element={<AppShell initialAccounts={[account]} />} />
        <Route path="/mail/:accountId/:mailboxId/:messageId" element={<AppShell initialAccounts={[account]} />} />
      </Routes>
    </MemoryRouter>,
  );
}

function seenFlowFetch() {
  return vi.fn((input: RequestInfo | URL) => {
    const url = String(input);
    if (url.startsWith("/api/accounts/acct1/mailboxes/inbox/messages")) {
      return jsonResponse({ messages: [unreadInboxMessage] });
    }
    if (url === "/api/messages/inbox-1") {
      return jsonResponse({ message: { ...unreadInboxMessage, textBody: ["Opened body"] } });
    }
    if (url === "/api/messages/inbox-1/seen") {
      return jsonResponse({ message: { ...unreadInboxMessage, unread: false } });
    }
    return jsonResponse({});
  });
}

function messageRow(id: string) {
  return document.querySelector<HTMLElement>(`[data-message-row][data-message-id="${id}"]`);
}

function unreadMarker(id: string) {
  return messageRow(id)?.querySelector<HTMLElement>("[data-show]");
}

function requestsTo(mock: ReturnType<typeof vi.fn>, url: string) {
  return mock.mock.calls.filter(([input]) => String(input) === url);
}

function seenRequestBody(mock: ReturnType<typeof vi.fn>) {
  const call = requestsTo(mock, "/api/messages/inbox-1/seen")[0];
  return JSON.parse(String(call?.[1]?.body));
}

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

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((nextResolve) => {
    resolve = nextResolve;
  });
  return { promise, resolve };
}

function jsonResponse(body: unknown) {
  return Promise.resolve(response(body));
}

function response(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

function wait(ms: number) {
  return new Promise((resolve) => window.setTimeout(resolve, ms));
}
