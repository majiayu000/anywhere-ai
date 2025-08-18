#!/bin/bash

echo "测试 Anywhere AI 聊天功能"
echo "========================="

# 1. 创建新会话
echo -e "\n1. 创建新的 Claude 会话..."
SESSION_RESPONSE=$(curl -s -X POST http://localhost:8081/api/v1/terminal/sessions \
  -H "Content-Type: application/json" \
  -d '{"tool":"claude","name":"chat-test"}')

SESSION_ID=$(echo $SESSION_RESPONSE | jq -r '.id')
echo "会话已创建: $SESSION_ID"

# 2. 等待 Claude 启动
echo -e "\n2. 等待 Claude 启动..."
sleep 5

# 3. 检查会话输出
echo -e "\n3. 检查会话状态..."
curl -s http://localhost:8081/api/v1/terminal/sessions/$SESSION_ID/output | jq -r '.output' | tail -10

# 4. 通过 tmux 发送消息
echo -e "\n4. 直接通过 tmux 发送测试消息..."
tmux send-keys -t $SESSION_ID "Hello Claude! Can you see this message?" Enter

# 5. 等待响应
echo -e "\n5. 等待响应..."
sleep 5

# 6. 查看输出
echo -e "\n6. 查看 Claude 的响应..."
tmux capture-pane -t $SESSION_ID -p | tail -30

echo -e "\n测试完成！"
echo "访问 http://localhost:8081/chat 查看聊天界面"
echo "选择会话 '$SESSION_ID' 进行交互"