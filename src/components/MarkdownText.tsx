import { Box, Text } from "ink";
import { theme } from "../theme.js";

type Block =
  | { type: "text"; content: string }
  | { type: "code"; lang: string; content: string };

export function parseMarkdown(text: string): Block[] {
  const blocks: Block[] = [];
  const lines = text.split("\n");
  let i = 0;
  let currentText = "";

  while (i < lines.length) {
    const line = lines[i]!;
    if (line.startsWith("```")) {
      if (currentText) {
        blocks.push({ type: "text", content: currentText.trimEnd() });
        currentText = "";
      }
      const lang = line.slice(3).trim();
      i++;
      let code = "";
      while (i < lines.length && !lines[i]!.startsWith("```")) {
        code += (code ? "\n" : "") + lines[i];
        i++;
      }
      blocks.push({ type: "code", lang, content: code });
      i++;
    } else {
      currentText += (currentText ? "\n" : "") + line;
      i++;
    }
  }

  if (currentText) {
    blocks.push({ type: "text", content: currentText.trimEnd() });
  }

  return blocks;
}

type Props = {
  content: string;
};

export function MarkdownText({ content }: Props) {
  const blocks = parseMarkdown(content);

  return (
    <Box flexDirection="column">
      {blocks.map((block, idx) => {
        if (block.type === "code") {
          return (
            <Box key={idx} flexDirection="column" marginY={1} paddingX={1} paddingY={1} borderStyle="single" borderColor={theme.dim}>
              {block.lang && <Text color={theme.muted} dimColor>{block.lang}</Text>}
              <Text color={theme.green} wrap="wrap">{block.content}</Text>
            </Box>
          );
        }
        return <Text key={idx} color={theme.text} wrap="wrap">{block.content}</Text>;
      })}
    </Box>
  );
}
