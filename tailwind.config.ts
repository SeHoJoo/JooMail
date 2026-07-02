import type { Config } from "tailwindcss";

export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        accent: "#2d64d8",
        ink: "#17191c",
        text: "#25292e",
        muted: "#9298a0",
        line: "#e8e9ec",
        panel: "#f6f7f8",
        selected: "#eaf1fd",
      },
      fontFamily: {
        sans: [
          "-apple-system",
          "BlinkMacSystemFont",
          "Apple SD Gothic Neo",
          "Noto Sans KR",
          "Segoe UI",
          "sans-serif",
        ],
      },
      boxShadow: {
        compose: "0 8px 28px -4px rgba(0,0,0,0.22)",
      },
    },
  },
  plugins: [],
} satisfies Config;
