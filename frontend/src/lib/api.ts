import type {
  WikiPage,
  WikiTreeNode,
  Conversation,
  ConversationMessage,
  Plan,
  ExecutionReport,
  ToolCallInfo,
  PermissionRequestEvent,
  AskUserRequestEvent,
  PermissionDecisionInput,
} from "../types";

// Write tools require user permission before execution. When a `tool_call_start`
// arrives for one of these, the frontend stamps the card as `pending`.
const WRITE_TOOLS = new Set([
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
}

export async function streamChat(
  req: ChatRequest,
  onChunk: (content: string) => void,
  onMeta: (data: { conversation_id?: number; plan?: Plan }) => void,
  onStatus?: (data: { step: number; max_steps: number; status: string }) => void,
  onError?: (error: string) => void,
  onToolCall?: (data: ToolCallInfo) => void,
  onPermissionRequired?: (data: PermissionRequestEvent) => void,
  onAskUserRequest?: (data: AskUserRequestEvent) => void,
): Promise<void> {
  const res = await fetch(`${BASE}/ai/chat`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(req),
  });

  if (!res.ok) {
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

  // Final dispatch in case stream ends without trailing empty line
  dispatchEvent();
}

// Plan API

export async function confirmPlan(planId: string, focusPageId?: number | null): Promise<ExecutionReport> {
  const res = await fetch(`${BASE}/plans/confirm`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ plan_id: planId, focus_page_id: focusPageId ?? null }),
  });
  if (!res.ok) throw new Error("Failed to confirm plan");
  return res.json();
}

export async function rejectPlan(planId: string): Promise<void> {
  const res = await fetch(`${BASE}/plans/reject`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ plan_id: planId }),
  });
  if (!res.ok) throw new Error("Failed to reject plan");
}

export async function getPlan(planId: string): Promise<Plan> {
  const res = await fetch(`${BASE}/plans?id=${encodeURIComponent(planId)}`);
  if (!res.ok) throw new Error("Failed to get plan");
  return res.json();
}

export async function createPlan(params: {
  reasoning: string;
  actions: { type: string; params: Record<string, unknown> }[];
}): Promise<Plan> {
  const res = await fetch(`${BASE}/plans`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(params),
  });
  if (!res.ok) throw new Error("Failed to create plan");
  return res.json();
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
