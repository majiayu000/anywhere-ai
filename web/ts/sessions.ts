// 会话管理模块
// Session Management Module

import type { 
  Session, 
  CreateSessionRequest, 
  ApiResponse,
  ModuleInterface 
} from './types.js';
import { CONFIG, API_ENDPOINTS, DEFAULTS } from './config.js';

/**
 * 会话管理器类
 * 负责会话的创建、加载、选择和删除等操作
 */
export class SessionManager implements ModuleInterface {
  private sessions: Session[] = [];
  private currentSessionId: string | null = null;
  private sessionChangeHandlers: Array<(sessionId: string | null, session?: Session) => void> = [];

  /**
   * 初始化会话管理器
   */
  public async init(): Promise<void> {
    await this.loadSessions();
  }

  /**
   * 销毁会话管理器
   */
  public destroy(): void {
    this.sessions = [];
    this.currentSessionId = null;
    this.sessionChangeHandlers = [];
  }

  /**
   * 加载所有会话
   */
  public async loadSessions(): Promise<Session[]> {
    try {
      const response = await fetch(`${CONFIG.API_BASE}${API_ENDPOINTS.SESSIONS.LIST}`);
      
      if (response.ok) {
        const sessions: Session[] = await response.json();
        this.sessions = sessions;
        this.notifySessionsUpdate();
        return sessions;
      } else {
        throw new Error(`HTTP ${response.status}: ${response.statusText}`);
      }
    } catch (error) {
      console.error('加载会话失败:', error);
      this.sessions = [];
      throw error;
    }
  }

  /**
   * 创建新会话
   */
  public async createSession(tool: string = 'claude', name?: string): Promise<Session> {
    try {
      const sessionName = name || `${DEFAULTS.SESSION_NAME_PREFIX}${Date.now()}`;
      const request: CreateSessionRequest = { tool, name: sessionName };

      const response = await fetch(`${CONFIG.API_BASE}${API_ENDPOINTS.SESSIONS.CREATE}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(request)
      });

      if (response.ok) {
        const session: Session = await response.json();
        this.sessions.unshift(session); // 添加到开头
        this.notifySessionsUpdate();
        return session;
      } else {
        throw new Error(`HTTP ${response.status}: ${response.statusText}`);
      }
    } catch (error) {
      console.error('创建会话失败:', error);
      throw error;
    }
  }

  /**
   * 删除会话
   */
  public async deleteSession(sessionId: string): Promise<void> {
    try {
      const response = await fetch(
        `${CONFIG.API_BASE}${API_ENDPOINTS.SESSIONS.DELETE(sessionId)}`,
        { method: 'DELETE' }
      );

      if (response.ok) {
        this.sessions = this.sessions.filter(s => s.id !== sessionId);
        
        // 如果删除的是当前会话，清除当前会话
        if (this.currentSessionId === sessionId) {
          this.currentSessionId = null;
          this.notifySessionChange(null);
        }
        
        this.notifySessionsUpdate();
      } else {
        throw new Error(`HTTP ${response.status}: ${response.statusText}`);
      }
    } catch (error) {
      console.error('删除会话失败:', error);
      throw error;
    }
  }

  /**
   * 选择会话
   */
  public selectSession(sessionId: string): Session | null {
    const session = this.sessions.find(s => s.id === sessionId) || null;
    
    if (session) {
      this.currentSessionId = sessionId;
      this.notifySessionChange(sessionId, session);
    }
    
    return session;
  }

  /**
   * 获取当前会话
   */
  public getCurrentSession(): Session | null {
    if (!this.currentSessionId) return null;
    return this.sessions.find(s => s.id === this.currentSessionId) || null;
  }

  /**
   * 获取所有会话
   */
  public getSessions(): Session[] {
    return [...this.sessions]; // 返回副本
  }

  /**
   * 根据ID获取会话
   */
  public getSession(sessionId: string): Session | null {
    return this.sessions.find(s => s.id === sessionId) || null;
  }

  /**
   * 获取当前会话ID
   */
  public getCurrentSessionId(): string | null {
    return this.currentSessionId;
  }

  /**
   * 检查会话是否存在
   */
  public hasSession(sessionId: string): boolean {
    return this.sessions.some(s => s.id === sessionId);
  }

  /**
   * 获取活跃会话数量
   */
  public getActiveSessionCount(): number {
    return this.sessions.filter(s => s.status === 'active').length;
  }

  /**
   * 获取会话状态统计
   */
  public getSessionStats(): { total: number; active: number; inactive: number; stopped: number } {
    const stats = { total: 0, active: 0, inactive: 0, stopped: 0 };
    
    this.sessions.forEach(session => {
      stats.total++;
      switch (session.status) {
        case 'active':
          stats.active++;
          break;
        case 'inactive':
          stats.inactive++;
          break;
        case 'stopped':
          stats.stopped++;
          break;
      }
    });
    
    return stats;
  }

  /**
   * 注册会话变化监听器
   */
  public onSessionChange(handler: (sessionId: string | null, session?: Session) => void): void {
    this.sessionChangeHandlers.push(handler);
  }

  /**
   * 取消会话变化监听器
   */
  public offSessionChange(handler: (sessionId: string | null, session?: Session) => void): void {
    const index = this.sessionChangeHandlers.indexOf(handler);
    if (index > -1) {
      this.sessionChangeHandlers.splice(index, 1);
    }
  }

  /**
   * 通知会话变化
   */
  private notifySessionChange(sessionId: string | null, session?: Session): void {
    this.sessionChangeHandlers.forEach(handler => {
      try {
        handler(sessionId, session);
      } catch (error) {
        console.error('会话变化处理器执行失败:', error);
      }
    });
  }

  /**
   * 通知会话列表更新
   */
  private notifySessionsUpdate(): void {
    // 可以添加会话列表更新的特定通知逻辑
    if (CONFIG.DEBUG.MESSAGES) {
      console.log('会话列表已更新:', this.sessions.length, '个会话');
    }
  }

  /**
   * 刷新会话列表
   */
  public async refreshSessions(): Promise<void> {
    await this.loadSessions();
  }

  /**
   * 清除当前会话选择
   */
  public clearCurrentSession(): void {
    this.currentSessionId = null;
    this.notifySessionChange(null);
  }
}

// 创建全局会话管理实例
export const sessionManager = new SessionManager();