// 配置模块 - 全局配置和常量
// Configuration Module - Global settings and constants

import type { Config } from './types.js';

// 应用配置
export const CONFIG: Config = {
  // 基础API地址
  API_BASE: 'http://localhost:8080',
  
  // WebSocket配置
  WEBSOCKET: {
    URL: 'ws://localhost:8080/api/v1/ws',
    RECONNECT_DELAY: 3000, // 重连延迟（毫秒）
    MAX_RETRIES: 5 // 最大重试次数
  },
  
  // 输入法配置
  IME: {
    COMPOSITION_DELAY: 10, // IME组合状态稳定延迟
    DEBUG_ENABLED: true // 是否启用IME调试日志
  },
  
  // 自动补全配置
  AUTOCOMPLETE: {
    MAX_ITEMS: 10, // 最大显示项目数
    MIN_QUERY_LENGTH: 1 // 最小查询长度
  },
  
  // UI配置
  UI: {
    TYPING_TIMEOUT: 30000, // 输入动画超时时间（毫秒）
    SCROLL_DELAY: 50, // 滚动延迟
    BUTTON_RESET_DELAY: 1000 // 按钮重置延迟
  },
  
  // 调试配置
  DEBUG: {
    WEBSOCKET: true,
    IME: true,
    AUTOCOMPLETE: false,
    MESSAGES: false
  }
} as const;

// API端点配置
export const API_ENDPOINTS = {
  SESSIONS: {
    LIST: '/api/v1/terminal/sessions',
    CREATE: '/api/v1/terminal/sessions',
    DELETE: (id: string) => `/api/v1/terminal/sessions/${id}`,
    MESSAGES: (id: string) => `/api/v1/terminal/sessions/${id}/messages`,
    OUTPUT: (id: string) => `/api/v1/terminal/sessions/${id}/output`,
    INPUT: (id: string) => `/api/v1/terminal/sessions/${id}/input`,
    ATTACH: (id: string) => `/api/v1/terminal/sessions/${id}/attach`,
    STATUS: (id: string) => `/api/v1/terminal/sessions/${id}/messages/status`
  },
  COMMANDS: '/api/v1/claude-commands',
  WEBSOCKET: '/api/v1/ws',
  HEALTH: '/health'
} as const;

// DOM选择器配置
export const DOM_SELECTORS = {
  STATUS_DOT: '#statusDot',
  STATUS_TEXT: '#statusText',
  SESSIONS_LIST: '#sessionsList',
  EMPTY_STATE: '#emptyState',
  CHAT_INTERFACE: '#chatInterface',
  CHAT_TITLE: '#chatTitle',
  MESSAGES_CONTAINER: '#messagesContainer',
  MESSAGE_INPUT: '#messageInput',
  SEND_BTN: '#sendBtn',
  AUTOCOMPLETE_DROPDOWN: '#autocompleteDropdown',
  INPUT_AREA: '.input-area'
} as const;

// CSS类名配置
export const CSS_CLASSES = {
  CONNECTED: 'connected',
  ACTIVE: 'active',
  LOADING: 'loading',
  SHOW: 'show',
  SELECTED: 'selected',
  SESSION: 'session',
  MESSAGE: 'message',
  USER: 'user',
  AGENT: 'agent',
  TYPING_INDICATOR: 'typing-indicator',
  AUTOCOMPLETE_ITEM: 'autocomplete-item',
  AUTOCOMPLETE_COMMAND: 'autocomplete-command',
  AUTOCOMPLETE_DESCRIPTION: 'autocomplete-description'
} as const;

// 事件名称配置
export const EVENTS = {
  DOM_CONTENT_LOADED: 'DOMContentLoaded',
  CLICK: 'click',
  KEYDOWN: 'keydown',
  INPUT: 'input',
  COMPOSITION_START: 'compositionstart',
  COMPOSITION_UPDATE: 'compositionupdate',
  COMPOSITION_END: 'compositionend',
  RESIZE: 'resize',
  BEFORE_UNLOAD: 'beforeunload'
} as const;

// WebSocket动作类型
export const WS_ACTIONS = {
  SUBSCRIBE: 'subscribe',
  SELECT_SESSION: 'selectSession',
  SEND_MESSAGE: 'sendMessage',
  MESSAGES: 'messages',
  NEW_MESSAGE: 'newMessage',
  TYPING: 'typing',
  STOP_TYPING: 'stopTyping'
} as const;

// 键盘键码配置
export const KEYS = {
  ENTER: 'Enter',
  ESCAPE: 'Escape',
  ARROW_UP: 'ArrowUp',
  ARROW_DOWN: 'ArrowDown',
  TAB: 'Tab'
} as const;

// 默认值配置
export const DEFAULTS = {
  SESSION_NAME_PREFIX: 'claude-',
  EMPTY_MESSAGE: '暂无消息，开始对话吧！',
  LOADING_MESSAGE: '加载中...',
  CONNECTING_MESSAGE: '连接中...',
  CONNECTED_MESSAGE: '已连接',
  DISCONNECTED_MESSAGE: '连接断开',
  ERROR_MESSAGE: '连接错误',
  NO_SESSIONS_MESSAGE: '暂无会话',
  LOAD_FAILED_MESSAGE: '加载失败',
  TYPING_INDICATOR_TEXT: '🤖 Claude 思考中',
  INPUT_PLACEHOLDER: '输入您的消息... (输入 / 查看命令)'
} as const;