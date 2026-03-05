# 蒜子计数器项目 - 需求文档

## 项目概述

复刻不蒜子（busuanzi.ibruce.info）极简网页计数器服务，命名为"蒜子"（Suanzi）。

## 技术栈

| 组件 | 技术选型 |
|------|----------|
| 后端 | Go + SQLite |
| 前端管理 | Vue 3 |
| 计数脚本 | 原生 JavaScript（静态文件） |
| 部署 | systemd + nginx 反向代理 |
| 端口 | 7788 |

## ⚠️ 红线（必须遵守）

1. **不动现有服务** - 不影响线上不蒜子（busuanzi.ibruce.info）任何配置
2. **不动现有配置** - 不修改 nginx、systemd 等现有服务配置
3. **不随意操作** - 不 kill 进程、不删文件、不改端口
4. **只用新目录** - 所有文件放在 `/data/suanzi/`
5. **必须 git 提交** - 所有代码改动必须 commit

## 目录结构

```
/data/suanzi/
├── src/              # 源代码（git 仓库）
│   ├── main.go       # 入口
│   ├── api/          # API 处理
│   ├── db/           # 数据库操作
│   ├── static/       # 静态文件（JS 脚本、前端页面）
│   └── PRD.md        # 本文档
├── bin/              # 编译后的二进制文件
├── data/             # SQLite 数据库
│   └── suanzi.db
├── logs/             # 日志文件
│   └── suanzi.log
└── static/           # 静态资源（可选）
```

## 核心功能

### 1. 计数 API

```
POST /api/count
Body: {
  "domain": string,      // 网站域名
  "type": "site_pv" | "site_uv" | "page_pv",
  "path": string,        // 页面路径（page_pv 时用）
  "uuid": string         // 用户 UUID（UV 计数用）
}
```

### 2. 查询 API

```
GET /api/count?domain=xxx&type=site_pv&path=xxx
Response: {
  "count": number
}
```

### 3. 初始化 API

```
POST /api/init
Body: {
  "domain": string,
  "type": string,
  "path": string,
  "count": number
}
```

### 4. 介绍页面

```
GET / - 介绍页面（类似 busuanzi.ibruce.info）
GET /js/suanzi.min.js - 计数脚本
```

## 数据库设计

```sql
-- 站点表
CREATE TABLE sites (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    domain TEXT UNIQUE NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- 计数表
CREATE TABLE counters (
    site_id INTEGER NOT NULL,
    type TEXT NOT NULL,           -- 'site_pv', 'site_uv', 'page_pv'
    path TEXT DEFAULT '',         -- 页面路径（page_pv 时用）
    count INTEGER DEFAULT 0,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (site_id, type, path),
    FOREIGN KEY (site_id) REFERENCES sites(id)
);

-- UV 记录表（按天去重）
CREATE TABLE uv_records (
    site_id INTEGER NOT NULL,
    uuid TEXT NOT NULL,
    date DATE NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (site_id, uuid, date),
    FOREIGN KEY (site_id) REFERENCES sites(id)
);
```

## UV 识别方案

- 前端 JS 生成 UUID 存入 Cookie（有效期 1 年）
- 每次请求携带 UUID
- 后端按 `site_id + uuid + date` 去重（每天只计一次 UV）

## 前端 JS 脚本功能

1. 检查 Cookie 是否有 UUID，没有则生成
2. 页面加载时调用 `/api/count` 记录 PV/UV
3. 查询计数值并回填到页面 span 标签
4. 支持的标签 ID：
   - `suanzi_value_site_pv` - 站点总访问量
   - `suanzi_value_site_uv` - 站点总访客数
   - `suanzi_value_page_pv` - 单页面阅读量

## 部署配置

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
RestartSec=3

[Install]
WantedBy=multi-user.target
```

### nginx 反向代理（后续配置）

```nginx
server {
    listen 80;
    server_name suanzi.cakepanit.com;
    
    location / {
        proxy_pass http://127.0.0.1:7788;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

## 开发流程

1. **阶段 1**: 目录结构 + 数据库初始化 + 基础 API
2. **阶段 2**: 计数逻辑（PV/UV）+ JS 脚本
3. **阶段 3**: 介绍页面 + 前端管理界面（Vue）
4. **阶段 4**: 部署配置（systemd + nginx）

## 验收节点

- [ ] 目录结构创建完成
- [ ] 数据库表创建完成
- [ ] 核心 API 可用（计数、查询）
- [ ] JS 脚本正常工作
- [ ] systemd 服务配置完成
- [ ] nginx 配置完成（需飞驰总确认）

---

**创建时间**: 2026-03-05
**负责人**: coder 智能体
**监督**: 饼铛
