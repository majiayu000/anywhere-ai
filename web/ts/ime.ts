// 输入法处理模块
// IME (Input Method Editor) Processing Module

import type { IMEState, IMEEventHandlers, ModuleInterface } from './types.js';
import { CONFIG, KEYS } from './config.js';

/**
 * IME管理器类
 * 基于Gemini CLI最佳实践，处理中文输入法的键盘事件
 */
export class IMEManager implements ModuleInterface {
  private state: IMEState = {
    isComposing: false,
    lastCompositionEnd: 0,
    compositionData: ''
  };

  private eventHandlers: IMEEventHandlers = {
    compositionStart: this.handleCompositionStart.bind(this),
    compositionUpdate: this.handleCompositionUpdate.bind(this),
    compositionEnd: this.handleCompositionEnd.bind(this)
  };

  private keydownHandler: (event: KeyboardEvent) => boolean = this.handleKeyDown.bind(this);
  private inputHandler: (event: Event) => void = this.handleInput.bind(this);

  /**
   * 初始化IME管理器
   */
  public init(): void {
    // IME管理器无需额外初始化
  }

  /**
   * 销毁IME管理器
   */
  public destroy(): void {
    this.resetState();
  }

  /**
   * 处理组合开始事件
   */
  private handleCompositionStart(event: CompositionEvent): void {
    if (CONFIG.DEBUG.IME) {
      console.log('🎌 组合开始:', event.data || '');
    }
    this.state.isComposing = true;
    this.state.compositionData = event.data || '';
  }

  /**
   * 处理组合更新事件
   */
  private handleCompositionUpdate(event: CompositionEvent): void {
    if (CONFIG.DEBUG.IME) {
      console.log('🎌 组合更新:', event.data || '');
    }
    this.state.isComposing = true; // 确保状态正确
    this.state.compositionData = event.data || '';
  }

  /**
   * 处理组合结束事件
   */
  private handleCompositionEnd(event: CompositionEvent): void {
    if (CONFIG.DEBUG.IME) {
      console.log('🎌 组合结束:', event.data || '');
    }
    this.state.isComposing = false;
    this.state.compositionData = '';
    this.state.lastCompositionEnd = Date.now();
    
    // 额外延迟确保状态完全稳定
    setTimeout(() => {
      if (CONFIG.DEBUG.IME) {
        console.log('🎌 组合状态稳定检查');
      }
    }, CONFIG.IME.COMPOSITION_DELAY);
  }

  /**
   * 处理键盘按下事件
   * 基于Gemini CLI的精确修饰符检查方法
   */
  private handleKeyDown(event: KeyboardEvent): boolean {
    // 只处理Enter键
    if (event.key !== KEYS.ENTER) {
      return false; // 不阻止其他键的处理
    }

    if (CONFIG.DEBUG.IME) {
      console.log('🔍 Enter键状态:', {
        eventIsComposing: event.isComposing,
        ctrl: event.ctrlKey,
        meta: event.metaKey,
        shift: event.shiftKey,
        alt: event.altKey,
        stateIsComposing: this.state.isComposing
      });
    }

    // 检查1：浏览器API检测输入法状态
    if (event.isComposing) {
      if (CONFIG.DEBUG.IME) {
        console.log('🎌 输入法组合中，跳过');
      }
      return true; // 阻止处理
    }

    // 检查2：内部状态检测（备用）
    if (this.state.isComposing) {
      if (CONFIG.DEBUG.IME) {
        console.log('🎌 内部状态显示组合中，跳过');
      }
      return true; // 阻止处理
    }

    // 检查3：Gemini CLI的修饰符检查
    if (event.ctrlKey || event.metaKey || event.shiftKey || event.altKey) {
      if (CONFIG.DEBUG.IME) {
        console.log('🎌 修饰符存在，跳过');
      }
      return true; // 阻止处理
    }

    // 检查4：时间窗口检查（防止输入法延迟问题）
    const timeSinceComposition = Date.now() - this.state.lastCompositionEnd;
    if (timeSinceComposition < CONFIG.IME.COMPOSITION_DELAY) {
      if (CONFIG.DEBUG.IME) {
        console.log('🎌 组合结束时间太近，跳过');
      }
      return true; // 阻止处理
    }

    if (CONFIG.DEBUG.IME) {
      console.log('✅ IME检查通过，允许处理Enter键');
    }
    return false; // 允许处理
  }

  /**
   * 处理输入事件
   * 立即同步输入法状态，不依赖异步更新
   */
  private handleInput(event: Event): void {
    // 如果事件包含isComposing属性，立即同步状态
    const inputEvent = event as InputEvent;
    if (inputEvent.isComposing !== undefined) {
      // 立即同步，不等待
      const wasComposing = this.state.isComposing;
      this.state.isComposing = inputEvent.isComposing;
      
      if (inputEvent.isComposing) {
        if (CONFIG.DEBUG.IME) {
          console.log('📝 输入法组合中，同步状态');
        }
      } else if (wasComposing) {
        // 输入法结束时立即更新时间戳
        this.state.lastCompositionEnd = Date.now();
        if (CONFIG.DEBUG.IME) {
          console.log('📝 输入法结束，更新时间戳');
        }
      }
    }
  }

  /**
   * 检查Enter键是否应该被处理
   * 外部调用接口，用于消息发送等操作
   */
  public shouldProcessEnterKey(event: KeyboardEvent): boolean {
    return !this.handleKeyDown(event);
  }

  /**
   * 绑定IME事件到输入元素
   */
  public bindToElement(element: HTMLElement): void {
    element.addEventListener('compositionstart', this.eventHandlers.compositionStart);
    element.addEventListener('compositionupdate', this.eventHandlers.compositionUpdate);
    element.addEventListener('compositionend', this.eventHandlers.compositionEnd);
    element.addEventListener('input', this.inputHandler);
  }

  /**
   * 从输入元素解绑IME事件
   */
  public unbindFromElement(element: HTMLElement): void {
    element.removeEventListener('compositionstart', this.eventHandlers.compositionStart);
    element.removeEventListener('compositionupdate', this.eventHandlers.compositionUpdate);
    element.removeEventListener('compositionend', this.eventHandlers.compositionEnd);
    element.removeEventListener('input', this.inputHandler);
  }

  /**
   * 获取当前IME状态
   */
  public getState(): IMEState {
    return { ...this.state }; // 返回副本
  }

  /**
   * 检查是否正在组合
   */
  public get isComposing(): boolean {
    return this.state.isComposing;
  }

  /**
   * 重置IME状态
   */
  public resetState(): void {
    this.state = {
      isComposing: false,
      lastCompositionEnd: 0,
      compositionData: ''
    };
  }

  /**
   * 获取调试信息
   */
  public getDebugInfo(): object {
    return {
      ...this.state,
      timeSinceComposition: Date.now() - this.state.lastCompositionEnd,
      debugEnabled: CONFIG.DEBUG.IME
    };
  }
}

// 创建全局IME管理实例
export const imeManager = new IMEManager();