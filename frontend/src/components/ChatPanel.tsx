import { useState, useEffect, useRef, useCallback, useMemo, forwardRef, useImperativeHandle } from "react";
import type { Conversation, ConversationMessage, PendingAction, Plan } from "../types";
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
  onPlanCreated?: (plan: Plan) => void;
  focusPageId?: number | null;
  currentSlug?: string;
  currentPageTitle?: string;
}

export const ChatPanel = forwardRef<{ appendToInput: (text: string) => void }, ChatPanelProps>(
  function ChatPanel({ onPageChanged, onPlanCreated, focusPageId, currentSlug, currentPageTitle }, ref) {
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [activeConv, setActiveConv] = useState<Conversation | null>(null);
  const [messages, setMessages] = useState<ConversationMessage[]>([]);
  const [input, setInput] = useState("");
  const [loading, setLoading] = useState(false);
  const [showNewDialog, setShowNewDialog] = useState(false);
  const [showList, setShowList] = useState(false);
  const [showMenu, setShowMenu] = useState(false);
  const [editingTitle, setEditingTitle] = useState(false);
  const [titleDraft, setTitleDraft] = useState("");
  const [agentStatus, setAgentStatus] = useState<{
    step: number;
    maxSteps: number;
    running: boolean;
  } | null>(null);

  const inputRef = useRef<HTMLInputElement>(null);

  useImperativeHandle(ref, () => ({
    appendToInput(text: string) {
      setInput((prev) => {
        const newInput = prev ? prev + "\n" + text : text;
        return newInput;
      });
      setTimeout(() => {
        inputRef.current?.focus();
      }, 0);
    },
  }));

  const messagesEndRef = useRef<HTMLDivElement>(null);
  const listRef = useRef<HTMLDivElement>(null);

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

  useEffect(() => {
    if (!showList && !showMenu) return;
    const handleClickOutside = (e: MouseEvent) => {
      if (listRef.current && !listRef.current.contains(e.target as Node)) {
        setShowList(false);
        setShowMenu(false);
      }
    };
    const handleEscape = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        setShowList(false);
        setShowMenu(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    document.addEventListener('keydown', handleEscape);
    return () => {
      document.removeEventListener('mousedown', handleClickOutside);
      document.removeEventListener('keydown', handleEscape);
    };
  }, [showList, showMenu]);

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

  async function handleSend(confirmedActions?: PendingAction[], planId?: string) {
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
          plan_id: planId,
          focus_page_id: focusPageId,
          current_slug: currentSlug,
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
          // Handle plan from AI
          if (meta.plan) {
            setMessages((prev) => {
              const updated = [...prev];
              const last = updated[updated.length - 1];
              if (last && last.role === "assistant") {
                updated[updated.length - 1] = { ...last, plan: meta.plan };
              }
              return updated;
            });
            onPlanCreated?.(meta.plan);
          }
          // Keep backward compat for pending_actions
          if (meta.pending_actions) {
            setMessages((prev) => {
              const updated = [...prev];
              const last = updated[updated.length - 1];
              if (last && last.role === "assistant") {
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

  const renderedMessages = useMemo(() =>
    messages.map((msg, i) => (
      <div key={msg.id || i} className={`flex ${msg.role === "user" ? "justify-end" : "justify-start"}`}>
        <div
          className={`max-w-[85%] rounded-2xl px-4 py-2.5 text-sm shadow-sm ${
            msg.role === "user"
              ? "bg-th-user-bubble text-th-user-bubble-text rounded-tr-md"
              : "bg-th-assistant-bubble text-th-assistant-bubble-text rounded-tl-md"
          }`}
        >
          {msg.role === "assistant" ? (
            <MarkdownContent content={msg.content} />
          ) : (
            <span className="whitespace-pre-wrap">{msg.content}</span>
          )}
          {msg.plan && (
            <div className="border border-th-accent/30 bg-th-accent/5 rounded-lg p-3 mt-3">
              <div className="text-sm text-th-text">
                {"已生成操作计划："}
                {msg.plan.reasoning.length > 100
                  ? msg.plan.reasoning.slice(0, 100) + "..."
                  : msg.plan.reasoning}
              </div>
              <div className="text-xs text-th-muted mt-1">
                {msg.plan.actions.length} 个操作 · 请在右侧查看详情
              </div>

            </div>
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
    )),
    [messages, loading]
  );


  return (
    <div className="flex flex-col h-full bg-th-bg-secondary">
      {/* Header - conversation picker */}
      <div className="relative shrink-0" ref={listRef}>
        <div className="flex items-center gap-1.5 p-2 border-b border-th-border bg-th-bg-tertiary">
          <button
            onClick={() => setShowList(!showList)}
            className="flex-1 flex items-center gap-2 min-w-0 px-2 py-1.5 rounded-md hover:bg-th-bg-secondary transition-colors text-left"
          >
            <svg className="w-4 h-4 shrink-0 text-th-text-muted" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
            </svg>
            <span className="text-sm truncate text-th-text-primary font-medium">
              {activeConv?.title || '选择会话...'}
            </span>
            <svg
              className={"w-3.5 h-3.5 shrink-0 text-th-text-muted transition-transform duration-200" + (showList ? ' rotate-180' : '')}
              fill="none" stroke="currentColor" viewBox="0 0 24 24"
            >
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
            </svg>
          </button>

          {activeConv && (
            <span className="text-xs text-th-text-muted bg-th-bg-secondary px-1.5 py-0.5 rounded shrink-0 font-mono tabular-nums">
              {activeConv.message_count}
            </span>
          )}

          <button
            onClick={() => { setShowNewDialog(true); setShowList(false); }}
            className="p-1.5 text-th-text-muted hover:text-th-accent hover:bg-th-accent-bg rounded-md transition-all duration-150 active:scale-90"
            title="新建会话"
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
            </svg>
          </button>

          {activeConv && (
            <div className="relative">
              <button
                onClick={() => setShowMenu(!showMenu)}
                className={"p-1.5 rounded-md transition-all duration-150 active:scale-90 " + (showMenu ? 'text-th-accent bg-th-accent-bg' : 'text-th-text-muted hover:text-th-text-secondary hover:bg-th-bg-secondary')}
                title="更多"
              >
                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 5v.01M12 12v.01M12 19v.01M12 6a1 1 0 110-2 1 1 0 010 2zm0 7a1 1 0 110-2 1 1 0 010 2zm0 7a1 1 0 110-2 1 1 0 010 2z" />
                </svg>
              </button>

              {showMenu && (
                <div className="absolute right-0 top-full mt-1 w-36 bg-th-bg-secondary border border-th-border rounded-lg shadow-th-lg py-1 z-50">
                  <button
                    onClick={() => {
                      setTitleDraft(activeConv.title || "");
                      setEditingTitle(true);
                      setShowMenu(false);
                    }}
                    className="flex items-center gap-2 w-full px-3 py-1.5 text-sm text-th-text-primary hover:bg-th-bg-tertiary transition-colors"
                  >
                    <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                    </svg>
                    重命名
                  </button>
                  <button
                    onClick={() => {
                      setShowMenu(false);
                      handleDeleteConversation();
                    }}
                    className="flex items-center gap-2 w-full px-3 py-1.5 text-sm text-th-error hover:bg-th-bg-tertiary transition-colors"
                  >
                    <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                    </svg>
                    删除
                  </button>
                </div>
              )}
            </div>
          )}
        </div>

        {/* Conversation list dropdown */}
        <div
          className={"overflow-hidden transition-all duration-200 ease-in-out " + (showList ? 'max-h-80 border-b border-th-border' : 'max-h-0')}
        >
          <div className="bg-th-bg-secondary">
            {conversations.length === 0 ? (
              <div className="px-4 py-8 text-center text-th-text-muted text-sm">
                暂无会话
              </div>
            ) : (
              <div className="py-1 max-h-64 overflow-y-auto custom-scroll">
                {conversations.map((conv) => {
                  const isActive = conv.id === activeConv?.id;
                  return (
                    <button
                      key={conv.id}
                      onClick={() => {
                        switchToConversation(conv);
                        setShowList(false);
                      }}
                      className={"flex items-center gap-3 w-full px-4 py-2.5 text-left transition-colors " + (isActive ? 'bg-th-accent-bg' : 'hover:bg-th-bg-tertiary')}
                    >
                      <div className="flex-1 min-w-0">
                        <div className={"text-sm truncate " + (isActive ? 'text-th-accent font-medium' : 'text-th-text-primary')}>
                          {conv.title || '会话 ' + conv.id}
                        </div>
                        <div className="text-xs text-th-text-muted mt-0.5">
                          {conv.message_count} 条消息
                        </div>
                      </div>
                      {isActive && (
                        <svg className="w-4 h-4 shrink-0 text-th-accent" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                        </svg>
                      )}
                    </button>
                  );
                })}
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Title edit */}
      {editingTitle && activeConv && (
        <div className="flex items-center gap-2 px-3 py-2 border-b border-th-border bg-th-accent-bg shrink-0">
          <input
            className="flex-1 text-sm border border-th-input-border bg-th-input-bg text-th-text-primary rounded px-2 py-1"
            value={titleDraft}
            onChange={(e) => setTitleDraft(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter" && !e.nativeEvent.isComposing) handleRenameTitle();
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
              if (e.key === "Enter" && !e.nativeEvent.isComposing) handleCreateConversation(titleDraft || undefined);
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
          <div className="text-center text-th-text-muted mt-12 space-y-3">
            <svg className="w-10 h-10 mx-auto opacity-30" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
            </svg>
            <p className="text-base font-medium">没有活动会话</p>
            <p className="text-sm opacity-60">点击 + 新建一个 AI 对话</p>
          </div>
        )}
        {renderedMessages}
        <div ref={messagesEndRef} />
      </div>

      {/* Input */}
      <div className="p-3 border-t border-th-border shrink-0">
        {currentPageTitle && (
          <div className="pb-2">
            <span className="text-xs text-th-text-muted flex items-center gap-1">
              <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
              </svg>
              当前页面：{currentPageTitle}
            </span>
          </div>
        )}
        <div className="flex gap-2">
          <input
            ref={inputRef}
            type="text"
            className="flex-1 border border-th-input-border bg-th-input-bg text-th-text-primary rounded-xl px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-th-accent/40 focus:border-th-accent transition-all duration-200"
            placeholder={activeConv ? "输入消息..." : "请先选择或新建会话"}
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter" && !e.shiftKey && !e.nativeEvent.isComposing) {
                e.preventDefault();
                handleSend();
              }
            }}
            disabled={!activeConv || loading}
          />
          <button
            onClick={() => handleSend()}
            disabled={!activeConv || loading || !input.trim()}
            className="px-3 rounded-xl text-white bg-th-accent hover:opacity-90 active:scale-[0.97] disabled:opacity-50 disabled:cursor-not-allowed disabled:active:scale-100 transition-all duration-150 flex items-center justify-center"
          >
            {loading ? (
              <svg className="w-5 h-5 animate-spin" fill="none" viewBox="0 0 24 24">
                <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
              </svg>
            ) : (
              <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 12h14M12 5l7 7-7 7" />
              </svg>
            )}
          </button>
        </div>
      </div>
    </div>
  );
});