export const theme = {
  bg: "#0d0d0f",
  surface: "#111113",
  border: "#1e1e24",
  borderHi: "#2e2e38",
  accent: "#7c6af7",
  green: "#22c55e",
  yellow: "#f59e0b",
  red: "#ef4444",
  blue: "#3b82f6",
  text: "#e2e2f0",
  muted: "#6b6b80",
  dim: "#3a3a4a",
  userBg: "#1a1730",
  aiBg: "#111113",
  toolBg: "#0f1117"
} as const;

export type Theme = typeof theme;

export function truncateText(value: string, width: number): string {
  if (width <= 0) return "";
  if (value.length <= width) return value;
  if (width === 1) return "\u2026";
  return `${value.slice(0, width - 1)}\u2026`;
}
