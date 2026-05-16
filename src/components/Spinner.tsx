import React from "react";
import { Text } from "ink";
import { theme } from "../theme.js";

const BLOCK_FRAMES = [
  "\u2588\u2591\u2591\u2591\u2591",
  "\u2591\u2588\u2591\u2591\u2591",
  "\u2591\u2591\u2588\u2591\u2591",
  "\u2591\u2591\u2591\u2588\u2591",
  "\u2591\u2591\u2591\u2591\u2588",
  "\u2591\u2591\u2591\u2588\u2591",
  "\u2591\u2591\u2588\u2591\u2591",
  "\u2591\u2588\u2591\u2591\u2591",
] as const;

const DOT_FRAMES = ["\u25CF", "\u25CB", "\u25CF", "\u25CB"] as const;

type Props = {
  label?: string;
  style?: "blocks" | "dots";
};

export function Spinner({ label, style = "blocks" }: Props) {
  const [index, setIndex] = React.useState(0);
  const frames = style === "dots" ? DOT_FRAMES : BLOCK_FRAMES;
  const interval = style === "dots" ? 400 : 100;

  React.useEffect(() => {
    const timer = setInterval(() => {
      setIndex((current) => (current + 1) % frames.length);
    }, interval);
    return () => clearInterval(timer);
  }, [frames.length, interval]);

  return (
    <Text color={theme.accent}>
      {frames[index]}{label ? ` ${label}` : ""}
    </Text>
  );
}
