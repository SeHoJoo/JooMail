import { describe, expect, it } from "vitest";
import { composeInitialState } from "./ComposePanel";
import type { Message } from "../types";

const message: Message = {
  id: "m1",
  accountId: "jooseho@good-night.co.kr",
  mailboxId: "inbox",
  sender: "Alice",
  senderEmail: "alice@example.com",
  initials: "AL",
  subject: "Project update",
  snippet: "Hello",
  time: "09:14",
  fullDate: "Fri, 3 Jul 2026 09:14:00 +0900",
  unread: true,
  body: ["Hello"],
  headers: {
    to: ["Jooseho <jooseho@good-night.co.kr>", "Bob <bob@example.com>"],
    cc: ["jooseho@good-night.co.kr", "Carol <carol@example.com>"],
  },
};

describe("composeInitialState", () => {
  it("filters current account from reply-all recipients", () => {
    const state = composeInitialState("replyAll", message, "jooseho@good-night.co.kr");

    expect(state.to).toBe("alice@example.com, bob@example.com");
    expect(state.cc).toBe("carol@example.com");
  });

  it("keeps forward body text-only and does not attach original attachments", () => {
    const state = composeInitialState("forward", message, "jooseho@good-night.co.kr");

    expect(state.subject).toBe("Fwd: Project update");
    expect(state.body).toContain("---------- Forwarded message ---------");
  });
});
