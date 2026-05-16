import * as fs from "node:fs/promises";
import * as path from "node:path";

const SESSION_DIR = path.join(process.env.HOME ?? "~", ".pi-opencode", "sessions");

export type SavedSession = {
  id: string;
  title: string;
  createdAt: number;
  updatedAt: number;
  messages: Array<{ role: string; content: string | null; tool_calls?: unknown[]; tool_call_id?: string }>;
};

async function ensureDir() {
  await fs.mkdir(SESSION_DIR, { recursive: true });
}

export async function saveSession(session: SavedSession): Promise<void> {
  await ensureDir();
  const filePath = path.join(SESSION_DIR, `${session.id}.json`);
  await fs.writeFile(filePath, JSON.stringify(session, null, 2), "utf-8");
}

export async function loadSession(id: string): Promise<SavedSession | null> {
  try {
    const filePath = path.join(SESSION_DIR, `${id}.json`);
    const content = await fs.readFile(filePath, "utf-8");
    return JSON.parse(content) as SavedSession;
  } catch {
    return null;
  }
}

export async function listSessions(): Promise<Array<{ id: string; title: string; updatedAt: number }>> {
  await ensureDir();
  try {
    const files = await fs.readdir(SESSION_DIR);
    const sessions: Array<{ id: string; title: string; updatedAt: number }> = [];

    for (const file of files) {
      if (!file.endsWith(".json")) continue;
      try {
        const content = await fs.readFile(path.join(SESSION_DIR, file), "utf-8");
        const data = JSON.parse(content) as SavedSession;
        sessions.push({ id: data.id, title: data.title, updatedAt: data.updatedAt });
      } catch {
        continue;
      }
    }

    return sessions.sort((a, b) => b.updatedAt - a.updatedAt);
  } catch {
    return [];
  }
}

export async function deleteSession(id: string): Promise<void> {
  try {
    await fs.unlink(path.join(SESSION_DIR, `${id}.json`));
  } catch {
  }
}
