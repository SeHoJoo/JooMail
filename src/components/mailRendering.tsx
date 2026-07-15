import { useMemo, useRef, useState, type ReactNode } from "react";
import type { Attachment, Message } from "../types";
import { Icon } from "./Icon";

const URL_PATTERN = /https?:\/\/[^\s<>"']+/gi;

export function MailHTMLBody({ html, className = "" }: { html: string; className?: string }) {
  const iframeRef = useRef<HTMLIFrameElement>(null);
  const [height, setHeight] = useState(120);
  const srcDoc = useMemo(() => mailHTMLSrcDoc(html), [html]);

  function resizeFrame() {
    const doc = iframeRef.current?.contentDocument;
    if (!doc) return;
    const nextHeight = Math.max(80, doc.documentElement.scrollHeight, doc.body?.scrollHeight ?? 0);
    setHeight(nextHeight);
  }

  function handleLoad() {
    resizeFrame();
    const images = iframeRef.current?.contentDocument?.images;
    if (images) {
      Array.from(images).forEach((image) => image.addEventListener("load", resizeFrame, { once: true }));
    }
    window.setTimeout(resizeFrame, 50);
    window.setTimeout(resizeFrame, 300);
  }

  return (
    <iframe
      ref={iframeRef}
      className={`w-full border-0 bg-transparent ${className}`.trim()}
      srcDoc={srcDoc}
      sandbox="allow-popups allow-popups-to-escape-sandbox allow-same-origin"
      title="메일 HTML 본문"
      style={{ height }}
      onLoad={handleLoad}
    />
  );
}

export function mailHTMLSrcDoc(html: string) {
  return `<!doctype html><html><head><base target="_blank"><style>${mailHTMLFrameCSS}</style></head><body>${stripExecutableEmailMarkup(html)}</body></html>`;
}

function stripExecutableEmailMarkup(html: string) {
  if (typeof DOMParser === "undefined") {
    return html
      .replace(/<script\b[^>]*>[\s\S]*?<\/script\s*>/gi, "")
      .replace(/<script\b[^>]*\/?\s*>/gi, "")
      .replace(/\son[a-z]+\s*=\s*(?:"[^"]*"|'[^']*'|[^\s>]+)/gi, "");
  }
  const document = new DOMParser().parseFromString(html, "text/html");
  document.querySelectorAll("script, iframe, object, embed, base").forEach((element) => element.remove());
  document.querySelectorAll("*").forEach((element) => {
    for (const attribute of Array.from(element.attributes)) {
      if (attribute.name.startsWith("on")) element.removeAttribute(attribute.name);
    }
  });
  return document.body.innerHTML;
}

const mailHTMLFrameCSS = `
html,body{margin:0;padding:0;background:transparent;color:#17191c;}
body{overflow-wrap:anywhere;}
img{max-width:100%;height:auto;}
img:not([src]){display:none;}
table{max-width:100%;}
pre{white-space:pre-wrap;overflow-x:auto;}
`;

export function renderTextWithLinks(text: string) {
  const parts: ReactNode[] = [];
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
