export type MailboxKind =
  | "inbox"
  | "starred"
  | "sent"
  | "drafts"
  | "archive"
  | "spam"
  | "trash"
  | "folder";

export type Mailbox = {
  id: string;
  label: string;
  kind: MailboxKind;
  selectable?: boolean;
  unread?: number;
  children?: Mailbox[];
};

export type Account = {
  id: string;
  email: string;
  label: string;
  initials: string;
  unread: number;
  storage: string;
  mailboxes: Mailbox[];
};

export type Attachment = {
  id?: string;
  name: string;
  size: string;
  type: "pdf" | "image" | "file";
};

export type MessageHeaders = {
  from?: string;
  to?: string[];
  cc?: string[];
  date?: string;
  subject?: string;
  messageId?: string;
  inReplyTo?: string;
  references?: string[];
};

export type Message = {
  id: string;
  accountId: string;
  mailboxId: string;
  threadId?: string;
  sender: string;
  senderEmail: string;
  initials: string;
  subject: string;
  snippet: string;
  time: string;
  fullDate: string;
  unread: boolean;
  selected?: boolean;
  flagged?: boolean;
  hasAttachment?: boolean;
  remoteImagesBlocked?: boolean;
  body: string[];
  htmlBody?: string;
  bullets?: string[];
  link?: string;
  attachments?: Attachment[];
  headers?: MessageHeaders;
};

export type ComposeMode = "compose" | "reply" | "replyAll" | "forward";

export type SearchScope = "mailbox" | "account";

export type RuleConditionField = "senderEmail" | "senderDomain" | "subject";

export type RuleMatch = "contains" | "equals";

export type RuleActionType = "move" | "moveSpam" | "moveTrash";

export type MailRule = {
  name?: string;
  condition: {
    field: RuleConditionField;
    match: RuleMatch;
    value: string;
  };
  action: {
    type: RuleActionType;
    mailboxId?: string;
  };
};

export type ComposeDraft = {
  fromAccountId: string;
  fromName?: string;
  to: string[];
  cc: string[];
  bcc: string[];
  subject: string;
  textBody: string;
  attachments?: File[];
};

export type MockMode = "normal" | "loading" | "error";
