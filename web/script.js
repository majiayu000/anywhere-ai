// Anywhere AI Chat - JavaScript文件

const API_BASE = 'http://localhost:8080';
let ws = null;
let currentSessionId = null;
let sessions = [];
let messages = [];
let isTyping = false;
let typingTimeout = null;
let isComposing = false; // IME 输入状态
let lastCompositionEnd = 0; // 最后一次composition结束时间
let compositionData = ''; // 当前组合文本

// Initialize
document.addEventListener('DOMContentLoaded', () => {
    loadSessions();
    connectWebSocket();
    loadClaudeCommands(); // 加载最新的Claude命令列表
    
    // 点击其他地方隐藏自动补全
    document.addEventListener('click', (event) => {
        const dropdown = document.getElementById('autocompleteDropdown');
        const inputArea = document.querySelector('.input-area');
        
        if (dropdown.classList.contains('show') && 
            !inputArea.contains(event.target)) {
            hideAutocomplete();
        }
    });
});

// WebSocket Connection
function connectWebSocket() {
    ws = new WebSocket(`ws://localhost:8080/api/v1/ws`);
    
    ws.onopen = () => {
        console.log('WebSocket连接成功');
        updateStatus(true, '已连接');
    };

    ws.onmessage = (event) => {
        const data = JSON.parse(event.data);
        handleWebSocketMessage(data);
    };

    ws.onclose = () => {
        console.log('WebSocket连接断开');
        updateStatus(false, '连接断开');
        setTimeout(connectWebSocket, 3000);
    };

    ws.onerror = (error) => {
        console.error('WebSocket错误:', error);
        updateStatus(false, '连接错误');
    };
}

// Update Connection Status
function updateStatus(connected, text) {
    const dot = document.getElementById('statusDot');
    const statusText = document.getElementById('statusText');
    
    if (connected) {
        dot.classList.add('connected');
        statusText.textContent = text || '已连接';
    } else {
        dot.classList.remove('connected');
        statusText.textContent = text || '未连接';
    }
}

// Handle WebSocket Messages
function handleWebSocketMessage(data) {
    switch(data.action) {
        case 'messages':
            if (data.sessionId === currentSessionId) {
                messages = data.data || [];
                renderMessages();
            }
            break;
            
        case 'newMessage':
            if (data.sessionId === currentSessionId && data.data) {
                const message = data.data;
                
                // 收到Agent消息时隐藏输入动画
                if (message.sender_type === 'AGENT') {
                    hideTypingIndicator();
                }
                
                messages.push(message);
                renderMessages();
                scrollToBottom();
            }
            break;
            
        case 'typing':
            if (data.sessionId === currentSessionId) {
                showTypingIndicator();
            }
            break;
            
        case 'stopTyping':
            if (data.sessionId === currentSessionId) {
                hideTypingIndicator();
            }
            break;
    }
}

// Load Sessions
async function loadSessions() {
    try {
        const response = await fetch(`${API_BASE}/api/v1/terminal/sessions`);
        if (response.ok) {
            sessions = await response.json();
            renderSessions();
        }
    } catch (error) {
        console.error('加载会话失败:', error);
        document.getElementById('sessionsList').innerHTML = 
            '<div style="color: #ef4444; text-align: center; padding: 1rem;">加载失败</div>';
    }
}

// Render Sessions
function renderSessions() {
    const container = document.getElementById('sessionsList');
    
    if (sessions.length === 0) {
        container.innerHTML = '<div style="text-align: center; color: #9ca3af; padding: 1rem;">暂无会话</div>';
        return;
    }

    container.innerHTML = sessions.map(session => `
        <div class="session ${session.id === currentSessionId ? 'active' : ''}" 
             onclick="selectSession('${session.id}')">
            <div class="session-name">
                🤖 ${session.name || session.id}
            </div>
            <div class="session-info">
                ${session.tool} • ${session.status === 'active' ? '运行中' : '已停止'}
            </div>
        </div>
    `).join('');
}

// Select Session
async function selectSession(sessionId) {
    // 重置输入状态
    hideTypingIndicator();
    
    currentSessionId = sessionId;
    const session = sessions.find(s => s.id === sessionId);
    
    if (session) {
        document.getElementById('emptyState').style.display = 'none';
        document.getElementById('chatInterface').style.display = 'flex';
        document.getElementById('chatTitle').textContent = `🤖 ${session.name || session.id}`;
        
        // 清空消息
        messages = [];
        document.getElementById('messagesContainer').innerHTML = '';
        
        // 订阅会话
        if (ws && ws.readyState === WebSocket.OPEN) {
            ws.send(JSON.stringify({ 
                action: 'subscribe', 
                sessionId: sessionId 
            }));
            
            ws.send(JSON.stringify({
                action: 'selectSession',
                sessionId: sessionId
            }));
        }
        
        renderSessions();
    }
}

