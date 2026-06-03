import type {
  WikiPage,
  WikiTreeNode,
  Conversation,
  ConversationMessage,
  ToolCallInfo,
  PermissionRequestEvent,
  AskUserRequestEvent,
  PermissionDecisionInput,
} from "../types";

// Write tools require user permission before execution. When a `tool_call_start`
// arrives for one of these, the frontend stamps the card as `pending`.
export const WRITE_TOOLS = new Set([
  "create_page",
  "update_page",
  "patch_page",
  "delete_page",
  "link_pages",
  "move_page",
]);

// Read tools run without permission. Stamped `running` on start, `done`/`error`
// on result. (ask_user is a separate flow handled via its own event.)
function statusForStart(name: string): "pending" | "running" {
  return WRITE_TOOLS.has(name) ? "pending" : "running";
}

function statusForResult(error: string): "done" | "error" {
  return error ? "error" : "done";
}

const BASE = "/api";

// Wiki API

export async function fetchWikiTree(): Promise<WikiTreeNode[]> {
  const res = await fetch(`${BASE}/wiki`);
  if (!res.ok) throw new Error("Failed to fetch wiki tree");
  const data = await res.json();
  return data.tree ?? [];
}

export async function fetchWikiPage(slug: string): Promise<WikiPage> {
  const res = await fetch(`${BASE}/wiki/${slug}`);
  if (!res.ok) throw new Error(`Failed to fetch page: ${slug}`);
  return res.json();
}

export async function fetchOverviewPage(): Promise<WikiPage> {
  const res = await fetch(`${BASE}/wiki/overview`);
  if (!res.ok) throw new Error("Failed to fetch overview");
  return res.json();
}

/**
 * Fetch a wiki page via the public share API.
 *
 * Used by the SPA when the user is on /share/{slug}?t={token} (anonymous
 * visitor view). The response omits `share_token` for security.
 *
 * Throws on 404 (token miss / page missing) — the caller can show an
 * appropriate error UI in that case.
 */
export async function fetchPublicSharePage(slug: string, token: string): Promise<WikiPage> {
  const res = await fetch(`${BASE}/share/${slug}?t=${encodeURIComponent(token)}`);
  if (!res.ok) throw new Error(`Public share fetch failed: ${res.status}`);
  return res.json();
}

// Conversation API

export async function listConversations(): Promise<Conversation[]> {
  const res = await fetch(`${BASE}/ai/conversations`);
  if (!res.ok) throw new Error("Failed to fetch conversations");
  const data = await res.json();
  return data.conversations ?? [];
}

export async function createConversation(): Promise<Conversation> {
  const res = await fetch(`${BASE}/ai/conversations`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      role: "wiki_maintainer",
      context_type: "wiki",
    }),
  });
  if (!res.ok) throw new Error("Failed to create conversation");
  return res.json();
}

export async function updateConversationTitle(id: number, title: string): Promise<Conversation> {
  const res = await fetch(`${BASE}/ai/conversations/${id}`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ title }),
  });
  if (!res.ok) throw new Error("Failed to update conversation title");
  return res.json();
}

export async function getConversationMessages(id: number): Promise<ConversationMessage[]> {
  const res = await fetch(`${BASE}/ai/conversations/${id}/messages`);
  if (!res.ok) throw new Error("Failed to fetch conversation messages");
  const data = await res.json();
  const messages: ConversationMessage[] = data.messages ?? [];
  // Parse tool_calls from JSON string to array (API returns raw JSON string from DB)
  for (const msg of messages) {
    if (typeof msg.tool_calls === "string") {
      try {
        msg.tool_calls = JSON.parse(msg.tool_calls);
      } catch {
        msg.tool_calls = undefined;
      }
    }
  }
  return messages;
}

export async function deleteConversation(id: number): Promise<void> {
  const res = await fetch(`${BASE}/ai/conversations/${id}`, { method: "DELETE" });
  if (!res.ok && res.status !== 204) throw new Error("Failed to delete conversation");
}

// AI Chat with SSE streaming

export interface ChatRequest {
  conversation_id: number;
  message: string;
  role?: string;
  context_type?: string;
  plan_id?: string;
  focus_page_id?: number | null;
  current_slug?: string;
  selected_text?: string;
  skill?: string; // optional: SKILL.md name for /command
}

