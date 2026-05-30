import { useState, useEffect, useRef, useCallback } from "react";
import type { Conversation, ConversationMessage, PendingAction } from "../types";
import {
  listConversations,
  createConversation,
  updateConversationTitle,
  deleteConversation,
  getConversationMessages,
  streamChat,
} from "../lib/api";
import { MarkdownContent } from "./MarkdownContent";

const STORAGE_KEY = "llm-wiki-active-conversation-id";

interface ChatPanelProps {
  onPageChanged?: () => void;
}

export function ChatPanel({ onPageChanged }: ChatPanelProps) {
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [activeConv, setActiveConv] = useState<Conversation | null>(null);
  const [messages, setMessages] = useState<ConversationMessage[]>([]);
  const [input, setInput] = useState("");
  const [loading, setLoading] = useState(false);
  const [showNewDialog, setShowNewDialog] = useState(false);
  const [editingTitle, setEditingTitle] = useState(false);
  const [titleDraft, setTitleDraft] = useState("");
  const [agentStatus, setAgentStatus] = useState<{
    step: number;
    maxSteps: number;
    running: boolean;
  } | null>(null);

  const messagesEndRef = useRef<HTMLDivElement>(null);

  const scrollToBottom = useCallback(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, []);

  useEffect(() => {
    scrollToBottom();
  }, [messages, scrollToBottom]);

  useEffect(() => {
    loadConversations();
  }, []);

  useEffect(() => {
    if (conversations.length === 0) return;
    const storedId = localStorage.getItem(STORAGE_KEY);
    if (storedId) {
      const id = parseInt(storedId, 10);
      const conv = conversations.find((c) => c.id === id);
      if (conv) {
        switchToConversation(conv);
      } else {
        localStorage.removeItem(STORAGE_KEY);
      }
    }
  }, [conversations.length > 0]);

  async function loadConversations() {
    try {
      const convs = await listConversations();
      setConversations(convs || []);
    } catch {
      // ignore
    }
  }

  async function switchToConversation(conv: Conversation) {
    setActiveConv(conv);
    localStorage.setItem(STORAGE_KEY, String(conv.id));
    try {
      const msgs = await getConversationMessages(conv.id);
      setMessages(msgs || []);
    } catch {
      setMessages([]);
    }
  }

  async function handleCreateConversation(title?: string) {
    try {
      const conv = await createConversation(title);
      await loadConversations();
      await switchToConversation(conv);
      setShowNewDialog(false);
      setTitleDraft("");
    } catch (e) {
      console.error("Failed to create conversation:", e);
    }
  }

  async function handleDeleteConversation() {
    if (!activeConv) return;
    try {
      await deleteConversation(activeConv.id);
      localStorage.removeItem(STORAGE_KEY);
      setActiveConv(null);
      setMessages([]);
      await loadConversations();
    } catch (e) {
      console.error("Failed to delete conversation:", e);
    }
  }

  async function handleRenameTitle() {
    if (!activeConv || !titleDraft.trim()) return;
    try {
      await updateConversationTitle(activeConv.id, titleDraft.trim());
      setActiveConv({ ...activeConv, title: titleDraft.trim() });
      setEditingTitle(false);
      await loadConversations();
    } catch (e) {
      console.error("Failed to update title:", e);
    }
  }

  async function handleSend(confirmedActions?: PendingAction[]) {
    if (!activeConv || loading) return;

    const userContent = input.trim();
    if (!userContent && !confirmedActions) return;

    if (userContent) {
      const userMsg: ConversationMessage = {
        id: Date.now(),
        role: "user",
        content: userContent,
        model_provider: null,
        token_count: null,
        created_at: new Date().toISOString(),
      };
      setMessages((prev) => [...prev, userMsg]);
      setInput("");
    }

    setLoading(true);
    setAgentStatus({ step: 0, maxSteps: 20, running: true });

    const assistantMsg: ConversationMessage = {
      id: Date.now() + 1,
      role: "assistant",
      content: "",
      model_provider: null,
      token_count: null,
      created_at: new Date().toISOString(),
    };
    setMessages((prev) => [...prev, assistantMsg]);

    try {
      let fullContent = "";
      let newConvId: number | undefined;

      await streamChat(
        {
          conversation_id: activeConv.id,
          message: userContent,
          role: "wiki_maintainer",
          context_type: "wiki",
          confirmed_actions: confirmedActions,
        },
        (content) => {
          fullContent += content;
          setMessages((prev) => {
            const updated = [...prev];
            const last = updated[updated.length - 1];
            if (last.role === "assistant") {
              updated[updated.length - 1] = { ...last, content: fullContent };
            }
            return updated;
          });
        },
        (meta) => {
          if (meta.conversation_id) {
            newConvId = meta.conversation_id;
          }
          if (meta.pending_actions) {
            setMessages((prev) => {
              const updated = [...prev];
              const last = updated[updated.length - 1];
              if (last.role === "assistant") {
                updated[updated.length - 1] = { ...last, pending_actions: meta.pending_actions };
              }
              return updated;
            });
          }
        },
        (status) => {
          setAgentStatus({ step: status.step, maxSteps: status.max_steps, running: true });
        },
      );

      if (newConvId && newConvId !== activeConv.id) {
        setActiveConv((prev) => (prev ? { ...prev, id: newConvId! } : prev));
        localStorage.setItem(STORAGE_KEY, String(newConvId));
        await loadConversations();
      }

      if (confirmedActions && confirmedActions.length > 0) {
        onPageChanged?.();
      }
    } catch (e) {
      setMessages((prev) => {
        const updated = [...prev];
        updated[updated.length - 1] = {
          ...updated[updated.length - 1],
          content: `Error: ${e}`,
        };
        return updated;
      });
    } finally {
      setLoading(false);
      setAgentStatus(null);
    }
  }

  function handleConfirm(actions: PendingAction[]) {
    handleSend(actions);
  }

  return (
    <div className="flex flex-col h-full bg-th-bg-secondary">
      {/* Header */}
      <div className="flex items-center gap-2 p-3 border-b border-th-border bg-th-bg-tertiary shrink-0">
        <select
          className="flex-1 text-sm border border-th-input-border rounded px-2 py-1.5 bg-th-input-bg text-th-text-primary"
          value={activeConv?.id ?? ""}
          onChange={(e) => {
            const conv = conversations.find((c) => c.id === Number(e.target.value));
            if (conv) switchToConversation(conv);
          }}
        >
          <option value="" disabled>
            选择会话...
          </option>
          {conversations.map((c) => (
            <option key={c.id} value={c.id}>
              {c.title || `会话 ${c.id}`} ({c.message_count})
            </option>
          ))}
        </select>
        <button
          onClick={() => setShowNewDialog(true)}
          className="p-1.5 text-th-text-muted hover:text-th-accent hover:bg-th-accent-bg rounded"
          title="新建会话"
        >
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
          </svg>
        </button>
        {activeConv && (
          <>
            <button
              onClick={() => {
                setTitleDraft(activeConv.title || "");
                setEditingTitle(true);
              }}
              className="p-1.5 text-th-text-muted hover:text-th-success hover:bg-th-accent-bg rounded"
              title="重命名"
            >
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
              </svg>
            </button>
            <button
              onClick={handleDeleteConversation}
              className="p-1.5 text-th-text-muted hover:text-th-error hover:bg-th-accent-bg rounded"
              title="删除会话"
            >
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
              </svg>
            </button>
          </>
        )}
      </div>

      {/* Title edit */}
      {editingTitle && activeConv && (
        <div className="flex items-center gap-2 px-3 py-2 border-b border-th-border bg-th-accent-bg shrink-0">
          <input
            className="flex-1 text-sm border border-th-input-border bg-th-input-bg text-th-text-primary rounded px-2 py-1"
            value={titleDraft}
            onChange={(e) => setTitleDraft(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") handleRenameTitle();
              if (e.key === "Escape") setEditingTitle(false);
            }}
            autoFocus
          />
          <button onClick={handleRenameTitle} className="text-xs bg-th-accent text-white px-2 py-1 rounded">
            保存
          </button>
          <button onClick={() => setEditingTitle(false)} className="text-xs bg-th-bg-tertiary text-th-text-secondary px-2 py-1 rounded">
            取消
          </button>
        </div>
      )}

      {/* New conversation dialog */}
      {showNewDialog && (
        <div className="p-4 border-b border-th-border bg-th-accent-bg space-y-3 shrink-0">
          <div className="text-sm font-medium text-th-text-secondary">新建会话</div>
          <input
            className="w-full text-sm border border-th-input-border bg-th-input-bg text-th-text-primary rounded px-2 py-1.5"
            placeholder="给会话起个名字（可选）"
            value={titleDraft}
            onChange={(e) => setTitleDraft(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") handleCreateConversation(titleDraft || undefined);
              if (e.key === "Escape") { setShowNewDialog(false); setTitleDraft(""); }
            }}
            autoFocus
          />
          <div className="flex justify-end gap-2">
            <button
              onClick={() => { setShowNewDialog(false); setTitleDraft(""); }}
              className="text-sm px-3 py-1.5 bg-th-bg-tertiary text-th-text-secondary rounded hover:bg-th-bg-primary"
            >
              取消
            </button>
            <button
              onClick={() => handleCreateConversation(titleDraft || undefined)}
              className="text-sm px-3 py-1.5 bg-th-accent text-white rounded hover:opacity-90"
            >
              创建
            </button>
          </div>
        </div>
      )}

      {/* Agent progress bar */}
      {agentStatus && (
        <div className="px-4 py-2 border-b border-th-border bg-th-accent-bg shrink-0">
          <div className="flex items-center gap-2 text-xs text-th-text-secondary">
            <span className="animate-pulse">🤖</span>
            <span>步骤 {agentStatus.step}/{agentStatus.maxSteps}</span>
            <div className="flex-1 h-1.5 bg-th-bg-tertiary rounded-full overflow-hidden">
              <div
                className="h-full bg-th-accent rounded-full transition-all duration-300"
                style={{ width: `${(agentStatus.step / agentStatus.maxSteps) * 100}%` }}
              />
            </div>
          </div>
        </div>
      )}

      {/* Messages */}
      <div className="flex-1 overflow-y-auto p-4 space-y-4 custom-scroll">
        {!activeConv && (
          <div className="text-center text-th-text-muted mt-10">
            <p className="text-lg mb-2">没有活动会话</p>
            <p className="text-sm">点击 + 新建一个 AI 对话</p>
          </div>
        )}
        {messages.map((msg, i) => (
          <div key={msg.id || i} className={`flex ${msg.role === "user" ? "justify-end" : "justify-start"}`}>
            <div
              className={`max-w-[85%] rounded-lg px-3 py-2 text-sm ${
                msg.role === "user"
                  ? "bg-th-user-bubble text-th-user-bubble-text"
                  : "bg-th-assistant-bubble text-th-assistant-bubble-text"
              }`}
            >
              {msg.role === "assistant" ? (
                <MarkdownContent content={msg.content} />
              ) : (
                <span className="whitespace-pre-wrap">{msg.content}</span>
              )}
              {msg.pending_actions && msg.pending_actions.length > 0 && (
                <div className="mt-2 pt-2 border-t border-th-border space-y-1">
                  <div className="text-xs text-th-text-muted font-medium">{msg.pending_actions.length} 个待确认操作</div>
                  {msg.pending_actions.map((action, j) => (
                    <div key={j} className="text-xs bg-th-accent-bg border border-th-accent p-2 rounded">
                      {action.preview}
                    </div>
                  ))}
                  <button
                    onClick={() => handleConfirm(msg.pending_actions!)}
                    className="mt-1 text-xs bg-th-success text-white px-3 py-1 rounded hover:opacity-90"
                  >
                    确认执行全部
                  </button>
                </div>
              )}
            </div>
          </div>
        ))}
        <div ref={messagesEndRef} />
      </div>

      {/* Input */}
      <div className="p-3 border-t border-th-border shrink-0">
        <div className="flex gap-2">
          <input
            type="text"
            className="flex-1 border border-th-input-border bg-th-input-bg text-th-text-primary rounded-lg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-th-accent"
            placeholder={activeConv ? "输入消息..." : "请先选择或新建会话"}
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter" && !e.shiftKey) {
                e.preventDefault();
                handleSend();
              }
            }}
            disabled={!activeConv || loading}
          />
          <button
            onClick={() => handleSend()}
            disabled={!activeConv || loading || !input.trim()}
            className="px-4 py-2 rounded-lg text-sm font-medium text-white bg-th-accent hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            发送
          </button>
        </div>
      </div>
    </div>
  );
}