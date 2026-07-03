import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { renderTextWithLinks, trimTrailingPunctuation } from "./mailRendering";

describe("mailRendering", () => {
  it("autolinks http URLs without trailing sentence punctuation", () => {
    const html = renderToStaticMarkup(<p>{renderTextWithLinks("Open https://example.com/report.")}</p>);

    expect(html).toContain('<a class="text-accent underline" href="https://example.com/report"');
    expect(html).toContain(">https://example.com/report</a>.");
  });

  it("trims common trailing punctuation", () => {
    expect(trimTrailingPunctuation("https://example.com/test),")).toBe("https://example.com/test");
  });
});