export async function streamChat(
  req: ChatRequest,
  onChunk: (content: string) => void,
  onMeta: (data: { conversation_id?: number }) => void,
  onStatus?: (data: { step: number; max_steps: number; status: string }) => void,
  onError?: (error: string) => void,
  onToolCall?: (data: ToolCallInfo) => void,
  onPermissionRequired?: (data: PermissionRequestEvent) => void,
  onAskUserRequest?: (data: AskUserRequestEvent) => void,
  signal?: AbortSignal,
): Promise<void> {
  const res = await fetch(`${BASE}/ai/chat`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(req),
    signal,
  });

  if (!res.ok) {
    if (signal?.aborted) return;
    throw new Error(`Chat failed: ${await res.text()}`);
  }

  const reader = res.body?.getReader();
  if (!reader) return;

  const decoder = new TextDecoder();
  let buffer = "";
  let currentEvent = "";
  let currentData = "";

  const dispatchEvent = () => {
    if (!currentEvent || !currentData) {
      currentEvent = "";
      currentData = "";
      return;
    }

    if (currentEvent === "agent_status" && onStatus) {
      try { onStatus(JSON.parse(currentData)); } catch { /* ignore */ }
      currentEvent = "";
      currentData = "";
      return;
    }

    if (currentEvent === "meta") {
      try { onMeta(JSON.parse(currentData)); } catch { /* ignore */ }
      currentEvent = "";
      currentData = "";
      return;
    }

    if (currentEvent === "content") {
      onChunk(currentData);
      currentEvent = "";
      currentData = "";
      return;
    }

    if (currentEvent === "error" && onError) {
      try { onError(currentData); } catch { /* ignore */ }
      currentEvent = "";
      currentData = "";
      return;
    }

    if (currentEvent === "tool_call_start" && onToolCall) {
      try {
        const data = JSON.parse(currentData);
        onToolCall({
          id: data.id,
          name: data.name,
          input: data.input || {},
          output: "",
          error: "",
          status: statusForStart(data.name),
        });
      } catch { /* ignore */ }
      currentEvent = "";
      currentData = "";
      return;
    }

    if (currentEvent === "tool_result" && onToolCall) {
      try {
        const data = JSON.parse(currentData);
        const errorStr = data.error || "";
        onToolCall({
          id: data.id,
          name: data.name,
          input: data.input || {},
          output: data.output || "",
          error: errorStr,
          status: statusForResult(errorStr),
        });
      } catch { /* ignore */ }
      currentEvent = "";
      currentData = "";
      return;
    }

    if (currentEvent === "permission_required" && onPermissionRequired) {
      try { onPermissionRequired(JSON.parse(currentData)); } catch { /* ignore */ }
      currentEvent = "";
      currentData = "";
      return;
    }

    if (currentEvent === "ask_user_request" && onAskUserRequest) {
      try { onAskUserRequest(JSON.parse(currentData)); } catch { /* ignore */ }
      currentEvent = "";
      currentData = "";
      return;
    }

    // Unknown events (done, etc.) — ignore
    currentEvent = "";
    currentData = "";
  };

  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;

      buffer += decoder.decode(value, { stream: true });
      const lines = buffer.split("\n");
      buffer = lines.pop() || "";

      for (const line of lines) {
        if (line.startsWith("event: ")) {
          dispatchEvent();
          currentEvent = line.slice(7);
          currentData = "";
          continue;
        }
        if (line.startsWith("data: ")) {
          const data = line.slice(6);
          if (data === "[DONE]") continue;
          if (currentData !== "") {
            currentData += "\n";
          }
          currentData += data;
          continue;
        }
        // Empty line — dispatch pending event
        if (line === "") {
          dispatchEvent();
          currentEvent = "";
          currentData = "";
        }
      }
    }
  } catch (e) {
    if (signal?.aborted) return;
    throw e;
  }

  // Final dispatch in case stream ends without trailing empty line
  dispatchEvent();
}

// Wiki Tree Operations

export async function createEmptyWikiPage(title: string, parentId: number | null): Promise<WikiPage> {
  const res = await fetch(`${BASE}/wiki/quick-create`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ title, parent_id: parentId }),
  });
  if (!res.ok) throw new Error("Failed to create page");
  return res.json();
}

