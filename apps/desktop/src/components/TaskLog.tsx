type Props = {
  lines: string[];
};

// Ambient console-noise log. Dim by default; never compete with main output.
export function TaskLog({ lines }: Props) {
  if (lines.length === 0) return null;
  return (
    <ul className="task-log" aria-label="Background tasks">
      {lines.map((body, idx) => (
        <li className="task-log-row" key={idx}>
          <span className="task-log-prefix">[task]</span>
          <span className="task-log-body">{body}</span>
        </li>
      ))}
    </ul>
  );
}
