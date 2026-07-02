type IconProps = {
  name:
    | "menu"
    | "search"
    | "compose"
    | "refresh"
    | "settings"
    | "chevron"
    | "inbox"
    | "star"
    | "send"
    | "draft"
    | "archive"
    | "spam"
    | "trash"
    | "folder"
    | "reply"
    | "replyAll"
    | "forward"
    | "more"
    | "paperclip"
    | "download"
    | "image"
    | "mail"
    | "alert"
    | "minimize"
    | "expand"
    | "close"
    | "bold"
    | "italic";
  className?: string;
};

const paths: Record<IconProps["name"], string[]> = {
  menu: ["M4 7h16", "M4 12h16", "M4 17h16"],
  search: ["M10.5 18a7.5 7.5 0 1 1 5.3-12.8 7.5 7.5 0 0 1-5.3 12.8Z", "m16 16 4 4"],
  compose: ["M12 20h9", "M16.5 3.5a2.1 2.1 0 0 1 3 3L8 18l-4 1 1-4L16.5 3.5Z"],
  refresh: ["M20 6v5h-5", "M4 18v-5h5", "M18.4 9A7 7 0 0 0 6.8 6.8L4 10", "M5.6 15a7 7 0 0 0 11.6 2.2L20 14"],
  settings: ["M12 15.5a3.5 3.5 0 1 0 0-7 3.5 3.5 0 0 0 0 7Z", "M19 12a7 7 0 0 0-.1-1l2-1.5-2-3.4-2.4 1a7 7 0 0 0-1.7-1L14.5 3h-5l-.3 3.1a7 7 0 0 0-1.7 1l-2.4-1-2 3.4 2 1.5a7 7 0 0 0 0 2l-2 1.5 2 3.4 2.4-1a7 7 0 0 0 1.7 1l.3 3.1h5l.3-3.1a7 7 0 0 0 1.7-1l2.4 1 2-3.4-2-1.5a7 7 0 0 0 .1-1Z"],
  chevron: ["m8 10 4 4 4-4"],
  inbox: ["M4 5h16l-2 10H6L4 5Z", "M8 15a4 4 0 0 0 8 0"],
  star: ["m12 3 2.7 5.5 6.1.9-4.4 4.3 1 6.1-5.4-2.9-5.4 2.9 1-6.1-4.4-4.3 6.1-.9L12 3Z"],
  send: ["M4 4l17 8-17 8 4-8-4-8Z", "M8 12h13"],
  draft: ["M5 4h10l4 4v12H5V4Z", "M14 4v5h5"],
  archive: ["M4 7h16v13H4V7Z", "M3 4h18v3H3V4Z", "M9 12h6"],
  spam: ["M12 3 21 19H3L12 3Z", "M12 9v4", "M12 17h.01"],
  trash: ["M4 7h16", "M9 7V5h6v2", "M7 7l1 13h8l1-13", "M10 11v5", "M14 11v5"],
  folder: ["M3 6h7l2 2h9v10H3V6Z"],
  reply: ["M10 8 5 12l5 4", "M6 12h8a5 5 0 0 1 5 5v1"],
  replyAll: ["M8 8 3 12l5 4", "M13 8 8 12l5 4", "M9 12h6a5 5 0 0 1 5 5v1"],
  forward: ["m14 8 5 4-5 4", "M5 12h14"],
  more: ["M6 12h.01", "M12 12h.01", "M18 12h.01"],
  paperclip: ["m21 11-8.5 8.5a5 5 0 0 1-7-7L14 4a3 3 0 0 1 4 4l-8.5 8.5a1.5 1.5 0 0 1-2-2L15 7"],
  download: ["M12 3v12", "m8 11 4 4 4-4", "M5 20h14"],
  image: ["M4 5h16v14H4V5Z", "m7 13 3-4 2 2 3-5 3 7", "M8 9h.01"],
  mail: ["M4 6h16v12H4V6Z", "m4 8 8 5 8-5"],
  alert: ["M12 3 21 19H3L12 3Z", "M12 9v4", "M12 17h.01"],
  minimize: ["M6 12h12"],
  expand: ["M8 3H3v5", "M16 3h5v5", "M8 21H3v-5", "M16 21h5v-5"],
  close: ["M6 6l12 12", "M18 6 6 18"],
  bold: ["M8 5h6a3 3 0 0 1 0 6H8V5Z", "M8 11h7a3 3 0 0 1 0 6H8v-6Z"],
  italic: ["M10 5h8", "M6 19h8", "M14 5l-4 14"],
};

export function Icon({ name, className = "h-4 w-4" }: IconProps) {
  return (
    <svg className={className} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      {paths[name].map((d) => (
        <path key={d} d={d} />
      ))}
    </svg>
  );
}
