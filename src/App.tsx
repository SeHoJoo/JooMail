import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { accounts as mockAccounts, messages as mockMessages } from "./data";
import type { Account, ComposeDraft, ComposeMode, MailRule, Mailbox, Message, MockMode, SearchScope } from "./types";
import { ComposePanel } from "./components/ComposePanel";
import { MessageList } from "./components/MessageList";
import { MobileInbox } from "./components/MobileInbox";
import { ReadingPane } from "./components/ReadingPane";
import { Sidebar } from "./components/Sidebar";
import { SettingsPanel } from "./components/SettingsPanel";
import { Toolbar } from "./components/Toolbar";
import { DevStateSwitcher, type QaState } from "./components/DevStateSwitcher";
import { LoginPage } from "./components/LoginPage";
import { AddAccountModal } from "./components/AddAccountModal";
import { Icon } from "./components/Icon";

const LIST_WIDTH_KEY = "joomail:list-width";
const REMOTE_IMAGES_KEY = "joomail:remote-images";
const ACCOUNT_NAMES_KEY = "joomail:account-names";
const MAIL_STATE_KEY = "joomail:mail-state";
const DEFAULT_LIST_WIDTH = 388;
const SEARCH_DEBOUNCE_MS = 300;
const SENT_COPY_WARNING = "전송은 완료됐지만 보낸편지함에 저장하지 못했습니다";
const QA_STATES: QaState[] = [
  "normal",
  "loading",
  "error",
  "empty",
  "empty-reading",
  "search",
  "search-account",
  "search-empty",
  "multiselect",
  "compose",
  "remote-images-shown",
  "quoted-expanded",
  "long-overflow",
  "many-attachments",
  "empty-custom-folder",
  "nested-tree",
  "mobile-reading-attachments",
  "compose-cc-bcc",
  "send-warning",
  "multi-account",
  "account-unavailable",
  "starred",
];
const SEARCH_QA_QUERY = "MIME";
const SEARCH_EMPTY_QA_QUERY = "qa-no-results-000";

export default function App() {
  const qaParam = new URLSearchParams(window.location.search).get("qa");
  if (import.meta.env.DEV && qaParam && qaParam !== "login") {
    return <AppShell />;
  }

  return <LoginGate />;
}

function LoginGate() {
  const [accounts, setAccounts] = useState<Account[] | null>(null);
  const [checking, setChecking] = useState(true);

  useEffect(() => {
    checkSession();
  }, []);

  async function checkSession() {
    setChecking(true);
    try {
      const body = await apiJSON<{ accounts: Account[] }>("/api/accounts");
      setAccounts(body.accounts);
    } catch {
      setAccounts(null);
    } finally {
      setChecking(false);
    }
  }

  if (checking) {
    return <div className="min-h-screen bg-panel" />;
  }

  if (!accounts) {
    return <LoginPage onLoginSuccess={() => checkSession()} />;
  }

  return <AppShell initialAccounts={accounts} onSessionExpired={() => setAccounts(null)} />;
}

type AppShellProps = {
  initialAccounts?: Account[];
  onSessionExpired?: () => void;
};

