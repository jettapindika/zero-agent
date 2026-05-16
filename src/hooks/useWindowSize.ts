import { useStdout } from "ink";

export function useWindowSize() {
  const { stdout } = useStdout();
  return {
    columns: stdout?.columns ?? 120,
    rows: stdout?.rows ?? 40
  };
}
