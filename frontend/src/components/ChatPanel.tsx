import { useState, useEffect, useRef, useCallback, useMemo, forwardRef, useImperativeHandle } from "react";
import type { Conversation, ConversationMessage, Plan, ToolCallInfo } from "../types";
import {
  listConversations,
  createConversation,
  updateConversationTitle,
  deleteConversation,
  getConversationMessages,
  streamChat,
} from "../lib/api";
import { MarkdownContent } from "./MarkdownContent";
import { ToolCallCard } from "./ToolCallCard";

const STORAGE_KEY = "llm-wiki-active-conversation-id";

interface ChatPanelProps {
  focusPageId?: number | null;
  currentSlug?: string;
  currentPageTitle?: string;
  onPlanReceived?: (plan: Plan) => void;
}

export const ChatPanel = forwardRef<{
	setSelectedText: (text: string, pageTitle: string) => void;
	sendMessage: (text: string) => void;
	continueAfterConfirm: () => void;
}, ChatPanelProps>(
  function ChatPanel({ focusPageId, currentSlug, currentPageTitle, onPlanReceived }, ref) {
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [activeConv, setActiveConv] = useState<Conversation | null>(null);
  const [messages, setMessages] = useState<ConversationMessage[]>([]);
  const [input, setInput] = useState("");
  const [loading, setLoading] = useState(false);
  const [showList, setShowList] = useState(false);
  const [showMenu, setShowMenu] = useState(false);
  const [editingTitle, setEditingTitle] = useState(false);
  const [titleDraft, setTitleDraft] = useState("");
  const [streamError, setStreamError] = useState<string | null>(null);
  const [streamingToolCalls, setStreamingToolCalls] = useState<Map<string, ToolCallInfo>>(new Map());

  const inputRef = useRef<HTMLInputElement>(null);
  const prevLoadingRef = useRef(false);

  const [selectedText, setSelectedText] = useState<string | null>(null);
  const [selectedTextPage, setSelectedTextPage] = useState<string | null>(null);

  useImperativeHandle(ref, () => ({
    setSelectedText(text: string, pageTitle: string) {
      setSelectedText(text);
      setSelectedTextPage(pageTitle);
      setTimeout(() => {
        inputRef.current?.focus();
      }, 0);
    },
    sendMessage(text: string) {
      handleSend(undefined, text);
    },
    async continueAfterConfirm() {
      if (loading) return;
      if (!activeConv) {
        const conv = await handleCreateConversation();
        if (!conv) return;
      }
      setStreamError(null);
      setLoading(true);
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
        const toolCallAccum = new Map<string, ToolCallInfo>();

        await streamChat(
          {
            conversation_id: activeConv!.id,
            message: "",
            focus_page_id: focusPageId,
            current_slug: currentSlug,
          },
          (chunk) => {
            fullContent += chunk;
            setMessages((prev) => {
              const msgs = [...prev];
              const last = msgs[msgs.length - 1];
              if (last && last.role === "assistant") {
                msgs[msgs.length - 1] = { ...last, content: fullContent };
              }
              return msgs;
            });
          },
          (meta) => {
            if (meta.conversation_id && meta.conversation_id !== activeConv!.id) {
              newConvId = meta.conversation_id;
            }
            if (meta.plan) {
              onPlanReceived?.(meta.plan);
            }
          },
          undefined,
          (error) => {
            setStreamError(error);
            setLoading(false);
          },
          (tc) => {
            toolCallAccum.set(tc.id, tc);
            setStreamingToolCalls(new Map(toolCallAccum));
          },
        );

        if (newConvId && newConvId !== activeConv!.id) {
          await loadConversations();
          const newConv = conversations.find((c) => c.id === newConvId);
          if (newConv) {
            await switchToConversation(newConv);
          }
        }
      } catch (e) {
        console.error("Continuation chat error:", e);
        setMessages((prev) => {
          const msgs = [...prev];
          const last = msgs[msgs.length - 1];
          if (last && last.role === "assistant" && !last.content) {
            msgs.pop();
          }
          return msgs;
        });
      } finally {
        setLoading(false);
        loadConversations();
      }
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
    if (prevLoadingRef.current && !loading) {
      inputRef.current?.focus();
    }
    prevLoadingRef.current = loading;
  }, [loading]);

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

  async function handleCreateConversation() {
    try {
      const conv = await createConversation();
      await loadConversations();
      await switchToConversation(conv);
      return conv;
    } catch (e) {
      console.error("Failed to create conversation:", e);
      return null;
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

  async function handleSend(planId?: string, messageOverride?: string, skipEmptyCheck?: boolean) {
    if (loading) return;

    const userContent = (messageOverride ?? input).trim();
    if (!userContent && !skipEmptyCheck) return;

    setStreamError(null);

    // Auto-create conversation if none active
    let conv = activeConv;
    if (!conv) {
      conv = await handleCreateConversation();
      if (!conv) return;
    }

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
      if (!messageOverride) setInput("");
    }

    setLoading(true);

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
      const toolCallAccum = new Map<string, ToolCallInfo>();

      await streamChat(
        {
          conversation_id: conv.id,
          message: userContent,
          plan_id: planId,
          focus_page_id: focusPageId,
          current_slug: currentSlug,
          selected_text: selectedText ?? undefined,
        },
        (chunk) => {
          fullContent += chunk;
          setMessages((prev) => {
            const msgs = [...prev];
            const last = msgs[msgs.length - 1];
            if (last && last.role === "assistant") {
              msgs[msgs.length - 1] = { ...last, content: fullContent };
            }
            return msgs;
          });
        },
        (meta) => {
          if (meta.conversation_id && meta.conversation_id !== conv.id) {
            newConvId = meta.conversation_id;
          }
          if (meta.plan) {
            onPlanReceived?.(meta.plan);
            // Replace the assistant message with a summary pointing to the right panel
            if (meta.plan.calibration_question) {
              const cq = meta.plan.calibration_question;
              const optionsText = cq.options ? cq.options.map((o, i) => `${i + 1}. ${o}`).join('\n') : '';
              const summary = cq.question
                ? `## ❓ 校准问题

${cq.question}

${optionsText ? '选项：\n' + optionsText + '\n\n请在右侧面板中选择，或直接在聊天中回复。' : '请在右侧面板或聊天中回复。'}`
                : '## 校准问题\n\n请在右侧面板中查看并回答。';
              setMessages((prev) => {
                const msgs = [...prev];
                const last = msgs[msgs.length - 1];
                if (last && last.role === "assistant") {
                  msgs[msgs.length - 1] = { ...last, content: summary };
                }
                return msgs;
              });
            } else if ((meta.plan.outline && meta.plan.outline.length > 0)) {
              const summary = meta.plan.reasoning
                ? `## 📋 知识大纲

${meta.plan.reasoning}

大纲已生成，请在右侧面板中查看并确认。`
                : '## 📋 知识大纲\n\n大纲已生成，请在右侧面板中查看并确认。';
              setMessages((prev) => {
                const msgs = [...prev];
                const last = msgs[msgs.length - 1];
                if (last && last.role === "assistant") {
                  msgs[msgs.length - 1] = { ...last, content: summary };
                }
                return msgs;
              });
            } else if (meta.plan.actions && meta.plan.actions.length > 0) {
              const summary = meta.plan.reasoning
                ? `## 📋 操作计划

${meta.plan.reasoning}

共 ${meta.plan.actions.length} 个操作待确认，请在右侧面板中查看。`
                : `## 📋 操作计划\n\n共 ${meta.plan.actions.length} 个操作待确认，请在右侧面板中查看。`;
              setMessages((prev) => {
                const msgs = [...prev];
                const last = msgs[msgs.length - 1];
                if (last && last.role === "assistant") {
                  msgs[msgs.length - 1] = { ...last, content: summary };
                }
                return msgs;
              });
            }
          }
        },
        undefined, /* onStatus — not used */
        (error) => {
          setStreamError(error);
          setLoading(false);
        },
        (tc) => {
          toolCallAccum.set(tc.id, tc);
          setStreamingToolCalls(new Map(toolCallAccum));
        }
      );

      // If API created a new conversation (e.g. auto-named), switch to it
      if (newConvId && newConvId !== conv.id) {
        await loadConversations();
        const newConv = conversations.find((c) => c.id === newConvId);
        if (newConv) {
          await switchToConversation(newConv);
        }
      }

      setSelectedText(null);
      setSelectedTextPage(null);

      // Persist streamed tool calls to the last assistant message
      if (toolCallAccum.size > 0) {
        const calls = Array.from(toolCallAccum.values()).filter(
          (tc) => tc.output || tc.error
        );
        if (calls.length > 0) {
          setMessages((prev) => {
            const msgs = [...prev];
            const last = msgs[msgs.length - 1];
            if (last && last.role === "assistant") {
              msgs[msgs.length - 1] = { ...last, tool_calls: calls };
            }
            return msgs;
          });
        }
        setStreamingToolCalls(new Map());
      }

      // Clean up empty assistant message (model returned no content and no tool calls)
      setMessages((prev) => {
        const msgs = [...prev];
        const last = msgs[msgs.length - 1];
        if (last && last.role === "assistant" && !last.content && (!last.tool_calls || last.tool_calls.length === 0)) {
          msgs.pop();
        }
        return msgs;
      });
    } catch (e) {
      console.error("Chat error:", e);
      setMessages((prev) => {
        const msgs = [...prev];
        const last = msgs[msgs.length - 1];
        if (last && last.role === "assistant" && !last.content) {
          msgs.pop();
        }
        return msgs;
      });
    } finally {
      setLoading(false);
      // Refresh conversation list to pick up any auto-generated title
      loadConversations();
    }
  }

  const renderedMessages = useMemo(() => {
    if (messages.length === 0) return null;

    return messages.map((msg, i) => {
      const isLast = i === messages.length - 1;

      return (
        <div key={msg.id} className={`flex ${msg.role === "user" ? "justify-end" : "justify-start"} animate-fade-in`}>
          {msg.role === "assistant" && (
            <div className="w-6 h-6 rounded-full bg-th-accent-bg flex items-center justify-center mr-2 shrink-0 mt-0.5">
              <span className="font-display text-[11px] font-bold text-th-accent leading-none">L</span>
            </div>
          )}
          <div
            className={`max-w-[85%] ${
              msg.role === "user"
                ? "bg-th-user-bubble text-th-user-bubble-text rounded-2xl rounded-br-md px-3.5 py-2 shadow-th"
                : "text-th-text-primary"
            }`}
          >
            {msg.role === "user" ? (
              <p className="text-[14px] leading-relaxed whitespace-pre-wrap break-words">{msg.content}</p>
            ) : (
              <div className="min-w-0">
                {msg.content ? (
                  <MarkdownContent content={msg.content} compact />
                ) : isLast && loading ? (
                  <div className="flex items-center gap-1 py-1 text-th-text-muted">
                    <span className="block w-1 h-3.5 bg-th-accent rounded-sm animate-cursor-scan" />
                    <span className="text-xs italic">正在思考</span>
                  </div>
                ) : null}
                {/* Tool calls from persisted messages */}
                {msg.tool_calls?.map((tc) => (
                  <ToolCallCard key={tc.id} toolCall={tc} defaultExpanded={false} />
                ))}
                {/* Streaming tool calls for the last message */}
                {isLast && loading && Array.from(streamingToolCalls.values()).map((tc) => (
                  <ToolCallCard key={tc.id} toolCall={tc} defaultExpanded={false} />
                ))}
              </div>
            )}
          </div>
        </div>
      );
    });
  }, [messages, loading, streamingToolCalls]);

  return (
    <div className="h-full flex flex-col bg-th-bg-primary">
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
            onClick={() => { handleCreateConversation(); setShowList(false); }}
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
                className="p-1.5 text-th-text-muted hover:text-th-accent hover:bg-th-accent-bg rounded-md transition-all duration-150 active:scale-90"
                title="更多"
              >
                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 5v.01M12 12v.01M12 19v.01" />
                </svg>
              </button>
              {showMenu && (
                <div className="absolute right-0 top-full mt-1 w-56 bg-th-bg-secondary border border-th-border rounded-lg shadow-th-lg z-20 py-1">
                  <button
                    onClick={() => { setEditingTitle(true); setTitleDraft(activeConv?.title || ""); setShowMenu(false); }}
                    className="w-full text-left px-3 py-2 text-sm text-th-text-primary hover:bg-th-bg-tertiary flex items-center gap-2"
                  >
                    <svg className="w-4 h-4 text-th-text-muted" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                    </svg>
                    重命名
                  </button>
                  <button
                    onClick={() => { handleDeleteConversation(); setShowMenu(false); }}
                    className="w-full text-left px-3 py-2 text-sm text-th-danger hover:bg-th-danger-bg flex items-center gap-2"
                  >
                    <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                    </svg>
                    删除会话
                  </button>
                </div>
              )}
            </div>
          )}
        </div>

        {/* Conversation list dropdown */}
        <div className={"overflow-hidden transition-all duration-200 ease-in-out " + (showList ? 'max-h-80 border-b border-th-border' : 'max-h-0')}>
          <div className="max-h-80 overflow-y-auto bg-th-bg-secondary">
            {conversations.length === 0 && (
              <div className="px-4 py-6 text-center text-sm text-th-text-muted">
                暂无会话
              </div>
            )}
            {conversations.map((conv) => (
              <button
                key={conv.id}
                onClick={() => { switchToConversation(conv); setShowList(false); }}
                className={"w-full text-left px-4 py-2.5 border-b border-th-border/50 last:border-b-0 hover:bg-th-bg-tertiary transition-colors " + (activeConv?.id === conv.id ? "bg-th-accent-bg" : "")}
              >
                <div className="text-sm font-medium text-th-text-primary truncate">{conv.title || '无标题'}</div>
                <div className="text-xs text-th-text-muted mt-0.5">
                  {conv.message_count} 条消息 · {new Date(conv.updated_at).toLocaleDateString('zh-CN')}
                </div>
              </button>
            ))}
          </div>
        </div>

        {/* Inline rename */}
        {editingTitle && (
          <div className="px-4 py-2 border-b border-th-border bg-th-accent-bg space-y-2 shrink-0">
            <div className="text-sm font-medium text-th-text-secondary">重命名会话</div>
            <input
              className="w-full text-sm border border-th-input-border bg-th-input-bg text-th-text-primary rounded px-2 py-1.5"
              placeholder="输入新名称"
              value={titleDraft}
              onChange={(e) => setTitleDraft(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter" && !e.nativeEvent.isComposing) handleRenameTitle();
                if (e.key === "Escape") { setEditingTitle(false); setTitleDraft(""); }
              }}
              autoFocus
            />
            <div className="flex justify-end gap-2">
              <button
                onClick={() => { setEditingTitle(false); setTitleDraft(""); }}
                className="text-sm px-3 py-1.5 bg-th-bg-tertiary text-th-text-secondary rounded hover:bg-th-bg-primary"
              >
                取消
              </button>
              <button
                onClick={handleRenameTitle}
                className="text-sm px-3 py-1.5 bg-th-accent text-white rounded hover:opacity-90"
              >
                保存
              </button>
            </div>
          </div>
        )}
      </div>

      {/* Messages */}
      <div className="flex-1 overflow-y-auto p-4 space-y-4 custom-scroll">
        {!activeConv && messages.length === 0 && (
          <div className="text-center text-th-text-muted mt-12 space-y-3">
            <svg className="w-10 h-10 mx-auto opacity-30" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
            </svg>
            <p className="text-base font-medium">开始对话</p>
            <p className="text-sm opacity-60">输入消息即可自动创建新会话</p>
          </div>
        )}
        {renderedMessages}
        <div ref={messagesEndRef} />
      </div>

      {streamError && (
        <div className="mx-4 mb-2 p-2.5 bg-th-error-bg border border-th-error/20 rounded-md text-xs text-th-error flex items-start gap-2 animate-fade-in">
          <span className="flex-1">{streamError}</span>
          <button onClick={() => setStreamError(null)} className="underline shrink-0">关闭</button>
        </div>
      )}

      
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
        {selectedText && (
          <div className="pb-2 flex items-center gap-1.5">
            <span className="text-xs text-th-text-muted flex items-center gap-1">
              <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1" />
              </svg>
              已引用选中文本{selectedTextPage && <span> · {selectedTextPage}</span>}
            </span>
            <button
              onClick={() => { setSelectedText(null); setSelectedTextPage(null); }}
              className="text-xs text-th-text-muted hover:text-th-text-primary ml-1"
            >
              × 移除
            </button>
          </div>
        )}
        <div className="flex gap-2">
          <input
            ref={inputRef}
            type="text"
            className="flex-1 border border-th-input-border bg-th-input-bg text-th-text-primary rounded-xl px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-th-accent/40 focus:border-th-accent transition-all duration-200"
            placeholder={activeConv ? "输入消息..." : "输入消息，自动新建会话..."}
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter" && !e.shiftKey && !e.nativeEvent.isComposing) {
                e.preventDefault();
                handleSend();
              }
            }}
            disabled={loading}
          />
          <button
            onClick={() => handleSend()}
            disabled={loading || !input.trim()}
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
