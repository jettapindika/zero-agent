import React from "react";
import { Text } from "ink";
import { theme } from "../theme.js";

const frames = ["\u280B", "\u2819", "\u2839", "\u2838", "\u283C", "\u2834", "\u2826", "\u2827", "\u2807", "\u280F"] as const;

type Props = {
  label?: string;
};

export function Spinner({ label }: Props) {
  const [index, setIndex] = React.useState(0);

  React.useEffect(() => {
    const timer = setInterval(() => {
      setIndex((current) => (current + 1) % frames.length);
    }, 80);
    return () => clearInterval(timer);
  }, []);

  return <Text color={theme.accent}>{frames[index]}{label ? ` ${label}` : ""}</Text>;
}
