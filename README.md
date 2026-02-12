# Cursor API 2 Claude

OpenAI 格式 API 代理，支持将请求转发到 Anthropic Claude 等多个后端，内置 Web 控制台可视化配置。

> **⚠️ 重要：本服务必须部署在公网可访问的服务器上。**
>
> Cursor 的自定义 OpenAI Base URL 功能，请求是从 Cursor 官方服务器发出的，而不是从你的本地电脑发出。因此本服务必须部署在有公网 IP 的服务器上（如云服务器），不能部署在内网或本地局域网中，否则 Cursor 无法访问。

## 工作原理

```
Cursor 客户端 → Cursor 服务器 → 你的公网服务器（本服务） → Anthropic / OpenAI API
```

## 特性

- **多 Provider 支持** — Anthropic、OpenAI 等多个后端，独立配置
- **权重负载均衡** — 同一模型多个 Provider，按权重自动分配
- **模型名称映射** — 请求中的模型名自动映射到实际模型（如 `gpt-4o` → `claude-sonnet-4-5`）
- **Web 控制台** — 浏览器直接管理 Provider、模型映射、测试连通性
- **模型启用/禁用** — 每个模型可独立开关
- **访问密码** — 支持 API Key 鉴权和管理后台密码保护
- **零外部前端依赖** — 纯 HTML + CSS + JS，内嵌到二进制

## 快速开始

### 本地运行

```bash
go build -o server .
./server
```

访问 `http://localhost:3029/admin` 进入控制台。

### Docker 部署

```bash
# 创建配置文件（首次）
echo '{"port":3029,"api_key":"","admin_password":"","providers":[]}' > config.json

# 启动
docker compose up -d
```

或手动运行：

```bash
docker run -d \
  -p 3029:3029 \
  -v ./config.json:/app/config.json \
  --restart unless-stopped \
  registry.cn-chengdu.aliyuncs.com/xarr/xarr-cursor-api-2-claude:latest
```

### 构建并推送镜像

```bash
make build          # 构建镜像
make push           # 构建并推送到阿里云
make push TAG=v1.0  # 指定 tag
```

## 配置说明

首次启动自动生成 `config.json`，也可通过 Web 控制台修改：

```json
{
  "port": 3029,
  "api_key": "your-api-key",
  "admin_password": "your-admin-password",
  "providers": [
    {
      "id": "claude-main",
      "name": "Claude 主线",
      "type": "anthropic",
      "base_url": "https://api.anthropic.com",
      "api_key": "sk-ant-xxx",
      "weight": 1,
      "timeout": 300,
      "models": [
        {"from": "gpt-4o", "to": "claude-sonnet-4-5", "enabled": true},
        {"from": "claude-*", "to": "claude-*", "enabled": true}
      ]
    }
  ]
}
```

| 字段 | 说明 |
|------|------|
| `port` | 监听端口 |
| `api_key` | API 访问密钥（空=不鉴权） |
| `admin_password` | 管理后台密码（空=无需密码） |
| `providers[].type` | `anthropic` 或 `openai` |
| `providers[].weight` | 权重（0=禁用） |
| `providers[].models[].from` | 请求中的模型名（支持通配符 `*`） |
| `providers[].models[].to` | 实际发送的模型名 |
| `providers[].models[].enabled` | 是否启用 |

## API 端点

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/v1/chat/completions` | OpenAI 格式对话（主端点） |
| POST | `/v1/messages` | Anthropic 格式透传 |
| GET | `/v1/models` | 已配置的模型列表 |
| GET | `/health` | 健康检查 |
| GET | `/admin` | Web 控制台 |

## 使用示例

在 Cursor 中配置自定义 OpenAI Base URL：

```
https://your-public-domain.com/v1
```

> 必须使用公网可访问的域名或 IP，建议配合 Nginx 反向代理并启用 HTTPS。

设置 API Key 为配置中的 `api_key` 值即可。
