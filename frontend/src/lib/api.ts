import type { WikiPage, WikiTreeNode, Conversation, ConversationMessage, PendingAction } from "../types";

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
  return data ?? [];
}

export async function createConversation(title?: string): Promise<Conversation> {
  const res = await fetch(`${BASE}/ai/conversations`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      role: "wiki_maintainer",
      context_type: "wiki",
      title,
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
  return data ?? [];
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
  confirmed_actions?: PendingAction[];
}

export async function streamChat(
  req: ChatRequest,
  onChunk: (content: string) => void,
  onMeta: (data: { conversation_id?: number; pending_actions?: PendingAction[] }) => void,
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

  while (true) {
    const { done, value } = await reader.read();
    if (done) break;

    buffer += decoder.decode(value, { stream: true });
    const lines = buffer.split("\n");
    buffer = lines.pop() || "";

    for (const line of lines) {
      if (line.startsWith("event: meta")) continue;
      if (line.startsWith("data: ")) {
        const data = line.slice(6);
        if (data === "[DONE]") continue;

        try {
          const parsed = JSON.parse(data);
          if (typeof parsed.conversation_id === "number" || parsed.pending_actions) {
            onMeta(parsed);
            continue;
          }
        } catch {
          // Not JSON — plain content
        }

        onChunk(data);
      }
    }
  }
}
