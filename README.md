# 🧄 蒜子 (Suanzi) - 极简网页计数器

永久免费的网页访问量统计服务，两行代码搞定网站流量监控。

## 快速开始

在你的网站 HTML 中添加：

```html
<script async src="//suanzi.cakepanit.com/js/suanzi.min.js"></script>
<span id="suanzi_container_site_pv">本站总访问量<span id="suanzi_value_site_pv"></span>次</span>
```

## 支持的统计类型

| 类型 | 说明 | HTML 标签 ID |
|------|------|-------------|
| `site_pv` | 站点总访问量 | `suanzi_value_site_pv` |
| `site_uv` | 站点总访客数（按天去重） | `suanzi_value_site_uv` |
| `page_pv` | 单页面阅读量 | `suanzi_value_page_pv` |

### 使用示例

```html
<!-- 站点总访问量 -->
<span id="suanzi_container_site_pv">本站总访问量<span id="suanzi_value_site_pv"></span>次</span>

<!-- 站点总访客数 -->
<span id="suanzi_container_site_uv">本站总访客数<span id="suanzi_value_site_uv"></span>人次</span>

<!-- 单页面阅读量 -->
<span id="suanzi_container_page_pv">本文阅读量<span id="suanzi_value_page_pv"></span>次</span>
```

## 技术栈

- **后端**: Go (原生 net/http)
- **数据库**: SQLite
- **前端**: 原生 JavaScript（无依赖）
- **部署**: systemd + nginx 反向代理

## API 接口

### POST /api/count - 计数

**批量模式（推荐）：**
```json
POST /api/count
Content-Type: application/json
Referer: https://yourdomain.com

{
  "domain": "yourdomain.com",
  "uuid": "用户 UUID",
  "path": "/page/path",
  "types": ["site_pv", "site_uv", "page_pv"]
}

Response:
{
  "counts": {
    "site_pv": 1234,
    "site_uv": 567,
    "page_pv": 89
  }
}
```

**单类型模式（向后兼容）：**
```json
{
  "domain": "yourdomain.com",
  "type": "site_pv",
  "path": "",
  "uuid": "用户 UUID"
}
```

### GET /api/count - 查询

```
GET /api/count?domain=example.com&type=site_pv&path=/page

Response:
{"count": 1234}
```

### POST /api/init - 初始化计数值

```json
POST /api/init
{
  "domain": "example.com",
  "type": "site_pv",
  "path": "",
  "count": 100
}
```

### GET /api/stats - 全站统计

```
GET /api/stats

Response:
{
  "sites": 4,
  "total_pv": 531,
  "total_uv": 45
}
```

## 核心逻辑

### UV 去重机制
- 前端生成 UUID 存入 Cookie（有效期 1 年）
- 后端按 `site_id + uuid + date` 去重
- 同一用户每天只计 1 次 UV

### PV 计数规则
- 访问页面时同时增加 `site_pv` 和 `page_pv`
- 只提交 `page_pv` 时自动连带增加 `site_pv`
- 每次刷新 +1

### 安全机制
- **Referer 校验**: Referer 域名必须与 `domain` 参数一致
- **CORS 跨域**: 允许所有域名调用（方便嵌入任意网站）
- **速率限制**: Let's Encrypt 证书申请 1 小时最多失败 5 次

## 部署配置

### 目录结构
```
/data/suanzi/
├── src/              # 源代码（git 仓库）
│   ├── main.go       # 入口
│   ├── api/          # API 处理
│   ├── db/           # 数据库操作
│   ├── static/       # 静态文件
│   └── README.md     # 本文档
├── bin/              # 编译后的二进制
├── data/             # SQLite 数据库
├── logs/             # 日志
└── static/           # 静态资源
```

### systemd 服务
```ini
[Unit]
Description=Suanzi Counter Service
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/data/suanzi
ExecStart=/data/suanzi/bin/suanzi
Restart=always

[Install]
WantedBy=multi-user.target
```

### nginx 配置
```nginx
server {
    listen 80;
    server_name suanzi.cakepanit.com;
    
    location / {
        proxy_pass http://127.0.0.1:7788;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
    
    location /js/ {
        proxy_pass http://127.0.0.1:7788;
    }
}

server {
    listen 443 ssl http2;
    server_name suanzi.cakepanit.com;
    
    ssl_certificate /etc/letsencrypt/live/suanzi.cakepanit.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/suanzi.cakepanit.com/privkey.pem;
    
    location / {
        proxy_pass http://127.0.0.1:7788;
        proxy_set_header Host $host;
    }
}
```

## 开发规范

- 所有代码改动必须 git 提交
- 不要随意修改 `.gitignore`
- 修改配置文件前需备份

## 项目历史

- **2026-03-05**: 项目创建，复刻不蒜子功能
- **2026-03-05**: 支持批量计数、Referer 校验、CORS 跨域
- **2026-03-05**: 添加全站统计展示

## 许可证

永久免费使用

## 在线演示

https://suanzi.cakepanit.com/
