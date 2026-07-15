import { render, screen } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { MailHTMLBody, mailHTMLSrcDoc, renderTextWithLinks, trimTrailingPunctuation } from "./mailRendering";

describe("mailRendering", () => {
  it("autolinks http URLs without trailing sentence punctuation", () => {
    const html = renderToStaticMarkup(<p>{renderTextWithLinks("Open https://example.com/report.")}</p>);

    expect(html).toContain('<a class="text-accent underline" href="https://example.com/report"');
    expect(html).toContain(">https://example.com/report</a>.");
  });

  it("trims common trailing punctuation", () => {
    expect(trimTrailingPunctuation("https://example.com/test),")).toBe("https://example.com/test");
  });

  it("isolates backend-sanitized HTML in an iframe srcdoc", () => {
    const html = '<table width="640"><tr><td><font color="#333333">Cell</font></td></tr></table>';
    const { container } = render(<MailHTMLBody html={html} />);

    const frame = screen.getByTitle("메일 HTML 본문");
    expect(frame).toHaveAttribute("srcdoc", expect.stringContaining('<table width="640">'));
    expect(frame).toHaveAttribute("srcdoc", expect.stringContaining('<font color="#333333">Cell</font>'));
    expect(container.querySelector("table")).toBeNull();
  });

  it("builds the mail HTML frame without app-level content classes", () => {
    const srcDoc = mailHTMLSrcDoc('<p class="external">Hello</p>');

    expect(srcDoc).toContain('<p class="external">Hello</p>');
    expect(srcDoc).not.toContain("[&_p]");
    expect(srcDoc).not.toContain("text-sm");
  });

  it("removes executable email markup before creating the sandboxed document", () => {
    const srcDoc = mailHTMLSrcDoc('<p onclick="alert(1)">Hello</p><script>alert(1)</script>');

    expect(srcDoc).toContain("<p>Hello</p>");
    expect(srcDoc).not.toContain("onclick");
    expect(srcDoc).not.toContain("<script");
  });
});