export async function renameWikiPage(pageId: number, newTitle: string): Promise<void> {
  const res = await fetch(`${BASE}/wiki/${pageId}/rename`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ title: newTitle }),
  });
  if (!res.ok) throw new Error("Failed to rename page");
}

export async function moveWikiPage(pageId: number, newParentId: number | null): Promise<void> {
  const res = await fetch(`${BASE}/wiki/${pageId}/move`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ parent_id: newParentId }),
  });
  if (!res.ok) throw new Error("Failed to move page");
}

export async function deleteWikiPage(id: number): Promise<void> {
  const res = await fetch(`/api/wiki/${id}`, { method: "DELETE" });
  if (!res.ok) throw new Error("Failed to delete wiki page");
}

export async function confirmPageContent(pageId: number): Promise<void> {
  const res = await fetch(`/api/wiki/${pageId}/confirm`, { method: "PUT" });
  if (!res.ok) throw new Error("Failed to confirm page content");
}

// Permission / ask_user response endpoints (Phase 7)

export async function postPermissionResponse(
  requestId: string,
  decisions: PermissionDecisionInput[],
): Promise<void> {
  const res = await fetch(`${BASE}/ai/permission_response`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ request_id: requestId, decisions }),
  });
  if (!res.ok) throw new Error("Failed to send permission response");
}

export async function postAskUserResponse(
  requestId: string,
  answer: string | string[] | "no_answer",
): Promise<void> {
  const res = await fetch(`${BASE}/ai/ask_user_response`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ request_id: requestId, answer }),
  });
  if (!res.ok) throw new Error("Failed to send ask_user response");
}

export interface SkillInfo {
  name: string;
  description: string;
}

export async function fetchSkills(): Promise<SkillInfo[]> {
  const res = await fetch(`${BASE}/skills`);
  if (!res.ok) throw new Error(`fetchSkills failed: ${res.status}`);
  return res.json();
}

// Twitter accounts

export type TrackedAccount = {
  id: number;
  handle: string;
  display_name?: string;
  enabled: boolean;
  notes?: string;
};

export async function listTwitterAccounts(): Promise<TrackedAccount[]> {
  const res = await fetch(`${BASE}/twitter/accounts`);
  if (!res.ok) throw new Error("listTwitterAccounts failed");
  return res.json();
}

export async function createTwitterAccount(handle: string, notes?: string): Promise<TrackedAccount> {
  const res = await fetch(`${BASE}/twitter/accounts`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ handle, notes }),
  });
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}

export async function updateTwitterAccount(
  id: number,
  patch: Partial<Pick<TrackedAccount, "handle" | "enabled" | "notes">>,
): Promise<void> {
  const res = await fetch(`${BASE}/twitter/accounts/${id}`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(patch),
  });
  if (!res.ok) throw new Error(await res.text());
}

export async function deleteTwitterAccount(id: number): Promise<void> {
  const res = await fetch(`${BASE}/twitter/accounts/${id}`, { method: "DELETE" });
  if (!res.ok) throw new Error(await res.text());
}

export async function getTwitterConfig(): Promise<{ rsshub_base_url: string }> {
  const res = await fetch(`${BASE}/twitter/config`);
  if (!res.ok) throw new Error("getTwitterConfig failed");
  return res.json();
}

export async function setTwitterConfig(rsshub_base_url: string): Promise<void> {
  const res = await fetch(`${BASE}/twitter/config`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ rsshub_base_url }),
  });
  if (!res.ok) throw new Error(await res.text());
}

// ── Twitter bulk import ──

export type BulkImportResult = {
  source: string;
  total_found: number;
  added: number;
  skipped_existing: number;
  added_handles?: string[];
  error?: string;
};

export async function bulkImportTwitterAccounts(url?: string): Promise<BulkImportResult> {
  const res = await fetch(`${BASE}/twitter/accounts/bulk-import`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(url ? { url } : {}),
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `bulkImport failed (${res.status})`);
  }
  return res.json();
}

// Cron "run now"

export async function runCronTaskNow(taskId: number): Promise<{ run_id: number }> {
  const res = await fetch(`${BASE}/cron/tasks/${taskId}/run-now`, { method: "POST" });
  if (!res.ok) throw new Error(await res.text());
  return res.json();
}