// Render Messages
function renderMessages() {
    const container = document.getElementById('messagesContainer');
    
    if (messages.length === 0 && !isTyping) {
        container.innerHTML = '<div class="loading">暂无消息，开始对话吧！</div>';
        return;
    }

    let html = messages.map(msg => {
        const isAgent = msg.sender_type === 'AGENT';
        const time = new Date(msg.created_at).toLocaleTimeString();
        
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
                    <div class="text">${formatMessage(msg.content)}</div>
                </div>
            </div>
        `;
    }).join('');
    
    // 添加输入动画
    if (isTyping) {
        html += `
            <div class="typing-indicator">
                <div class="avatar">🤖</div>
                <div class="typing-dots"></div>
            </div>
        `;
    }
    
    container.innerHTML = html;
    // 延迟滚动确保DOM已更新
    setTimeout(() => scrollToBottom(), 50);
}

// Format Message Content
function formatMessage(content) {
    // HTML转义
    content = content.replace(/&/g, '&amp;')
                   .replace(/</g, '&lt;')
                   .replace(/>/g, '&gt;');
    
    // 代码块
    content = content.replace(/```([\s\S]*?)```/g, '<pre>$1</pre>');
    
    // 行内代码
    content = content.replace(/`([^`]+)`/g, '<code>$1</code>');
    
    // 换行
    content = content.replace(/\n/g, '<br>');
    
    return content;
}

// Send Message - 移除重复检查，相信handleKeyPress的判断
async function sendMessage() {
    const input = document.getElementById('messageInput');
    const message = input.value.trim();
    
    // 只做基本检查
    if (!message || !currentSessionId) {
        console.log('🚫 消息为空或无会话');
        return;
    }
    
    console.log('📤 执行发送消息:', message);
    

    // 显示加载状态
    const sendBtn = document.getElementById('sendBtn');
    sendBtn.disabled = true;
    sendBtn.classList.add('loading');
    sendBtn.textContent = '发送中...';

    try {
        if (ws && ws.readyState === WebSocket.OPEN) {
            const messageData = {
                action: 'sendMessage',
                sessionId: currentSessionId,
                input: message,
                timestamp: Date.now() // 添加时间戳防重复
            };
            
            console.log('🔗 WebSocket发送数据:', messageData);
            ws.send(JSON.stringify(messageData));
            
            console.log('🧹 清空输入框');
            input.value = '';
            autoResize(input);
            
            // 显示输入动画
            showTypingIndicator();
            console.log('✅ 消息发送完成，等待回复');
        } else {
            console.error('❌ WebSocket未连接，无法发送消息');
        }
    } catch (error) {
        console.error('发送消息失败:', error);
        hideTypingIndicator();
    } finally {
        // 重置发送按钮
        setTimeout(() => {
            sendBtn.disabled = false;
            sendBtn.classList.remove('loading');
            sendBtn.textContent = '发送';
        }, 1000);
    }
}

// Show/Hide Typing Indicator
function showTypingIndicator() {
    if (!isTyping && currentSessionId) {
        isTyping = true;
        renderMessages();
        
        // 30秒后自动隐藏
        typingTimeout = setTimeout(() => {
            hideTypingIndicator();
        }, 30000);
    }
}

function hideTypingIndicator() {
    if (isTyping) {
        isTyping = false;
        if (typingTimeout) {
            clearTimeout(typingTimeout);
            typingTimeout = null;
        }
        renderMessages();
    }
}

// Handle Key Press & IME - 基于Gemini CLI最佳实践
function handleKeyPress(event) {
    // 首先处理自动补全导航
    if (handleAutocompleteNavigation(event)) {
        return;
    }

    // Enter键处理 - 极简Gemini CLI模式，移除所有时间缓冲
    if (event.key === 'Enter') {
        console.log('🔍 Enter键状态:', {
            eventIsComposing: event.isComposing,
            ctrl: event.ctrlKey,
            meta: event.metaKey, 
            shift: event.shiftKey,
            alt: event.altKey
        });
        
        // 唯一检查1：浏览器API检测输入法状态
        if (event.isComposing) {
            console.log('🎌 输入法组合中，跳过');
            return;
        }
        
        // 唯一检查2：Gemini CLI的修饰符检查
        if (event.ctrlKey || event.metaKey || event.shiftKey || event.altKey) {
            console.log('🎌 修饰符存在，跳过');
            return;
        }
        
        // 立即发送，不再有任何延迟或缓冲！
        console.log('✅ 立即发送消息');
        event.preventDefault();
        sendMessage();
    }
}

function handleCompositionStart(event) {
    console.log('🎌 组合开始:', event.data || '');
    isComposing = true;
    compositionData = event.data || '';
}

function handleCompositionUpdate(event) {
    console.log('🎌 组合更新:', event.data || '');
    isComposing = true; // 确保状态正确
    compositionData = event.data || '';
}

function handleCompositionEnd(event) {
    console.log('🎌 组合结束:', event.data || '');
    isComposing = false;
    compositionData = '';
    lastCompositionEnd = Date.now();
    
    // 额外延迟确保状态完全稳定
    setTimeout(() => {
        console.log('🎌 组合状态稳定检查');
    }, 10);
}

// Command autocomplete data - 从官方文档动态获取
let claudeCommands = [
    // 默认命令，当网络请求失败时使用
    { command: '/help', description: '获取使用帮助' },
    { command: '/clear', description: '清空对话历史' },
    { command: '/status', description: '查看账户和系统状态' },
    { command: '/model', description: '选择或更改AI模型' }
];

// 从官方文档获取最新命令列表
async function loadClaudeCommands() {
    try {
        // 尝试通过后端代理获取命令列表
        const response = await fetch(`${API_BASE}/api/v1/claude-commands`);
        
        if (response.ok) {
            const data = await response.json();
            if (data.commands && data.commands.length > 0) {
                claudeCommands = data.commands.map(cmd => ({
                    command: cmd.command,
                    description: translateDescription(cmd.description)
                }));
                console.log(`✅ 通过后端代理成功加载 ${claudeCommands.length} 个Claude命令`);
                return;
            }
        }
        
        // 如果后端代理失败，使用备用完整列表
        claudeCommands = [
            { command: '/add-dir', description: '添加额外的工作目录' },
            { command: '/agents', description: '管理专用任务的自定义AI子代理' },
            { command: '/bug', description: '报告错误（发送对话到Anthropic）' },
            { command: '/clear', description: '清空对话历史' },
            { command: '/compact', description: '压缩对话，可选聚焦指令' },
            { command: '/config', description: '查看/修改配置' },
            { command: '/cost', description: '显示token使用统计' },
            { command: '/doctor', description: '检查Claude Code安装健康状态' },
            { command: '/help', description: '获取使用帮助' },
            { command: '/init', description: '使用CLAUDE.md指南初始化项目' },
            { command: '/login', description: '切换Anthropic账户' },
            { command: '/logout', description: '退出Anthropic账户' },
            { command: '/mcp', description: '管理MCP服务器连接和OAuth认证' },
            { command: '/memory', description: '编辑CLAUDE.md记忆文件' },
            { command: '/model', description: '选择或更改AI模型' },
            { command: '/permissions', description: '查看或更新权限' },
            { command: '/pr_comments', description: '查看拉取请求评论' },
            { command: '/review', description: '请求代码审查' },
            { command: '/status', description: '查看账户和系统状态' },
            { command: '/terminal-setup', description: '安装Shift+Enter换行键绑定' },
            { command: '/vim', description: '进入vim模式，交替插入和命令模式' }
        ];
        console.log('📋 使用内置完整命令列表');
        
    } catch (error) {
        console.error('❌ 获取Claude命令失败:', error);
        // 网络错误时使用备用命令列表
        claudeCommands = [
            { command: '/add-dir', description: '添加额外的工作目录' },
            { command: '/agents', description: '管理专用任务的自定义AI子代理' },
            { command: '/bug', description: '报告错误（发送对话到Anthropic）' },
            { command: '/clear', description: '清空对话历史' },
            { command: '/compact', description: '压缩对话，可选聚焦指令' },
            { command: '/config', description: '查看/修改配置' },
            { command: '/cost', description: '显示token使用统计' },
            { command: '/doctor', description: '检查Claude Code安装健康状态' },
            { command: '/help', description: '获取使用帮助' },
            { command: '/init', description: '使用CLAUDE.md指南初始化项目' },
            { command: '/login', description: '切换Anthropic账户' },
            { command: '/logout', description: '退出Anthropic账户' },
            { command: '/mcp', description: '管理MCP服务器连接和OAuth认证' },
            { command: '/memory', description: '编辑CLAUDE.md记忆文件' },
            { command: '/model', description: '选择或更改AI模型' },
            { command: '/permissions', description: '查看或更新权限' },
            { command: '/pr_comments', description: '查看拉取请求评论' },
            { command: '/review', description: '请求代码审查' },
            { command: '/status', description: '查看账户和系统状态' },
            { command: '/terminal-setup', description: '安装Shift+Enter换行键绑定' },
            { command: '/vim', description: '进入vim模式，交替插入和命令模式' }
        ];
        console.log('🔄 使用备用命令列表');
    }
}

// 翻译常见命令描述
function translateDescription(desc) {
    const translations = {
        'Add additional working directories': '添加额外的工作目录',
        'Manage custom AI subagents for specialized tasks': '管理专用任务的自定义AI子代理',
        'Report bugs (sends conversation to Anthropic)': '报告错误（发送对话到Anthropic）',
        'Clear conversation history': '清空对话历史',
        'Compact conversation with optional focus instructions': '压缩对话，可选聚焦指令',
        'View/modify configuration': '查看/修改配置',
        'Show token usage statistics': '显示token使用统计',
        'Checks the health of your Claude Code installation': '检查Claude Code安装健康状态',
        'Get usage help': '获取使用帮助',
        'Initialize project with CLAUDE.md guide': '使用CLAUDE.md指南初始化项目',
        'Switch Anthropic accounts': '切换Anthropic账户',
        'Sign out from your Anthropic account': '退出Anthropic账户',
        'Manage MCP server connections and OAuth authentication': '管理MCP服务器连接和OAuth认证',
        'Edit CLAUDE.md memory files': '编辑CLAUDE.md记忆文件',
        'Select or change the AI model': '选择或更改AI模型',
        'View or update permissions': '查看或更新权限',
        'View pull request comments': '查看拉取请求评论',
        'Request code review': '请求代码审查',
        'View account and system statuses': '查看账户和系统状态',
        'Install Shift+Enter key binding for newlines': '安装Shift+Enter换行键绑定',
        'Enter vim mode for alternating insert and command modes': '进入vim模式，交替插入和命令模式'
    };
    
    return translations[desc] || desc;
}

let selectedCommandIndex = -1;

// 监听input事件，辅助判断输入法状态和命令补全 - 立即同步状态
function handleInput(event) {
    // 关键修复：立即同步状态，不依赖异步更新
    if (event.isComposing !== undefined) {
        // 立即同步，不等待
        isComposing = event.isComposing;
        
        if (event.isComposing) {
            console.log('📝 输入法组合中，跳过命令补全');
            return;
        } else {
            // 输入法结束时立即更新时间戳
            lastCompositionEnd = Date.now();
            console.log('📝 输入法结束，更新时间戳');
        }
    }
    
    // 只在确认非组合状态下处理命令自动补全
    if (!isComposing && !event.isComposing) {
        handleCommandAutocomplete(event);
    }
}

// 分离命令自动补全逻辑
function handleCommandAutocomplete(event) {
    const input = event.target;
    const value = input.value;
    const cursorPosition = input.selectionStart;
    
    // 检查光标位置的文本是否以 / 开头
    const textBeforeCursor = value.substring(0, cursorPosition);
    const lastSlashIndex = textBeforeCursor.lastIndexOf('/');
    
    if (lastSlashIndex !== -1 && lastSlashIndex === textBeforeCursor.length - 1) {
        // 刚输入 /，显示所有命令
        showAutocomplete(claudeCommands, '');
    } else if (lastSlashIndex !== -1) {
        // 有 / 且后面有文本，进行筛选
        const commandText = textBeforeCursor.substring(lastSlashIndex);
        if (commandText.startsWith('/') && commandText.length > 1) {
            const query = commandText.toLowerCase();
            const filteredCommands = claudeCommands.filter(cmd => 
                cmd.command.toLowerCase().includes(query)
            );
            if (filteredCommands.length > 0) {
                showAutocomplete(filteredCommands, query);
            } else {
                hideAutocomplete();
            }
        } else if (commandText === '/') {
            showAutocomplete(claudeCommands, '');
        }
    } else {
        hideAutocomplete();
    }
}

// 显示自动补全下拉框
function showAutocomplete(commands, query) {
    const dropdown = document.getElementById('autocompleteDropdown');
    selectedCommandIndex = -1;
    
    dropdown.innerHTML = commands.map((cmd, index) => `
        <div class="autocomplete-item" data-index="${index}" onclick="selectCommand('${cmd.command}')">
            <div class="autocomplete-command">${cmd.command}</div>
            <div class="autocomplete-description">${cmd.description}</div>
        </div>
    `).join('');
    
    dropdown.classList.add('show');
}

// 隐藏自动补全下拉框
function hideAutocomplete() {
    const dropdown = document.getElementById('autocompleteDropdown');
    dropdown.classList.remove('show');
    selectedCommandIndex = -1;
}

// 选择命令
function selectCommand(command) {
    const input = document.getElementById('messageInput');
    const value = input.value;
    const cursorPosition = input.selectionStart;
    
    // 找到最后一个 / 的位置
    const textBeforeCursor = value.substring(0, cursorPosition);
    const lastSlashIndex = textBeforeCursor.lastIndexOf('/');
    
    if (lastSlashIndex !== -1) {
        // 替换从 / 开始到光标位置的文本
        const newValue = value.substring(0, lastSlashIndex) + command + ' ' + value.substring(cursorPosition);
        input.value = newValue;
        
        // 设置光标位置到命令后面
        const newCursorPos = lastSlashIndex + command.length + 1;
        input.setSelectionRange(newCursorPos, newCursorPos);
    }
    
    hideAutocomplete();
    input.focus();
}

// 处理键盘导航
function handleAutocompleteNavigation(event) {
    const dropdown = document.getElementById('autocompleteDropdown');
    if (!dropdown.classList.contains('show')) return false;
    
    const items = dropdown.querySelectorAll('.autocomplete-item');
    if (items.length === 0) return false;
    
    switch(event.key) {
        case 'ArrowDown':
            event.preventDefault();
            selectedCommandIndex = (selectedCommandIndex + 1) % items.length;
            updateSelectedCommand(items);
            return true;
            
        case 'ArrowUp':
            event.preventDefault();
            selectedCommandIndex = selectedCommandIndex <= 0 ? items.length - 1 : selectedCommandIndex - 1;
            updateSelectedCommand(items);
            return true;
            
        case 'Enter':
            if (selectedCommandIndex >= 0 && selectedCommandIndex < items.length) {
                event.preventDefault();
                const selectedCommand = items[selectedCommandIndex].querySelector('.autocomplete-command').textContent;
                selectCommand(selectedCommand);
                return true;
            }
            break;
            
        case 'Escape':
            hideAutocomplete();
            return true;
    }
    
    return false;
}

// 更新选中的命令样式
function updateSelectedCommand(items) {
    items.forEach((item, index) => {
        if (index === selectedCommandIndex) {
            item.classList.add('selected');
        } else {
            item.classList.remove('selected');
        }
    });
}

// Auto Resize Textarea
function autoResize(textarea) {
    textarea.style.height = 'auto';
    textarea.style.height = Math.min(textarea.scrollHeight, 100) + 'px';
}

// Create New Session
async function createNewSession() {
    try {
        const response = await fetch(`${API_BASE}/api/v1/terminal/sessions`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ 
                tool: 'claude', 
                name: `claude-${Date.now()}` 
            })
        });

        if (response.ok) {
            const session = await response.json();
            await loadSessions();
            selectSession(session.id);
        }
    } catch (error) {
        console.error('创建会话失败:', error);
    }
}

// Clear Messages
function clearMessages() {
    messages = [];
    hideTypingIndicator();
    renderMessages();
}

// Scroll to Bottom
function scrollToBottom() {
    const container = document.getElementById('messagesContainer');
    if (container) {
        // 使用 requestAnimationFrame 确保DOM已更新
        requestAnimationFrame(() => {
            container.scrollTop = container.scrollHeight;
        });
    }
}

// 强制滚动到底部（调试用）
function forceScrollToBottom() {
    const container = document.getElementById('messagesContainer');
    if (container) {
        container.scrollTo({
            top: container.scrollHeight,
            behavior: 'smooth'
        });
        console.log(`滚动: scrollHeight=${container.scrollHeight}, scrollTop=${container.scrollTop}`);
    }
}