export function AppShell({ initialAccounts, onSessionExpired }: AppShellProps) {
  const useApi = Boolean(initialAccounts);
  const navigate = useNavigate();
  const routeParams = useParams<{ accountId?: string; mailboxId?: string; messageId?: string }>();
  const initialAccountList = initialAccounts ?? mockAccounts;
  const restoredMailState = loadMailState(initialAccountList);
  const routeAccountId = useApi && routeParams.accountId && initialAccountList.some((account) => account.id === routeParams.accountId) ? routeParams.accountId : "";
  const initialAccountId = routeAccountId || restoredMailState.activeAccountId || initialAccountList[0].id;
  const initialMailboxId = useApi && routeParams.mailboxId ? routeParams.mailboxId : restoredMailState.byAccount[initialAccountId]?.mailboxId ?? "inbox";
  const [accounts, setAccounts] = useState<Account[]>(initialAccountList);
  const [accountId, setAccountId] = useState(initialAccountId);
  const [mailboxId, setMailboxId] = useState(initialMailboxId);
  const [selectedMessageId, setSelectedMessageId] = useState(useApi ? routeParams.messageId ?? "" : restoredMailState.byAccount[initialAccountId]?.messageId ?? "m1");
  const [checkedIds, setCheckedIds] = useState<Set<string>>(new Set());
  const [lastCheckedId, setLastCheckedId] = useState("");
  const [searchInput, setSearchInput] = useState("");
  const [search, setSearch] = useState("");
  const [searchScope, setSearchScope] = useState<SearchScope>(restoredMailState.searchScope);
  const [mailStateByAccount, setMailStateByAccount] = useState<Record<string, AccountMailState>>(restoredMailState.byAccount);
  const [composeOpen, setComposeOpen] = useState(false);
  const [composeDirty, setComposeDirty] = useState(false);
  const [sendWarning, setSendWarning] = useState("");
  const [replyMessageId, setReplyMessageId] = useState<string | null>(null);
  const [composeMode, setComposeMode] = useState<ComposeMode>("compose");
  const [mode, setMode] = useState<MockMode>("normal");
  const [apiMessages, setApiMessages] = useState<Message[]>([]);
  const [selectedMessageDetail, setSelectedMessageDetail] = useState<Message | undefined>();
  const [reloadToken, setReloadToken] = useState(0);
  const [listWidth, setListWidth] = useState(() => Number(localStorage.getItem(LIST_WIDTH_KEY)) || DEFAULT_LIST_WIDTH);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [addAccountOpen, setAddAccountOpen] = useState(false);
  const [reauthEmail, setReauthEmail] = useState("");
  const [accountNames, setAccountNames] = useState<Record<string, string>>(() => loadAccountNames());
  const [showRemoteImagesByDefault, setShowRemoteImagesByDefault] = useState(() => localStorage.getItem(REMOTE_IMAGES_KEY) === "true");
  const [forceEmptyList, setForceEmptyList] = useState(false);
  const [forceMobileReadingId, setForceMobileReadingId] = useState("");
  const [forceMobileFolderOpen, setForceMobileFolderOpen] = useState(false);
  const [forceQuotedOpen, setForceQuotedOpen] = useState(false);
  const [forceComposeCcBcc, setForceComposeCcBcc] = useState(false);
  const shellRef = useRef<HTMLDivElement>(null);
  const searchInputRef = useRef<HTMLInputElement>(null);
  const composeDirtyRef = useRef(false);
  const initialQaAppliedRef = useRef(false);
  const lastRouteRef = useRef({
    accountId: routeParams.accountId ?? "",
    mailboxId: routeParams.mailboxId ?? "inbox",
    messageId: routeParams.messageId ?? "",
  });

  const displayAccounts = useMemo(() => accounts.map((account) => withDisplayName(account, accountNames[account.email])), [accounts, accountNames]);
  const selectedAccount = displayAccounts.find((account) => account.id === accountId) ?? displayAccounts[0];

  const visibleMessages = useMemo(() => {
    if (useApi) return apiMessages;
    if (forceEmptyList) return [];
    const accountMessages = mockMessages.filter((message) => message.accountId === accountId);
    if (search.trim()) {
      const query = search.trim().toLowerCase();
      const scopedMessages = searchScope === "account" ? accountMessages : accountMessages.filter((message) => message.mailboxId === mailboxId);
      return scopedMessages.filter((message) =>
        [message.sender, message.subject, message.snippet, ...message.body].some((field) => field.toLowerCase().includes(query)),
      );
    }
    return accountMessages.filter((message) => message.mailboxId === mailboxId);
  }, [accountId, apiMessages, forceEmptyList, mailboxId, search, searchScope, useApi]);

  const selectedMessage = useApi ? selectedMessageDetail ?? visibleMessages.find((message) => message.id === selectedMessageId) : visibleMessages.find((message) => message.id === selectedMessageId);
  const composeMessage = replyMessageId
    ? useApi
      ? selectedMessageDetail?.id === replyMessageId
        ? selectedMessageDetail
        : visibleMessages.find((message) => message.id === replyMessageId)
      : mockMessages.find((message) => message.id === replyMessageId)
    : undefined;
  const flatMailboxes = useMemo(() => flattenMailboxes(selectedAccount.mailboxes), [selectedAccount.mailboxes]);
  const selectedMailbox = flatMailboxes.find((mailbox) => mailbox.id === mailboxId);

  useEffect(() => {
    localStorage.setItem(LIST_WIDTH_KEY, String(listWidth));
  }, [listWidth]);

  useEffect(() => {
    localStorage.setItem(REMOTE_IMAGES_KEY, String(showRemoteImagesByDefault));
  }, [showRemoteImagesByDefault]);

  useEffect(() => {
    localStorage.setItem(ACCOUNT_NAMES_KEY, JSON.stringify(accountNames));
  }, [accountNames]);

  useEffect(() => {
    if (!useApi) return;
    if (!routeParams.accountId || !accounts.some((account) => account.id === routeParams.accountId)) return;

    const nextAccountId = routeParams.accountId;
    const nextMailboxId = routeParams.mailboxId ?? "inbox";
    const nextMessageId = routeParams.messageId ?? "";
    const previousRoute = lastRouteRef.current;
    const mailboxChanged = previousRoute.accountId !== nextAccountId || previousRoute.mailboxId !== nextMailboxId;
    const messageChanged = previousRoute.messageId !== nextMessageId;
    if (!mailboxChanged && !messageChanged) return;
    lastRouteRef.current = { accountId: nextAccountId, mailboxId: nextMailboxId, messageId: nextMessageId };

    setAccountId((current) => (current === nextAccountId ? current : nextAccountId));
    setMailboxId((current) => (current === nextMailboxId ? current : nextMailboxId));
    setSelectedMessageId((current) => (current === nextMessageId ? current : nextMessageId));
    setSelectedMessageDetail((current) => (current && current.id === nextMessageId && !mailboxChanged ? current : undefined));
    if (mailboxChanged) {
      setSearchInput("");
      setSearch("");
      setCheckedIds(new Set());
      setLastCheckedId("");
    }
  }, [accounts, routeParams.accountId, routeParams.mailboxId, routeParams.messageId, useApi]);

  useEffect(() => {
    if (!useApi || !accountId || !mailboxId) return;
    const encodedAccount = encodeURIComponent(accountId);
    const encodedMailbox = encodeURIComponent(mailboxId);
    const encodedMessage = selectedMessageId ? `/${encodeURIComponent(selectedMessageId)}` : "";
    const nextPath = `/mail/${encodedAccount}/${encodedMailbox}${encodedMessage}`;
    if (window.location.pathname !== nextPath) {
      navigate(`${nextPath}${window.location.search}`, { replace: true });
    }
  }, [accountId, mailboxId, navigate, selectedMessageId, useApi]);

  useEffect(() => {
    const nextState = {
      activeAccountId: accountId,
      searchScope,
      byAccount: {
        ...mailStateByAccount,
        [accountId]: { mailboxId, messageId: selectedMessageId },
      },
    };
    localStorage.setItem(MAIL_STATE_KEY, JSON.stringify(nextState));
    setMailStateByAccount(nextState.byAccount);
  }, [accountId, mailboxId, selectedMessageId, searchScope]);

  useEffect(() => {
    if (!useApi) return;
    let cancelled = false;
    setMode("loading");
    setSelectedMessageDetail(undefined);
    apiJSON<{ messages: ApiMessage[] }>(messageSummariesPath(accountId, mailboxId, search, searchScope))
      .then((body) => {
        if (cancelled) return;
        const nextMessages = body.messages.map(summaryToMessage);
        setApiMessages(nextMessages);
        setMode("normal");
      })
      .catch((error) => {
        if (cancelled) return;
        if (isUnauthorized(error)) {
          onSessionExpired?.();
          return;
        }
        setApiMessages([]);
        setSelectedMessageId("");
        setMode("error");
      });
    return () => {
      cancelled = true;
    };
  }, [accountId, mailboxId, onSessionExpired, reloadToken, search, searchScope, useApi]);

  useEffect(() => {
    if (!useApi || !selectedMessageId) {
      if (useApi) setSelectedMessageDetail(undefined);
      return;
    }
    let cancelled = false;
    apiJSON<{ message: ApiMessage }>(`/api/messages/${encodeURIComponent(selectedMessageId)}`)
      .then((body) => {
        if (!cancelled) {
          const message = normalizeMessage(body.message);
          setSelectedMessageDetail(message);
          updateMessageUnread(message.id, false);
          if (message.unread) {
            void apiJSON(`/api/messages/${encodeURIComponent(message.id)}/seen`, {
              method: "PATCH",
              body: JSON.stringify({ seen: true }),
            }).catch((error) => {
              if (!cancelled) updateMessageUnread(message.id, true);
              if (isUnauthorized(error)) onSessionExpired?.();
            });
          }
        }
      })
      .catch((error) => {
        if (cancelled) return;
        if (isUnauthorized(error)) {
          onSessionExpired?.();
          return;
        }
        if (error instanceof ApiError && error.status === 404) {
          setSelectedMessageDetail(undefined);
          setSelectedMessageId("");
          navigate(`/mail/${encodeURIComponent(accountId)}/${encodeURIComponent(mailboxId)}`, { replace: true });
          return;
        }
        setMode("error");
      });
    return () => {
      cancelled = true;
    };
  }, [onSessionExpired, selectedMessageId, useApi]);

  useEffect(() => {
    if (useApi) return;
    if (!visibleMessages.length) {
      if (selectedMessageId) setSelectedMessageId("");
      return;
    }
    if (selectedMessageId && !visibleMessages.some((message) => message.id === selectedMessageId)) {
      setSelectedMessageId(visibleMessages[0]?.id ?? "");
    }
  }, [selectedMessageId, useApi, visibleMessages]);

  useEffect(() => {
    if (searchInput === search) return;
    if (!searchInput.trim()) {
      setSearch("");
      return;
    }
    const timer = window.setTimeout(() => setSearch(searchInput), SEARCH_DEBOUNCE_MS);
    return () => window.clearTimeout(timer);
  }, [search, searchInput]);

  useEffect(() => {
    composeDirtyRef.current = composeDirty;
  }, [composeDirty]);

  useEffect(() => {
    if (!composeOpen) return;
    const marker = { ...(window.history.state ?? {}), joomailCompose: true };
    window.history.pushState(marker, "", window.location.href);

    function handlePopState() {
      if (!closeCompose()) {
        window.history.pushState(marker, "", window.location.href);
      }
    }

    window.addEventListener("popstate", handlePopState);
    return () => window.removeEventListener("popstate", handlePopState);
  }, [composeOpen]);

  useEffect(() => {
    if (initialQaAppliedRef.current) return;
    initialQaAppliedRef.current = true;

    const qaParam = new URLSearchParams(window.location.search).get("qa");
    if (isQaState(qaParam)) {
      applyQaState(qaParam);
    }
  }, []);

  useEffect(() => {
    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape" && settingsOpen) {
        event.preventDefault();
        setSettingsOpen(false);
        return;
      }

      if (event.key === "Escape" && composeOpen) {
        event.preventDefault();
        closeCompose();
        return;
      }

      if (isTypingTarget(event.target)) return;
      if (event.metaKey || event.ctrlKey || event.altKey) return;

      if (event.key === "/") {
        event.preventDefault();
        searchInputRef.current?.focus();
        return;
      }

      if (event.key === "c") {
        event.preventDefault();
        openCompose();
        return;
      }

      if (event.key === "Escape") {
        if (checkedIds.size > 0) {
          event.preventDefault();
          clearChecked();
          return;
        }
        if (search) {
          event.preventDefault();
          handleSearch("");
        }
        return;
      }

      if (event.key === "ArrowDown" || event.key === "j" || event.key === "ArrowUp" || event.key === "k") {
        if (!visibleMessages.length) return;
        event.preventDefault();
        const movingDown = event.key === "ArrowDown" || event.key === "j";
        const currentIndex = visibleMessages.findIndex((message) => message.id === selectedMessageId);
        const fallbackIndex = movingDown ? -1 : visibleMessages.length;
        const nextIndex = movingDown
          ? Math.min(visibleMessages.length - 1, (currentIndex === -1 ? fallbackIndex : currentIndex) + 1)
          : Math.max(0, (currentIndex === -1 ? fallbackIndex : currentIndex) - 1);
        setSelectedMessageId(visibleMessages[nextIndex].id);
        return;
      }

      if (event.key === "Enter") {
        const focusedMessageId = getFocusedMessageId(event.target);
        const nextSelectedId = focusedMessageId || selectedMessageId || visibleMessages[0]?.id;
        if (!nextSelectedId) return;
        event.preventDefault();
        setSelectedMessageId(nextSelectedId);
        return;
      }

      if (event.key === "x") {
        const targetId = selectedMessageId || visibleMessages[0]?.id;
        if (!targetId) return;
        event.preventDefault();
        toggleChecked(targetId);
        return;
      }

      if (event.key === "r") {
        const targetId = selectedMessageId || visibleMessages[0]?.id;
        if (!targetId) return;
        event.preventDefault();
        setSelectedMessageId(targetId);
        openReply("reply", targetId);
      }
    }

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [checkedIds.size, composeOpen, search, selectedMessageId, settingsOpen, visibleMessages]);

  function applyQaState(nextState: QaState) {
    const accountMessages = mockMessages.filter((message) => message.accountId === accountId);
    const inboxMessages = accountMessages.filter((message) => message.mailboxId === "inbox");
    const firstInboxId = inboxMessages[0]?.id ?? "";
    const searchMessages = accountMessages.filter((message) =>
      [message.sender, message.subject, message.snippet, ...message.body].some((field) => field.toLowerCase().includes(SEARCH_QA_QUERY.toLowerCase())),
    );
    const longOverflowId = accountMessages.find((message) => message.id === "qa-long-overflow")?.id ?? firstInboxId;
    const manyAttachmentsId = accountMessages.find((message) => message.id === "qa-many-attachments")?.id ?? firstInboxId;
    const quotedId = accountMessages.find((message) => message.id === "qa-quoted")?.id ?? firstInboxId;

    setForceEmptyList(nextState === "empty");
    setForceMobileReadingId("");
    setForceMobileFolderOpen(false);
    setForceQuotedOpen(false);
    setForceComposeCcBcc(false);
    setMode(nextState === "loading" || nextState === "error" ? nextState : "normal");
    setMailboxId("inbox");
    setSearchInput("");
    setSearch("");
    setSearchScope("mailbox");
    setCheckedIds(new Set());
    setLastCheckedId("");
    setReplyMessageId(null);
    setComposeMode("compose");
    setComposeOpen(nextState === "compose" || nextState === "compose-cc-bcc");
    setSelectedMessageId(firstInboxId);
    setShowRemoteImagesByDefault(nextState === "remote-images-shown");
	setSendWarning(nextState === "send-warning" ? SENT_COPY_WARNING : "");
	if (nextState === "account-unavailable") {
		setAccounts((current) => current.map((account, index) => index === 0 ? { ...account, status: "unavailable" } : account));
	}

    if (nextState === "empty" || nextState === "empty-reading" || nextState === "search-empty") {
      setSelectedMessageId("");
    }

    if (nextState === "search") {
      setSearchInput(SEARCH_QA_QUERY);
      setSearch(SEARCH_QA_QUERY);
      setSelectedMessageId(searchMessages[0]?.id ?? "");
    }

    if (nextState === "search-account") {
      setSearchInput(SEARCH_QA_QUERY);
      setSearch(SEARCH_QA_QUERY);
      setSearchScope("account");
      setSelectedMessageId(searchMessages[0]?.id ?? "");
    }

    if (nextState === "search-empty") {
      setSearchInput(SEARCH_EMPTY_QA_QUERY);
      setSearch(SEARCH_EMPTY_QA_QUERY);
    }

    if (nextState === "multiselect") {
      setCheckedIds(new Set(inboxMessages.slice(0, 3).map((message) => message.id)));
      setLastCheckedId(inboxMessages[2]?.id ?? "");
    }

    if (nextState === "remote-images-shown") {
      setSelectedMessageId(firstInboxId);
    }

    if (nextState === "quoted-expanded") {
      setSelectedMessageId(quotedId);
      setForceQuotedOpen(true);
    }

    if (nextState === "long-overflow") {
      setSelectedMessageId(longOverflowId);
    }

    if (nextState === "many-attachments") {
      setSelectedMessageId(manyAttachmentsId);
    }

	if (nextState === "starred") {
		setMailboxId("starred");
		setSelectedMessageId(accountMessages.find((message) => message.flagged)?.id ?? "");
	}

    if (nextState === "empty-custom-folder") {
      setMailboxId("clients");
      setSelectedMessageId("");
    }

    if (nextState === "nested-tree") {
      setMailboxId("clients");
      setSelectedMessageId("");
      setForceMobileFolderOpen(true);
    }

    if (nextState === "mobile-reading-attachments") {
      setSelectedMessageId(manyAttachmentsId);
      setForceMobileReadingId(manyAttachmentsId);
    }

    if (nextState === "compose-cc-bcc") {
      setForceComposeCcBcc(true);
    }
  }

  function selectAccount(nextAccountId: string) {
    const restored = mailStateByAccount[nextAccountId];
    setAccountId(nextAccountId);
    setMailboxId(restored?.mailboxId ?? "inbox");
    setSelectedMessageId(useApi ? "" : restored?.messageId ?? "");
    setForceEmptyList(false);
    setCheckedIds(new Set());
    setLastCheckedId("");
    if (useApi) {
      setSearchInput("");
      setSearch("");
      return;
    }
    triggerLoading();
  }

  function selectMailbox(nextMailboxId: string) {
    const nextMailbox = flatMailboxes.find((mailbox) => mailbox.id === nextMailboxId);
    if (nextMailbox?.selectable === false) return;
    setMailboxId(nextMailboxId);
    setSearchInput("");
    setSearch("");
    setForceEmptyList(false);
    setCheckedIds(new Set());
    setLastCheckedId("");
    if (useApi) {
      setSelectedMessageId("");
      return;
    }
    setMode(nextMailboxId === "spam" ? "error" : "loading");
    window.setTimeout(() => setMode(nextMailboxId === "spam" ? "error" : "normal"), 420);
  }

  function triggerLoading() {
    setForceEmptyList(false);
    setMode("loading");
    window.setTimeout(() => setMode("normal"), 420);
  }

  function retry() {
    if (useApi) {
      setReloadToken((value) => value + 1);
      return;
    }
    setForceEmptyList(false);
    triggerLoading();
    setMailboxId("inbox");
  }

  function handleSearch(nextSearch: string) {
    setForceEmptyList(false);
    setSearchInput(nextSearch);
    if (!nextSearch.trim()) {
      setSearch("");
      setSearchScope("mailbox");
      clearChecked();
    }
  }

  function handleSearchScope(nextScope: SearchScope) {
    setForceEmptyList(false);
    setSearchScope(nextScope);
  }

  function openCompose() {
    setReplyMessageId(null);
    setComposeMode("compose");
    setComposeDirty(false);
    composeDirtyRef.current = false;
    setComposeOpen(true);
  }

  function openReply(mode: Exclude<ComposeMode, "compose"> = "reply", messageId = selectedMessageId) {
    if (!messageId) return;
    setReplyMessageId(messageId);
    setComposeMode(mode);
    setComposeDirty(false);
    composeDirtyRef.current = false;
    setComposeOpen(true);
  }

  function closeCompose(options?: { force?: boolean }) {
    if (!options?.force && composeDirtyRef.current && !window.confirm("작성 중인 내용을 버리고 닫을까요?")) {
      return false;
    }
    setComposeOpen(false);
    setReplyMessageId(null);
    setComposeMode("compose");
    setComposeDirty(false);
    composeDirtyRef.current = false;
    return true;
  }

  function updateComposeDirty(dirty: boolean) {
    composeDirtyRef.current = dirty;
    setComposeDirty(dirty);
  }

  async function sendDraft(draft: ComposeDraft) {
    try {
      const fromAccount = displayAccounts.find((account) => account.id === draft.fromAccountId);
      const configuredFromName = fromAccount ? accountNames[fromAccount.email]?.trim() : "";
      const nextDraft = { ...draft, fromName: configuredFromName };
      let response: SendResponse;
      if (draft.attachments?.length) {
        const formData = new FormData();
        formData.set("fromAccountId", nextDraft.fromAccountId);
        formData.set("fromName", nextDraft.fromName ?? "");
        formData.set("to", JSON.stringify(nextDraft.to));
        formData.set("cc", JSON.stringify(nextDraft.cc));
        formData.set("bcc", JSON.stringify(nextDraft.bcc));
        formData.set("subject", nextDraft.subject);
        formData.set("textBody", nextDraft.textBody);
        nextDraft.attachments?.forEach((file) => formData.append("attachments", file, file.name));
        response = await apiJSON<SendResponse>("/api/send", {
          method: "POST",
          body: formData,
        });
      } else {
        response = await apiJSON<SendResponse>("/api/send", {
          method: "POST",
          body: JSON.stringify(nextDraft),
        });
      }
      setSendWarning(response.sentCopyStored ? "" : SENT_COPY_WARNING);
      setReloadToken((value) => value + 1);
    } catch (error) {
      if (isUnauthorized(error)) onSessionExpired?.();
      throw error;
    }
  }

  async function saveDraft(draft: ComposeDraft) {
    try {
      const fromAccount = displayAccounts.find((account) => account.id === draft.fromAccountId);
      const configuredFromName = fromAccount ? accountNames[fromAccount.email]?.trim() : "";
      const nextDraft = { ...draft, fromName: configuredFromName };
      if (draft.attachments?.length) {
        const formData = new FormData();
        formData.set("fromAccountId", nextDraft.fromAccountId);
        formData.set("fromName", nextDraft.fromName ?? "");
        formData.set("to", JSON.stringify(nextDraft.to));
        formData.set("cc", JSON.stringify(nextDraft.cc));
        formData.set("bcc", JSON.stringify(nextDraft.bcc));
        formData.set("subject", nextDraft.subject);
        formData.set("textBody", nextDraft.textBody);
        nextDraft.attachments?.forEach((file) => formData.append("attachments", file, file.name));
        await apiJSON("/api/drafts", {
          method: "POST",
          body: formData,
        });
      } else {
        await apiJSON("/api/drafts", {
          method: "POST",
          body: JSON.stringify(nextDraft),
        });
      }
      setReloadToken((value) => value + 1);
    } catch (error) {
      if (isUnauthorized(error)) onSessionExpired?.();
      throw error;
    }
  }

  const loadRules = useCallback(
    async (targetAccountId: string) => {
      try {
        const body = await apiJSON<{ rules: MailRule[] }>(`/api/accounts/${encodeURIComponent(targetAccountId)}/rules`);
        return body.rules;
      } catch (error) {
        if (isUnauthorized(error)) onSessionExpired?.();
        if (error instanceof ApiError && error.status === 503) {
          throw new Error("rules unavailable");
        }
        throw error;
      }
    },
    [onSessionExpired],
  );

  const saveRules = useCallback(
    async (targetAccountId: string, rules: MailRule[]) => {
      try {
        const body = await apiJSON<{ rules: MailRule[] }>(`/api/accounts/${encodeURIComponent(targetAccountId)}/rules`, {
          method: "PUT",
          body: JSON.stringify({ rules }),
        });
        return body.rules;
      } catch (error) {
        if (isUnauthorized(error)) onSessionExpired?.();
        if (error instanceof ApiError && error.status === 503) {
          throw new Error("rules unavailable");
        }
        if (error instanceof ApiError && error.status === 409) {
          throw new Error("rules conflict");
        }
        throw error;
      }
    },
    [onSessionExpired],
  );

  function updateAccountName(email: string, value: string) {
    setAccountNames((current) => {
      const next = { ...current };
      const trimmed = value.trim();
      if (trimmed) {
        next[email] = value;
      } else {
        delete next[email];
      }
      return next;
    });
  }

  async function logout() {
    try {
      await apiJSON("/api/logout", { method: "POST" });
    } catch {
      // The local session should still be dropped if the server already expired it.
    } finally {
      onSessionExpired?.();
    }
  }

  async function refreshAccounts(nextAccount?: Account) {
    if (!useApi) {
      if (nextAccount) setAccountId(nextAccount.id);
      setAddAccountOpen(false);
      return;
    }
    try {
      const body = await apiJSON<{ accounts: Account[] }>("/api/accounts");
      setAccounts(body.accounts);
      const nextID = nextAccount?.id ?? body.accounts[0]?.id;
      if (nextID) {
        setAccountId(nextID);
      }
      setMailboxId("inbox");
      setSelectedMessageId("");
      setSelectedMessageDetail(undefined);
      setApiMessages([]);
      setSearchInput("");
      setSearch("");
      setCheckedIds(new Set());
      setLastCheckedId("");
      setReloadToken((token) => token + 1);
      setAddAccountOpen(false);
    } catch (error) {
      if (isUnauthorized(error)) onSessionExpired?.();
    }
  }

  function selectMessage(id: string, options?: { shift: boolean; toggle: boolean }) {
    if (options?.shift) {
      selectMessageRange(id);
      return;
    }
    if (options?.toggle) {
      toggleChecked(id);
      setSelectedMessageId(id);
      return;
    }
    setSelectedMessageId(id);
  }

  function selectMessageRange(id: string) {
    const anchorId = lastCheckedId || selectedMessageId || visibleMessages[0]?.id;
    const anchorIndex = visibleMessages.findIndex((message) => message.id === anchorId);
    const targetIndex = visibleMessages.findIndex((message) => message.id === id);
    if (anchorIndex === -1 || targetIndex === -1) {
      toggleChecked(id);
      setSelectedMessageId(id);
      return;
    }
    const from = Math.min(anchorIndex, targetIndex);
    const to = Math.max(anchorIndex, targetIndex);
    setCheckedIds((current) => new Set([...current, ...visibleMessages.slice(from, to + 1).map((message) => message.id)]));
    setLastCheckedId(id);
    setSelectedMessageId(id);
  }

  function toggleChecked(id: string) {
    setLastCheckedId(id);
    setCheckedIds((current) => {
      const next = new Set(current);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  function toggleAllChecked() {
    setCheckedIds((current) => {
      const visibleIds = visibleMessages.map((message) => message.id);
      if (visibleIds.length === 0) return current;
      const allChecked = visibleIds.every((id) => current.has(id));
      if (allChecked) {
        const next = new Set(current);
        visibleIds.forEach((id) => next.delete(id));
        setLastCheckedId("");
        return next;
      }
      setLastCheckedId(visibleIds[visibleIds.length - 1] ?? "");
      return new Set([...current, ...visibleIds]);
    });
  }

  function clearChecked() {
    setCheckedIds(new Set());
    setLastCheckedId("");
  }

  async function toggleFlagged(message: Message) {
    const nextFlagged = !message.flagged;
    updateMessageFlagged(message.id, nextFlagged);
    if (!useApi) return;
    try {
      await apiJSON(`/api/messages/${encodeURIComponent(message.id)}/flagged`, {
        method: "PATCH",
        body: JSON.stringify({ flagged: nextFlagged }),
      });
    } catch (error) {
      updateMessageFlagged(message.id, Boolean(message.flagged));
      if (isUnauthorized(error)) onSessionExpired?.();
    }
  }

  function updateMessageFlagged(id: string, flagged: boolean) {
    setApiMessages((current) => current.flatMap((item) => {
      if (item.id !== id) return [item];
      if (mailboxId === "starred" && !flagged) return [];
      return [{ ...item, flagged }];
    }));
    setSelectedMessageDetail((current) => (current?.id === id ? { ...current, flagged } : current));
  }

  async function markUnread(message: Message) {
    updateMessageUnread(message.id, true);
    if (!useApi) return;
    try {
      await apiJSON(`/api/messages/${encodeURIComponent(message.id)}/seen`, {
        method: "PATCH",
        body: JSON.stringify({ seen: false }),
      });
    } catch (error) {
      updateMessageUnread(message.id, false);
      if (isUnauthorized(error)) onSessionExpired?.();
      throw error;
    }
  }

  function updateMessageUnread(id: string, unread: boolean) {
    setApiMessages((current) => current.map((item) => (item.id === id ? { ...item, unread } : item)));
    setSelectedMessageDetail((current) => (current?.id === id ? { ...current, unread } : current));
  }

  async function moveMessage(message: Message, targetMailboxId: string) {
    if (!targetMailboxId) throw new Error("missing target mailbox");
    if (!useApi) {
      setSelectedMessageId("");
      return;
    }
    const previousMessages = apiMessages;
    const previousDetail = selectedMessageDetail;
    setApiMessages((current) => current.filter((item) => item.id !== message.id));
    setSelectedMessageDetail(undefined);
    setSelectedMessageId("");
    try {
      await apiJSON(`/api/messages/${encodeURIComponent(message.id)}/move`, {
        method: "POST",
        body: JSON.stringify({ mailboxId: targetMailboxId }),
      });
    } catch (error) {
      setApiMessages(previousMessages);
      setSelectedMessageDetail(previousDetail);
      setSelectedMessageId(message.id);
      if (isUnauthorized(error)) onSessionExpired?.();
      throw error;
    }
  }

  function moveToKind(message: Message, kind: Mailbox["kind"]) {
    const mailbox = flatMailboxes.find((item) => item.kind === kind && item.selectable !== false);
    if (!mailbox) throw new Error(`missing ${kind} mailbox`);
    return moveMessage(message, mailbox.id);
  }

  function bulkMoveToKind(kind: Mailbox["kind"]) {
    const mailbox = flatMailboxes.find((item) => item.kind === kind && item.selectable !== false);
    if (!mailbox) throw new Error(`missing ${kind} mailbox`);
    return bulkMoveMessages(mailbox.id);
  }

  async function bulkMoveMessages(targetMailboxId: string) {
    const selectedMessages = visibleMessages.filter((message) => checkedIds.has(message.id));
    if (!selectedMessages.length) return;
    if (!useApi) {
      clearChecked();
      setSelectedMessageId("");
      return;
    }
    const movedIds = new Set<string>();
    let unauthorized = false;
    for (const message of selectedMessages) {
      try {
        await apiJSON(`/api/messages/${encodeURIComponent(message.id)}/move`, {
          method: "POST",
          body: JSON.stringify({ mailboxId: targetMailboxId }),
        });
        movedIds.add(message.id);
      } catch (error) {
        if (isUnauthorized(error)) unauthorized = true;
      }
    }
    if (movedIds.size) {
      setApiMessages((current) => current.filter((message) => !movedIds.has(message.id)));
      setSelectedMessageDetail((current) => (current && movedIds.has(current.id) ? undefined : current));
      setSelectedMessageId((current) => (movedIds.has(current) ? "" : current));
      setCheckedIds((current) => {
        const next = new Set(current);
        movedIds.forEach((id) => next.delete(id));
        return next;
      });
    }
    if (unauthorized) {
      onSessionExpired?.();
      throw new Error("session expired");
    }
    if (movedIds.size !== selectedMessages.length) {
      throw new Error("some messages failed to move");
    }
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
      <MobileInbox
        account={selectedAccount}
        messages={visibleMessages}
        selectedMessage={selectedMessage}
        selectedId={selectedMessageId}
        selectedMailboxId={mailboxId}
        checkedIds={checkedIds}
        search={search}
        searchInput={searchInput}
        searchScope={searchScope}
        mode={mode}
        scrollKey={`${accountId}:${mailboxId}:${searchScope}:${search}`}
        showRemoteImagesByDefault={showRemoteImagesByDefault}
        forceReadingId={forceMobileReadingId}
        folderMenuOpenByDefault={forceMobileFolderOpen}
        onRetry={retry}
        onCompose={openCompose}
        onReply={(messageId) => openReply("reply", messageId)}
        onSearch={handleSearch}
        onSearchScopeChange={handleSearchScope}
        onSelectMessage={setSelectedMessageId}
        onSelectMailbox={selectMailbox}
        onToggleChecked={toggleChecked}
        onToggleFlagged={toggleFlagged}
        onClearChecked={clearChecked}
        onBulkArchive={() => bulkMoveToKind("archive")}
        onBulkTrash={() => bulkMoveToKind("trash")}
        onLogout={useApi ? logout : undefined}
      />
      {sendWarning ? (
        <div className="fixed bottom-4 left-4 right-4 z-50 flex min-h-11 items-center rounded-lg border border-[#ead8a6] bg-[#fff9e8] px-3 py-2 text-[12.5px] text-[#6f5310] shadow-compose md:left-auto md:right-5 md:w-[430px]" role="status">
          <span className="min-w-0 flex-1">{sendWarning}</span>
          <button className="ml-3 flex h-7 w-7 shrink-0 items-center justify-center rounded-md hover:bg-black/5" aria-label="전송 알림 닫기" onClick={() => setSendWarning("")} type="button">
            <Icon name="close" className="h-3.5 w-3.5" />
          </button>
        </div>
      ) : null}
      <div className="hidden h-screen flex-col md:flex">
        <Toolbar search={searchInput} searchInputRef={searchInputRef} onSearch={handleSearch} onRefresh={retry} onSettings={() => setSettingsOpen(true)} />
        <div className="min-h-0 flex flex-1">
          <Sidebar
            accounts={displayAccounts}
            selectedAccount={selectedAccount}
            selectedMailboxId={mailboxId}
            onSelectAccount={selectAccount}
            onSelectMailbox={selectMailbox}
            onCompose={openCompose}
            onAddAccount={useApi ? () => { setReauthEmail(""); setAddAccountOpen(true); } : undefined}
            onReauthenticate={useApi ? (email) => { setReauthEmail(email); setAddAccountOpen(true); } : undefined}
            onRetryAccount={useApi ? () => void refreshAccounts() : undefined}
            onLogout={useApi ? logout : undefined}
          />
          <MessageList
            title={selectedMailbox?.label ?? "받은편지함"}
            unreadCount={visibleMessages.length === 0 && mode === "normal" && !search ? 0 : selectedAccount.unread}
            messages={visibleMessages}
            mailboxes={flatMailboxes}
            selectedId={selectedMessage?.id}
            checkedIds={checkedIds}
            search={search}
            searchScope={searchScope}
            mode={mode}
            scrollKey={`${accountId}:${mailboxId}:${searchScope}:${search}`}
            onRetry={retry}
            onSearchScopeChange={handleSearchScope}
            onSelectMessage={selectMessage}
            onToggleAllChecked={toggleAllChecked}
            onToggleChecked={toggleChecked}
            onToggleFlagged={toggleFlagged}
            onArchive={(message) => moveToKind(message, "archive")}
            onTrash={(message) => moveToKind(message, "trash")}
            onMarkUnread={markUnread}
            onBulkArchive={() => bulkMoveToKind("archive")}
            onBulkTrash={() => bulkMoveToKind("trash")}
            onBulkMove={bulkMoveMessages}
            onClearChecked={clearChecked}
          />
          <div className="hidden w-1 cursor-col-resize bg-white hover:bg-selected md:block" onPointerDown={startResize} aria-label="리스트 폭 조절" />
          <ReadingPane
            message={selectedMessage}
            mode={mode}
            mailboxes={flatMailboxes}
            showRemoteImagesByDefault={showRemoteImagesByDefault}
            quotedOpenByDefault={forceQuotedOpen}
            onRetry={retry}
            onReply={() => openReply("reply")}
            onReplyAll={() => openReply("replyAll")}
            onForward={() => openReply("forward")}
            onToggleFlagged={toggleFlagged}
            onArchive={(message) => moveToKind(message, "archive")}
            onTrash={(message) => moveToKind(message, "trash")}
            onMove={moveMessage}
            onMarkUnread={markUnread}
          />
        </div>
      </div>
      {settingsOpen ? (
        <SettingsPanel
          account={selectedAccount}
          displayName={accountNames[selectedAccount.email] ?? ""}
          onDisplayNameChange={(value) => updateAccountName(selectedAccount.email, value)}
          remoteImagesEnabled={showRemoteImagesByDefault}
          onRemoteImagesChange={setShowRemoteImagesByDefault}
          mailboxes={flatMailboxes}
          onLoadRules={useApi ? () => loadRules(selectedAccount.id) : undefined}
          onSaveRules={useApi ? (rules) => saveRules(selectedAccount.id, rules) : undefined}
          onLogout={useApi ? logout : undefined}
          onClose={() => setSettingsOpen(false)}
        />
      ) : null}
      {addAccountOpen ? <AddAccountModal initialEmail={reauthEmail} onClose={() => setAddAccountOpen(false)} onAdded={refreshAccounts} /> : null}
      {composeOpen ? <ComposePanel accounts={displayAccounts} account={selectedAccount} mode={composeMode} message={composeMessage} ccBccOpenByDefault={forceComposeCcBcc} onDirtyChange={updateComposeDirty} onClose={closeCompose} onSend={useApi ? sendDraft : undefined} onSaveDraft={useApi ? saveDraft : undefined} /> : null}
      {import.meta.env.DEV && !composeOpen ? <DevStateSwitcher states={QA_STATES} onApply={applyQaState} /> : null}
    </div>
  );
}

function isQaState(value: string | null): value is QaState {
  return QA_STATES.includes(value as QaState);
}

function getFocusedMessageId(target: EventTarget | null) {
  if (!(target instanceof Element)) return "";
  return target.closest<HTMLElement>("[data-message-id]")?.dataset.messageId ?? "";
}

function isTypingTarget(target: EventTarget | null) {
  if (!(target instanceof HTMLElement)) return false;
  const tagName = target.tagName.toLowerCase();
  return tagName === "input" || tagName === "textarea" || tagName === "select" || target.isContentEditable;
}

type ApiMessage = Omit<Message, "body"> & {
  textBody?: string[];
  body?: string[];
};

type SendResponse = {
  status: "sent";
  sentCopyStored: boolean;
};

class ApiError extends Error {
  status: number;

  constructor(status: number) {
    super(`API request failed with ${status}`);
    this.status = status;
  }
}

async function apiJSON<T = unknown>(path: string, init: RequestInit = {}) {
  const headers = new Headers(init.headers);
  if (!(init.body instanceof FormData) && !headers.has("Content-Type")) headers.set("Content-Type", "application/json");
  const response = await fetch(path, {
    ...init,
    headers,
    credentials: "include",
  });
  if (!response.ok) throw new ApiError(response.status);
  return (await response.json()) as T;
}

function isUnauthorized(error: unknown) {
  return error instanceof ApiError && error.status === 401;
}

function summaryToMessage(message: ApiMessage): Message {
  return normalizeMessage({ ...message, body: message.body ?? [] });
}

function normalizeMessage(message: ApiMessage): Message {
  return {
    ...message,
    body: message.textBody ?? message.body ?? [],
  };
}

type AccountMailState = {
  mailboxId: string;
  messageId: string;
};

type StoredMailState = {
  activeAccountId: string;
  searchScope: SearchScope;
  byAccount: Record<string, AccountMailState>;
};

function messageSummariesPath(accountId: string, mailboxId: string, search: string, searchScope: SearchScope) {
  const params = new URLSearchParams();
  if (search.trim()) {
    params.set("q", search.trim());
    params.set("scope", searchScope);
  }
  const suffix = params.toString() ? `?${params.toString()}` : "";
  return `/api/accounts/${encodeURIComponent(accountId)}/mailboxes/${encodeURIComponent(mailboxId)}/messages${suffix}`;
}

function loadMailState(accounts: Account[]): StoredMailState {
  const fallbackAccountId = accounts[0]?.id ?? "";
  const fallback = { activeAccountId: fallbackAccountId, searchScope: "mailbox" as SearchScope, byAccount: {} };
  try {
    const raw = localStorage.getItem(MAIL_STATE_KEY);
    if (!raw) return fallback;
    const parsed = JSON.parse(raw) as Partial<StoredMailState>;
    const activeAccountId = accounts.some((account) => account.id === parsed.activeAccountId) ? parsed.activeAccountId ?? fallbackAccountId : fallbackAccountId;
    return {
      activeAccountId,
      searchScope: parsed.searchScope === "account" ? "account" : "mailbox",
      byAccount: typeof parsed.byAccount === "object" && parsed.byAccount ? parsed.byAccount : {},
    };
  } catch {
    return fallback;
  }
}

function flattenMailboxes(mailboxes: Mailbox[]): Mailbox[] {
  return mailboxes.flatMap((mailbox) => [mailbox, ...flattenMailboxes(mailbox.children ?? [])]);
}

function loadAccountNames(): Record<string, string> {
  try {
    const value = localStorage.getItem(ACCOUNT_NAMES_KEY);
    if (!value) return {};
    const parsed = JSON.parse(value);
    return parsed && typeof parsed === "object" ? parsed : {};
  } catch {
    return {};
  }
}

function withDisplayName(account: Account, displayName?: string): Account {
  const name = displayName?.trim();
  if (!name) return account;
  return {
    ...account,
    label: name,
    initials: firstInitial(name),
  };
}

function firstInitial(value: string) {
  return value.trim().slice(0, 1).toUpperCase() || "";
}
