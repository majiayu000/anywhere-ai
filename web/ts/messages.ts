// 消息处理模块
// Message Processing Module

import type { 
  Message, 
  MessageSenderType,
  ModuleInterface 
} from './types.js';
import { CONFIG } from './config.js';

/**
 * 消息管理器类
 * 负责消息的显示、格式化和状态管理
 */
export class MessageManager implements ModuleInterface {
  private messages: Message[] = [];
  private isTyping: boolean = false;
  private typingTimeout: number | null = null;
  private messageUpdateHandlers: Array<() => void> = [];
  private typingHandlers: Array<(isTyping: boolean) => void> = [];

  /**
   * 初始化消息管理器
   */
  public init(): void {
    // 初始化完成
  }

  /**
   * 销毁消息管理器
   */
  public destroy(): void {
    this.clearMessages();
    this.hideTypingIndicator();
    this.messageUpdateHandlers = [];
    this.typingHandlers = [];
  }

  /**
   * 设置消息列表
   */
  public setMessages(messages: Message[]): void {
    this.messages = [...messages];
    this.notifyMessageUpdate();
  }

  /**
   * 添加消息
   */
  public addMessage(message: Message): void {
    this.messages.push(message);
    
    // 如果收到Agent消息，隐藏输入指示器
    if (message.sender_type === 'AGENT') {
      this.hideTypingIndicator();
    }
    
    this.notifyMessageUpdate();
  }

  /**
   * 添加多条消息
   */
  public addMessages(messages: Message[]): void {
    this.messages.push(...messages);
    this.notifyMessageUpdate();
  }

  /**
   * 获取所有消息
   */
  public getMessages(): Message[] {
    return [...this.messages]; // 返回副本
  }

  /**
   * 清空消息
   */
  public clearMessages(): void {
    this.messages = [];
    this.hideTypingIndicator();
    this.notifyMessageUpdate();
  }

  /**
   * 获取消息数量
   */
  public getMessageCount(): number {
    return this.messages.length;
  }

  /**
   * 根据发送者类型获取消息
   */
  public getMessagesBySender(senderType: MessageSenderType): Message[] {
    return this.messages.filter(msg => msg.sender_type === senderType);
  }

  /**
   * 获取最后一条消息
   */
  public getLastMessage(): Message | null {
    return this.messages.length > 0 ? this.messages[this.messages.length - 1] : null;
  }

  /**
   * 显示输入指示器
   */
  public showTypingIndicator(): void {
    if (!this.isTyping) {
      this.isTyping = true;
      this.notifyTypingChange(true);
      this.notifyMessageUpdate();
      
      // 设置超时自动隐藏
      this.typingTimeout = window.setTimeout(() => {
        this.hideTypingIndicator();
      }, CONFIG.UI.TYPING_TIMEOUT);
    }
  }

  /**
   * 隐藏输入指示器
   */
  public hideTypingIndicator(): void {
    if (this.isTyping) {
      this.isTyping = false;
      if (this.typingTimeout !== null) {
        clearTimeout(this.typingTimeout);
        this.typingTimeout = null;
      }
      this.notifyTypingChange(false);
      this.notifyMessageUpdate();
    }
  }

  /**
   * 获取输入指示器状态
   */
  public getTypingState(): boolean {
    return this.isTyping;
  }

