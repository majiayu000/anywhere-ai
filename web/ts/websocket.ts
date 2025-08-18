// WebSocket通信模块
// WebSocket Communication Module

import type { 
  WebSocketMessage, 
  ConnectionState, 
  ConnectionHandler, 
  MessageHandler,
  ModuleInterface 
} from './types.js';
import { CONFIG, WS_ACTIONS } from './config.js';

/**
 * WebSocket管理器类
 * 负责WebSocket连接、重连、消息处理等功能
 */
export class WebSocketManager implements ModuleInterface {
  private ws: WebSocket | null = null;
  private isConnected: boolean = false;
  private reconnectCount: number = 0;
  private messageHandlers = new Map<string, MessageHandler[]>();
  private connectionHandlers: ConnectionHandler[] = [];
  private reconnectTimer: number | null = null;

  /**
   * 初始化WebSocket管理器
   */
  public init(): void {
    this.connect();
  }

  /**
   * 销毁WebSocket管理器
   */
  public destroy(): void {
    this.disconnect();
    this.messageHandlers.clear();
    this.connectionHandlers = [];
    if (this.reconnectTimer !== null) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
  }

  /**
   * 连接WebSocket
   */
  public connect(): void {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      if (CONFIG.DEBUG.WEBSOCKET) {
        console.log('WebSocket已连接，跳过重复连接');
      }
      return;
    }

    try {
      this.ws = new WebSocket(CONFIG.WEBSOCKET.URL);
      this.setupEventHandlers();
    } catch (error) {
      console.error('WebSocket连接失败:', error);
      this.handleReconnect();
    }
  }

  /**
   * 设置WebSocket事件处理器
   */
  private setupEventHandlers(): void {
    if (!this.ws) return;

    this.ws.onopen = () => {
      if (CONFIG.DEBUG.WEBSOCKET) {
        console.log('WebSocket连接成功');
      }
      this.isConnected = true;
      this.reconnectCount = 0;
      this.notifyConnectionHandlers(true, '已连接');
    };

    this.ws.onmessage = (event: MessageEvent) => {
      try {
        const data: WebSocketMessage = JSON.parse(event.data);
        this.handleMessage(data);
      } catch (error) {
        console.error('WebSocket消息解析失败:', error);
      }
    };

    this.ws.onclose = (event: CloseEvent) => {
      if (CONFIG.DEBUG.WEBSOCKET) {
        console.log('WebSocket连接断开', event);
      }
      this.isConnected = false;
      this.notifyConnectionHandlers(false, '连接断开');
      
      // 只有在非正常关闭时才重连
      if (!event.wasClean) {
        this.handleReconnect();
      }
    };

    this.ws.onerror = (error: Event) => {
      console.error('WebSocket错误:', error);
      this.isConnected = false;
      this.notifyConnectionHandlers(false, '连接错误');
    };
  }

  /**
   * 处理重连逻辑
   */
  private handleReconnect(): void {
    if (this.reconnectCount >= CONFIG.WEBSOCKET.MAX_RETRIES) {
      console.error('WebSocket重连次数达到上限，停止重连');
      return;
    }

    this.reconnectCount++;
    if (CONFIG.DEBUG.WEBSOCKET) {
      console.log(
        `WebSocket将在${CONFIG.WEBSOCKET.RECONNECT_DELAY}ms后重连 ` +
        `(${this.reconnectCount}/${CONFIG.WEBSOCKET.MAX_RETRIES})`
      );
    }
    
    this.reconnectTimer = window.setTimeout(() => {
      this.connect();
    }, CONFIG.WEBSOCKET.RECONNECT_DELAY);
  }

  /**
   * 发送消息
   */
  public send(data: WebSocketMessage | string): boolean {
    if (!this.isConnected || !this.ws || this.ws.readyState !== WebSocket.OPEN) {
      console.error('WebSocket未连接，无法发送消息');
      return false;
    }

    try {
      const message = typeof data === 'string' ? data : JSON.stringify(data);
      this.ws.send(message);
      
      if (CONFIG.DEBUG.WEBSOCKET) {
        console.log('WebSocket发送数据:', data);
      }
      return true;
    } catch (error) {
      console.error('WebSocket发送消息失败:', error);
      return false;
    }
  }

  /**
   * 订阅会话
   */
  public subscribeSession(sessionId: string): boolean {
    return this.send({
      action: WS_ACTIONS.SUBSCRIBE,
      sessionId
    });
  }

  /**
   * 选择会话
   */
  public selectSession(sessionId: string): boolean {
    return this.send({
      action: WS_ACTIONS.SELECT_SESSION,
      sessionId
    });
  }

  /**
   * 发送消息到会话
   */
  public sendMessageToSession(sessionId: string, input: string): boolean {
    return this.send({
      action: WS_ACTIONS.SEND_MESSAGE,
      sessionId,
      input,
      timestamp: Date.now()
    });
  }

  /**
   * 注册消息处理器
   */
  public onMessage(action: string, handler: MessageHandler): void {
    if (!this.messageHandlers.has(action)) {
      this.messageHandlers.set(action, []);
    }
    this.messageHandlers.get(action)!.push(handler);
  }

  /**
   * 取消消息处理器
   */
  public offMessage(action: string, handler: MessageHandler): void {
    if (this.messageHandlers.has(action)) {
      const handlers = this.messageHandlers.get(action)!;
      const index = handlers.indexOf(handler);
      if (index > -1) {
        handlers.splice(index, 1);
      }
    }
  }

  /**
   * 注册连接状态处理器
   */
  public onConnectionChange(handler: ConnectionHandler): void {
    this.connectionHandlers.push(handler);
  }

  /**
   * 取消连接状态处理器
   */
  public offConnectionChange(handler: ConnectionHandler): void {
    const index = this.connectionHandlers.indexOf(handler);
    if (index > -1) {
      this.connectionHandlers.splice(index, 1);
    }
  }

  /**
   * 处理收到的消息
   */
  private handleMessage(data: WebSocketMessage): void {
    if (CONFIG.DEBUG.WEBSOCKET) {
      console.log('WebSocket收到消息:', data);
    }
    
    if (data.action && this.messageHandlers.has(data.action)) {
      const handlers = this.messageHandlers.get(data.action)!;
      handlers.forEach(handler => {
        try {
          handler(data);
        } catch (error) {
          console.error(`处理WebSocket消息失败 (${data.action}):`, error);
        }
      });
    }
  }

  /**
   * 通知连接状态变化
   */
  private notifyConnectionHandlers(connected: boolean, message: string): void {
    this.connectionHandlers.forEach(handler => {
      try {
        handler(connected, message);
      } catch (error) {
        console.error('连接状态处理器执行失败:', error);
      }
    });
  }

  /**
   * 断开连接
   */
  public disconnect(): void {
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
    this.isConnected = false;
  }

  /**
   * 获取连接状态
   */
  public getConnectionState(): ConnectionState {
    return {
      isConnected: this.isConnected,
      readyState: this.ws ? this.ws.readyState : WebSocket.CLOSED,
      reconnectCount: this.reconnectCount
    };
  }

  /**
   * 检查是否已连接
   */
  public get connected(): boolean {
    return this.isConnected && this.ws !== null && this.ws.readyState === WebSocket.OPEN;
  }
}

// 创建全局WebSocket管理实例
export const wsManager = new WebSocketManager();