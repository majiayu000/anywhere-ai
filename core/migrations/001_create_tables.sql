-- 创建基础表结构
-- 基于Omnara模式，适配跨设备终端管理需求

-- 1. AgentInstance 表 (对应Omnara的agent_instances)
CREATE TABLE IF NOT EXISTS agent_instances (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id VARCHAR(255) NOT NULL,             -- 用户ID (可以是UUID或字符串)
    tool_name VARCHAR(50) NOT NULL,            -- claude, gemini, cursor, etc.
    status VARCHAR(20) DEFAULT 'active',       -- active, paused, ended
    name VARCHAR(255),                         -- 用户自定义会话名
    
    -- 设备管理 (我们的扩展)
    owner_device_id VARCHAR(100) NOT NULL,     -- 创建会话的设备ID
    current_device_id VARCHAR(100),            -- 当前活跃的设备ID
    
    -- 时间戳
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    ended_at TIMESTAMP,
    last_activity_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    -- 会话状态 (我们的扩展 - 存储PTY状态、环境变量等)
    session_state JSONB DEFAULT '{}',
    
    -- Git集成 (学习Omnara)
    git_diff TEXT,
    initial_git_hash VARCHAR(40),
    
    -- 权限状态 (学习Omnara的permission_state)
    permission_state JSONB DEFAULT '{}',
    
    -- 约束
    CHECK (status IN ('active', 'paused', 'ended')),
    CHECK (tool_name IN ('claude', 'gemini', 'cursor', 'custom'))
);

-- 2. Messages 表 (完全学习Omnara)
CREATE TABLE IF NOT EXISTS messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_instance_id UUID NOT NULL REFERENCES agent_instances(id) ON DELETE CASCADE,
    sender_type VARCHAR(10) NOT NULL,          -- 'USER' or 'AGENT'
    content TEXT NOT NULL,
    requires_user_input BOOLEAN DEFAULT FALSE,
    git_diff TEXT,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    -- 约束
    CHECK (sender_type IN ('USER', 'AGENT'))
);

-- 3. Session State 表 (我们的扩展 - 存储详细的终端状态)
CREATE TABLE IF NOT EXISTS session_states (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_instance_id UUID NOT NULL REFERENCES agent_instances(id) ON DELETE CASCADE,
    
    -- PTY状态
    terminal_buffer TEXT,                      -- 终端缓冲区内容
    cursor_position JSONB,                     -- {row: int, col: int}
    terminal_size JSONB,                       -- {rows: int, cols: int}
    
    -- 进程状态
    process_pid INTEGER,                       -- 进程ID
    working_directory TEXT,                    -- 工作目录
    environment_vars JSONB DEFAULT '{}',      -- 环境变量
    
    -- 历史记录
    command_history TEXT[],                    -- 命令历史
    output_history TEXT[],                     -- 输出历史
    
    -- 版本控制
    state_version INTEGER DEFAULT 1,          -- 状态版本号
    checksum VARCHAR(64),                      -- 状态校验和
    
    -- 时间戳
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 4. Devices 表 (设备管理)
CREATE TABLE IF NOT EXISTS devices (
    id VARCHAR(100) PRIMARY KEY,              -- 设备唯一标识
    user_id VARCHAR(255) NOT NULL,            -- 所属用户
    device_name VARCHAR(100) NOT NULL,        -- 设备名称
    device_type VARCHAR(20) NOT NULL,         -- ios, macos, web
    platform_info JSONB DEFAULT '{}',         -- 平台信息
    
    -- 网络信息 (用于mDNS发现)
    ip_address INET,                           -- IP地址
    port INTEGER,                              -- 服务端口
    
    -- 状态
    is_online BOOLEAN DEFAULT FALSE,           -- 是否在线
    last_seen_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    -- 时间戳
    registered_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    -- 约束
    CHECK (device_type IN ('ios', 'macos', 'web', 'linux', 'windows'))
);

-- 5. Session Transfers 表 (会话迁移记录)
CREATE TABLE IF NOT EXISTS session_transfers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_instance_id UUID NOT NULL REFERENCES agent_instances(id) ON DELETE CASCADE,
    from_device_id VARCHAR(100) NOT NULL REFERENCES devices(id),
    to_device_id VARCHAR(100) NOT NULL REFERENCES devices(id),
    
    -- 迁移状态
    transfer_status VARCHAR(20) DEFAULT 'pending',  -- pending, in_progress, completed, failed
    transfer_method VARCHAR(20) NOT NULL,           -- full_state, messages_only, hybrid
    
    -- 迁移数据
    transferred_data_size BIGINT,               -- 传输数据大小(字节)
    compression_ratio REAL,                     -- 压缩比例
    
    -- 时间统计
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP,
    duration_ms INTEGER,                        -- 迁移耗时(毫秒)
    
    -- 错误信息
    error_message TEXT,
    
    -- 约束
    CHECK (transfer_status IN ('pending', 'in_progress', 'completed', 'failed')),
    CHECK (transfer_method IN ('full_state', 'messages_only', 'hybrid')),
    CHECK (from_device_id != to_device_id)
);