  /**
   * 格式化消息内容
   */
  public formatMessage(content: string): string {
    // HTML转义
    let formatted = content
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;');
    
    // 代码块处理
    formatted = formatted.replace(/```([\s\S]*?)```/g, '<pre>$1</pre>');
    
    // 行内代码处理
    formatted = formatted.replace(/`([^`]+)`/g, '<code>$1</code>');
    
    // 换行处理
    formatted = formatted.replace(/\n/g, '<br>');
    
    // 链接处理（简单实现）
    formatted = formatted.replace(
      /(https?:\/\/[^\s]+)/g, 
      '<a href="$1" target="_blank" rel="noopener noreferrer">$1</a>'
    );
    
    return formatted;
  }

  /**
   * 获取消息发送时间的格式化字符串
   */
  public formatMessageTime(createdAt: string): string {
    try {
      const date = new Date(createdAt);
      return date.toLocaleTimeString('zh-CN', {
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit'
      });
    } catch (error) {
      console.error('时间格式化失败:', error);
      return '';
    }
  }

  /**
   * 生成消息HTML
   */
  public generateMessageHTML(message: Message): string {
    const isAgent = message.sender_type === 'AGENT';
    const time = this.formatMessageTime(message.created_at);
    const formattedContent = this.formatMessage(message.content);
    
    return `
      <div class="message ${isAgent ? 'agent' : 'user'}">
        <div class="avatar">
          ${isAgent ? '🤖' : '👤'}
        </div>
        <div class="content">
          <div class="sender">
            ${isAgent ? 'Claude' : '我'}
            <span class="time">${time}</span>
          </div>
          <div class="text">${formattedContent}</div>
        </div>
      </div>
    `;
  }

  /**
   * 生成输入指示器HTML
   */
  public generateTypingIndicatorHTML(): string {
    return `
      <div class="typing-indicator">
        <div class="avatar">🤖</div>
        <div class="typing-dots"></div>
      </div>
    `;
  }

  /**
   * 生成消息列表HTML
   */
  public generateMessagesHTML(): string {
    if (this.messages.length === 0 && !this.isTyping) {
      return `<div class="loading">${CONFIG.DEFAULTS.EMPTY_MESSAGE}</div>`;
    }

    let html = this.messages.map(msg => this.generateMessageHTML(msg)).join('');
    
    if (this.isTyping) {
      html += this.generateTypingIndicatorHTML();
    }
    
    return html;
  }

  /**
   * 检查消息是否需要用户输入
   */
  public hasMessageRequiringInput(): boolean {
    return this.messages.some(msg => msg.requires_user_input);
  }

  /**
   * 获取需要用户输入的消息
   */
  public getMessagesRequiringInput(): Message[] {
    return this.messages.filter(msg => msg.requires_user_input);
  }

  /**
   * 注册消息更新监听器
   */
  public onMessageUpdate(handler: () => void): void {
    this.messageUpdateHandlers.push(handler);
  }

  /**
   * 取消消息更新监听器
   */
  public offMessageUpdate(handler: () => void): void {
    const index = this.messageUpdateHandlers.indexOf(handler);
    if (index > -1) {
      this.messageUpdateHandlers.splice(index, 1);
    }
  }

  /**
   * 注册输入状态监听器
   */
  public onTypingChange(handler: (isTyping: boolean) => void): void {
    this.typingHandlers.push(handler);
  }

  /**
   * 取消输入状态监听器
   */
  public offTypingChange(handler: (isTyping: boolean) => void): void {
    const index = this.typingHandlers.indexOf(handler);
    if (index > -1) {
      this.typingHandlers.splice(index, 1);
    }
  }

  /**
   * 通知消息更新
   */
  private notifyMessageUpdate(): void {
    this.messageUpdateHandlers.forEach(handler => {
      try {
        handler();
      } catch (error) {
        console.error('消息更新处理器执行失败:', error);
      }
    });
  }

  /**
   * 通知输入状态变化
   */
  private notifyTypingChange(isTyping: boolean): void {
    this.typingHandlers.forEach(handler => {
      try {
        handler(isTyping);
      } catch (error) {
        console.error('输入状态处理器执行失败:', error);
      }
    });
  }

  /**
   * 获取消息统计信息
   */
  public getMessageStats(): { total: number; user: number; agent: number } {
    const stats = { total: 0, user: 0, agent: 0 };
    
    this.messages.forEach(message => {
      stats.total++;
      if (message.sender_type === 'USER') {
        stats.user++;
      } else if (message.sender_type === 'AGENT') {
        stats.agent++;
      }
    });
    
    return stats;
  }
}

// 创建全局消息管理实例
export const messageManager = new MessageManager();