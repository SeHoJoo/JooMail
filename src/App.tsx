import { useEffect, useMemo, useRef, useState } from "react";
import { accounts, messages as allMessages } from "./data";
import type { MockMode } from "./types";
import { ComposePanel } from "./components/ComposePanel";
import { MessageList } from "./components/MessageList";
import { MobileInbox } from "./components/MobileInbox";
import { ReadingPane } from "./components/ReadingPane";
import { Sidebar } from "./components/Sidebar";
import { Toolbar } from "./components/Toolbar";

const LIST_WIDTH_KEY = "joomail:list-width";

export default function App() {
  return <AppShell />;
}

export function AppShell() {
  const [accountId, setAccountId] = useState(accounts[0].id);
  const [mailboxId, setMailboxId] = useState("inbox");
  const [selectedMessageId, setSelectedMessageId] = useState("m1");
  const [checkedIds, setCheckedIds] = useState<Set<string>>(new Set());
  const [search, setSearch] = useState("");
  const [composeOpen, setComposeOpen] = useState(false);
  const [mode, setMode] = useState<MockMode>("normal");
  const [listWidth, setListWidth] = useState(() => Number(localStorage.getItem(LIST_WIDTH_KEY)) || 388);
  const shellRef = useRef<HTMLDivElement>(null);

  const selectedAccount = accounts.find((account) => account.id === accountId) ?? accounts[0];

  const visibleMessages = useMemo(() => {
    const accountMessages = allMessages.filter((message) => message.accountId === accountId);
    if (search.trim()) {
      const query = search.trim().toLowerCase();
      return accountMessages.filter((message) =>
        [message.sender, message.subject, message.snippet, ...message.body].some((field) => field.toLowerCase().includes(query)),
      );
    }
    return accountMessages.filter((message) => message.mailboxId === mailboxId);
  }, [accountId, mailboxId, search]);

  const selectedMessage = visibleMessages.find((message) => message.id === selectedMessageId) ?? visibleMessages[0];
  const selectedMailbox = selectedAccount.mailboxes
    .flatMap((mailbox) => [mailbox, ...(mailbox.children ?? [])])
    .find((mailbox) => mailbox.id === mailboxId);

  useEffect(() => {
    localStorage.setItem(LIST_WIDTH_KEY, String(listWidth));
  }, [listWidth]);

  useEffect(() => {
    if (!visibleMessages.some((message) => message.id === selectedMessageId)) {
      setSelectedMessageId(visibleMessages[0]?.id ?? "");
    }
  }, [selectedMessageId, visibleMessages]);

  function selectAccount(nextAccountId: string) {
    setAccountId(nextAccountId);
    setMailboxId("inbox");
    setCheckedIds(new Set());
    triggerLoading();
  }

  function selectMailbox(nextMailboxId: string) {
    setMailboxId(nextMailboxId);
    setSearch("");
    setCheckedIds(new Set());
    setMode(nextMailboxId === "spam" ? "error" : "loading");
    window.setTimeout(() => setMode(nextMailboxId === "spam" ? "error" : "normal"), 420);
  }

  function triggerLoading() {
    setMode("loading");
    window.setTimeout(() => setMode("normal"), 420);
  }

  function retry() {
    triggerLoading();
    setMailboxId("inbox");
  }

  function toggleChecked(id: string) {
    setCheckedIds((current) => {
      const next = new Set(current);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  function startResize(event: React.PointerEvent<HTMLDivElement>) {
    const startX = event.clientX;
    const startWidth = listWidth;
    event.currentTarget.setPointerCapture(event.pointerId);

    function move(moveEvent: PointerEvent) {
      const nextWidth = Math.min(460, Math.max(320, startWidth + moveEvent.clientX - startX));
      setListWidth(nextWidth);
    }

    function stop() {
      window.removeEventListener("pointermove", move);
      window.removeEventListener("pointerup", stop);
    }

    window.addEventListener("pointermove", move);
    window.addEventListener("pointerup", stop);
  }

  return (
    <div className="min-h-screen bg-white text-ink" style={{ "--list-width": `${listWidth}px` } as React.CSSProperties} ref={shellRef}>
      <MobileInbox account={selectedAccount} messages={visibleMessages.length ? visibleMessages : allMessages.filter((message) => message.accountId === accountId)} onCompose={() => setComposeOpen(true)} onSelectMessage={setSelectedMessageId} />
      <div className="hidden h-screen flex-col md:flex">
        <Toolbar search={search} onSearch={setSearch} onCompose={() => setComposeOpen(true)} />
        <div className="min-h-0 flex flex-1">
          <Sidebar
            accounts={accounts}
            selectedAccount={selectedAccount}
            selectedMailboxId={mailboxId}
            onSelectAccount={selectAccount}
            onSelectMailbox={selectMailbox}
            onCompose={() => setComposeOpen(true)}
          />
          <MessageList
            title={selectedMailbox?.label ?? "받은편지함"}
            unreadCount={selectedAccount.unread}
            messages={visibleMessages}
            selectedId={selectedMessage?.id}
            checkedIds={checkedIds}
            search={search}
            mode={mode}
            onRetry={retry}
            onSelectMessage={setSelectedMessageId}
            onToggleChecked={toggleChecked}
            onClearChecked={() => setCheckedIds(new Set())}
          />
          <div className="hidden w-1 cursor-col-resize bg-white hover:bg-selected md:block" onPointerDown={startResize} aria-label="리스트 폭 조절" />
          <ReadingPane message={selectedMessage} mode={mode} onRetry={retry} onReply={() => setComposeOpen(true)} />
        </div>
      </div>
      {composeOpen ? <ComposePanel account={selectedAccount} message={selectedMessage} onClose={() => setComposeOpen(false)} /> : null}
    </div>
  );
}
