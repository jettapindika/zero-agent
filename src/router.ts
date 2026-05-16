import OpenAI from "openai";

const ROUTER_BASE_URL = "http://127.0.0.1:20128/v1";
const ROUTER_API_KEY = "sk_9router";
const DEFAULT_MODEL = "kr/claude-opus-4.6";

export const client = new OpenAI({
  baseURL: ROUTER_BASE_URL,
  apiKey: ROUTER_API_KEY,
});

export const AVAILABLE_MODELS = [
  { label: "claude-opus-4.6", value: "kr/claude-opus-4.6" },
  { label: "claude-sonnet-4.5", value: "kr/claude-sonnet-4.5" },
  { label: "claude-haiku-4.5", value: "kr/claude-haiku-4.5" },
  { label: "gpt-5.5", value: "cx/gpt-5.5" },
  { label: "gpt-5.4", value: "cx/gpt-5.4" },
  { label: "glm-5", value: "kr/glm-5" },
] as const;

export { DEFAULT_MODEL };
