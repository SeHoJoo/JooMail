import type React from "react";
import type { Attachment, Message } from "../types";
import { Icon } from "./Icon";

const URL_PATTERN = /https?:\/\/[^\s<>"']+/gi;

export const htmlContentClassName =
  "overflow-x-auto text-text [&_a]:text-accent [&_a]:underline [&_blockquote]:border-l-2 [&_blockquote]:border-line [&_blockquote]:pl-3 [&_blockquote]:text-muted [&_font]:font-inherit [&_h1]:mb-4 [&_h1]:text-xl [&_h1]:font-bold [&_h2]:mb-3 [&_h2]:text-lg [&_h2]:font-bold [&_h3]:mb-3 [&_h3]:text-base [&_h3]:font-bold [&_img:not([src])]:hidden [&_img]:h-auto [&_img]:max-w-full [&_li]:mb-1 [&_ol]:mb-5 [&_ol]:list-decimal [&_ol]:pl-5 [&_p]:mb-5 [&_pre]:overflow-x-auto [&_pre]:whitespace-pre-wrap [&_pre]:rounded [&_pre]:bg-[#f7f8f9] [&_pre]:p-3 [&_table]:mb-5 [&_table]:max-w-full [&_table]:border-collapse [&_td]:border-line [&_td]:align-top [&_th]:border-line [&_th]:align-top [&_ul]:mb-5 [&_ul]:list-disc [&_ul]:pl-5";

export function renderTextWithLinks(text: string) {
  const parts: React.ReactNode[] = [];
  let cursor = 0;
  for (const match of text.matchAll(URL_PATTERN)) {
    const index = match.index ?? 0;
    const rawURL = trimTrailingPunctuation(match[0]);
    if (index > cursor) parts.push(text.slice(cursor, index));
    parts.push(
      <a key={`${index}-${rawURL}`} className="text-accent underline" href={rawURL} rel="noreferrer" target="_blank">
        {rawURL}
      </a>,
    );
    cursor = index + rawURL.length;
    if (rawURL.length < match[0].length) parts.push(match[0].slice(rawURL.length));
  }
  if (cursor < text.length) parts.push(text.slice(cursor));
  return parts.length ? <>{parts}</> : text;
}

export function attachmentURL(message: Message, attachment: Attachment) {
  if (!attachment.id) return undefined;
  return `/api/messages/${encodeURIComponent(message.id)}/attachments/${encodeURIComponent(attachment.id)}`;
}

export function AttachmentIcon({ attachment }: { attachment: Attachment }) {
  if (attachment.type === "image") return <Icon name="image" className="h-4 w-4" />;
  if (attachment.type === "pdf") return <Icon name="mail" className="h-4 w-4" />;
  return <Icon name="paperclip" className="h-4 w-4" />;
}

export function trimTrailingPunctuation(value: string) {
  return value.replace(/[),.;!?]+$/g, "");
}
