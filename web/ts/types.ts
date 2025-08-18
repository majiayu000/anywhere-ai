// 类型定义文件 - 全局类型和接口
// Type Definitions - Global types and interfaces

// 基础配置类型
export interface Config {
  readonly API_BASE: string;
  readonly WEBSOCKET: {
    readonly URL: string;
    readonly RECONNECT_DELAY: number;
    readonly MAX_RETRIES: number;
  };
  readonly IME: {
    readonly COMPOSITION_DELAY: number;
    readonly DEBUG_ENABLED: boolean;
  };
  readonly AUTOCOMPLETE: {
    readonly MAX_ITEMS: number;
    readonly MIN_QUERY_LENGTH: number;
  };
  readonly UI: {
    readonly TYPING_TIMEOUT: number;
    readonly SCROLL_DELAY: number;
    readonly BUTTON_RESET_DELAY: number;
  };
  readonly DEBUG: {
    readonly WEBSOCKET: boolean;
    readonly IME: boolean;
    readonly AUTOCOMPLETE: boolean;
    readonly MESSAGES: boolean;
  };
}

// 会话相关类型
export interface Session {
  id: string;
  name: string;
  tool: string;
  status: 'active' | 'inactive' | 'stopped';
  created_at: string;
  updated_at: string;
}

export interface CreateSessionRequest {
  tool: string;
  name: string;
}

// 消息相关类型
export type MessageSenderType = 'USER' | 'AGENT';

export interface Message {
  id: string;
  session_id: string;
  sender_type: MessageSenderType;
  content: string;
  requires_user_input: boolean;
  created_at: string;
  updated_at: string;
}

export interface SendMessageRequest {
  action: 'sendMessage';
  sessionId: string;
  input: string;
  timestamp: number;
}

// WebSocket消息类型
export interface WebSocketMessage {
  action: string;
  sessionId?: string;
  data?: any;
  error?: string;
  timestamp?: number;
}

export interface WebSocketMessageHandlers {
  messages: (data: WebSocketMessage) => void;
  newMessage: (data: WebSocketMessage) => void;
  typing: (data: WebSocketMessage) => void;
  stopTyping: (data: WebSocketMessage) => void;
}

// 连接状态类型
export interface ConnectionState {
  isConnected: boolean;
  readyState: number;
  reconnectCount: number;
}

export type ConnectionHandler = (connected: boolean, message: string) => void;
export type MessageHandler = (data: WebSocketMessage) => void;

// Claude命令相关类型
export interface ClaudeCommand {
  command: string;
  description: string;
}

export interface ClaudeCommandsResponse {
  commands: ClaudeCommand[];
  source: 'api' | 'fallback';
}

// IME相关类型
export interface IMEState {
  isComposing: boolean;
  lastCompositionEnd: number;
  compositionData: string;
}

export interface IMEEventHandlers {
  compositionStart: (event: CompositionEvent) => void;
  compositionUpdate: (event: CompositionEvent) => void;
  compositionEnd: (event: CompositionEvent) => void;
}

// 自动补全相关类型
export interface AutocompleteState {
  isVisible: boolean;
  selectedIndex: number;
  filteredCommands: ClaudeCommand[];
  query: string;
}

export interface AutocompleteEventHandlers {
  show: (commands: ClaudeCommand[], query: string) => void;
  hide: () => void;
  select: (command: string) => void;
  navigate: (direction: 'up' | 'down') => void;
}

// UI工具相关类型
export interface UIElements {
  statusDot: HTMLElement;
  statusText: HTMLElement;
  sessionsList: HTMLElement;
  emptyState: HTMLElement;
  chatInterface: HTMLElement;
  chatTitle: HTMLElement;
  messagesContainer: HTMLElement;
  messageInput: HTMLTextAreaElement;
  sendBtn: HTMLButtonElement;
  autocompleteDropdown: HTMLElement;
}

// 应用状态类型
export interface AppState {
  currentSessionId: string | null;
  sessions: Session[];
  messages: Message[];
  isTyping: boolean;
  typingTimeout: number | null;
  imeState: IMEState;
  autocompleteState: AutocompleteState;
}

// 事件类型
export interface AppEventHandlers {
  sessionSelect: (sessionId: string) => void;
  messagesSend: (message: string) => void;
  messagesReceive: (message: Message) => void;
  connectionChange: (connected: boolean, status: string) => void;
}

// API响应类型
export interface ApiResponse<T = any> {
  success: boolean;
  data?: T;
  error?: string;
  message?: string;
}

// 键盘事件相关类型
export interface KeyboardEventHandler {
  (event: KeyboardEvent): boolean | void;
}

export interface KeyboardHandlers {
  keyPress: KeyboardEventHandler;
  input: KeyboardEventHandler;
  autocompleteNavigation: KeyboardEventHandler;
}

// 滚动和UI动画类型
export interface ScrollOptions {
  behavior: 'auto' | 'smooth';
  block?: 'start' | 'center' | 'end' | 'nearest';
  inline?: 'start' | 'center' | 'end' | 'nearest';
}

// 错误处理类型
export class AppError extends Error {
  constructor(
    message: string,
    public code: string,
    public context?: any
  ) {
    super(message);
    this.name = 'AppError';
  }
}

export interface ErrorHandler {
  (error: Error | AppError): void;
}

// 日志记录类型
export type LogLevel = 'debug' | 'info' | 'warn' | 'error';

export interface Logger {
  debug: (message: string, ...args: any[]) => void;
  info: (message: string, ...args: any[]) => void;
  warn: (message: string, ...args: any[]) => void;
  error: (message: string, ...args: any[]) => void;
}

// 模块导出接口
export interface ModuleInterface {
  init(): void | Promise<void>;
  destroy?(): void | Promise<void>;
}