-- 创建索引优化查询性能

-- agent_instances 表索引
CREATE INDEX IF NOT EXISTS idx_agent_instances_user_status ON agent_instances(user_id, status);
CREATE INDEX IF NOT EXISTS idx_agent_instances_device ON agent_instances(current_device_id);
CREATE INDEX IF NOT EXISTS idx_agent_instances_activity ON agent_instances(last_activity_at DESC);
CREATE INDEX IF NOT EXISTS idx_agent_instances_tool ON agent_instances(tool_name);

-- messages 表索引
CREATE INDEX IF NOT EXISTS idx_messages_instance_created ON messages(agent_instance_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_messages_requires_input ON messages(agent_instance_id, requires_user_input, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_messages_sender_type ON messages(sender_type, created_at DESC);

-- session_states 表索引
CREATE INDEX IF NOT EXISTS idx_session_states_instance ON session_states(agent_instance_id);
CREATE INDEX IF NOT EXISTS idx_session_states_version ON session_states(agent_instance_id, state_version DESC);
CREATE INDEX IF NOT EXISTS idx_session_states_updated ON session_states(updated_at DESC);

-- devices 表索引
CREATE INDEX IF NOT EXISTS idx_devices_user ON devices(user_id);
CREATE INDEX IF NOT EXISTS idx_devices_online ON devices(is_online, last_seen_at DESC);
CREATE INDEX IF NOT EXISTS idx_devices_type ON devices(device_type);

-- session_transfers 表索引
CREATE INDEX IF NOT EXISTS idx_session_transfers_instance ON session_transfers(agent_instance_id);
CREATE INDEX IF NOT EXISTS idx_session_transfers_status ON session_transfers(transfer_status, started_at DESC);
CREATE INDEX IF NOT EXISTS idx_session_transfers_from_device ON session_transfers(from_device_id);
CREATE INDEX IF NOT EXISTS idx_session_transfers_to_device ON session_transfers(to_device_id);

-- 创建更新时间戳的触发器
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- 为需要自动更新时间戳的表创建触发器
CREATE TRIGGER update_session_states_updated_at BEFORE UPDATE ON session_states
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- 创建清理函数 (删除过期数据)
CREATE OR REPLACE FUNCTION cleanup_old_data()
RETURNS INTEGER AS $$
DECLARE
    deleted_count INTEGER := 0;
    temp_count INTEGER;
BEGIN
    -- 清理90天前结束的会话
    DELETE FROM agent_instances 
    WHERE status = 'ended' 
    AND ended_at < CURRENT_TIMESTAMP - INTERVAL '90 days';
    GET DIAGNOSTICS temp_count = ROW_COUNT;
    deleted_count := deleted_count + temp_count;
    
    -- 清理30天前的会话状态快照 (保留最新版本)
    DELETE FROM session_states 
    WHERE created_at < CURRENT_TIMESTAMP - INTERVAL '30 days'
    AND id NOT IN (
        SELECT DISTINCT ON (agent_instance_id) id
        FROM session_states
        ORDER BY agent_instance_id, state_version DESC
    );
    GET DIAGNOSTICS temp_count = ROW_COUNT;
    deleted_count := deleted_count + temp_count;
    
    -- 清理7天前离线的设备记录
    DELETE FROM devices 
    WHERE is_online = FALSE 
    AND last_seen_at < CURRENT_TIMESTAMP - INTERVAL '7 days';
    GET DIAGNOSTICS temp_count = ROW_COUNT;
    deleted_count := deleted_count + temp_count;
    
    -- 清理30天前完成的传输记录
    DELETE FROM session_transfers 
    WHERE transfer_status IN ('completed', 'failed')
    AND completed_at < CURRENT_TIMESTAMP - INTERVAL '30 days';
    GET DIAGNOSTICS temp_count = ROW_COUNT;
    deleted_count := deleted_count + temp_count;
    
    RETURN deleted_count;
END;
$$ LANGUAGE plpgsql;

-- 创建定期清理的任务 (需要pg_cron扩展)
-- SELECT cron.schedule('cleanup-old-data', '0 2 * * *', 'SELECT cleanup_old_data();');

-- 插入默认数据
INSERT INTO devices (id, user_id, device_name, device_type, platform_info, is_online)
VALUES 
    ('system-default', 'system', 'Default Device', 'web', '{"version": "1.0"}', TRUE)
ON CONFLICT (id) DO NOTHING;