# new-api P0 上线操作手册

适用目标：

- 单机部署
- 公开注册开启
- 用户名密码登录开启
- 第三方登录关闭（本手册不启用 OAuth）
- 支付关闭
- 用户量不大，先求稳定可用

本文档按顺序执行，不追求复杂架构。

## 1. 目标架构

建议先用一台机器跑这 4 个组件：

- `new-api`
- `PostgreSQL`
- `Redis`
- `Nginx`

推荐访问链路：

`用户 -> HTTPS 域名 -> Nginx -> 127.0.0.1:3000 -> new-api`

不要把 `3000` 端口直接暴露到公网。

## 2. 前置准备

上线前先准备好：

- 一台 Linux 服务器，建议 `2C4G`
- 一个域名，例如 `transferapi.space`
- 域名已经解析到服务器公网 IP
- 服务器已安装：
  - `docker`
  - `docker compose`
  - `nginx`

建议系统层只开放这些端口：

- `22`
- `80`
- `443`

不要对公网开放：

- `3000`
- `5432`
- `6379`

## 3. 拉代码并进入目录

```bash
git clone https://github.com/QuantumNous/new-api.git
cd new-api
```

如果你已经在当前目录，就直接继续下一步。

## 4. 准备环境变量

复制模板：

```bash
cp .env.example .env
```

编辑 `.env`：

```bash
nano .env
```

P0 推荐值示例：

```dotenv
APP_BIND_IP=127.0.0.1
APP_PORT=3000
TZ=Asia/Shanghai
NODE_NAME=new-api-node-1

SESSION_SECRET=改成一段长随机字符串
CRYPTO_SECRET=改成另一段长随机字符串

POSTGRES_USER=newapi
POSTGRES_PASSWORD=改成强密码
POSTGRES_DB=new-api

REDIS_PASSWORD=改成强密码

ERROR_LOG_ENABLED=true
BATCH_UPDATE_ENABLED=true
STREAMING_TIMEOUT=300

TLS_INSECURE_SKIP_VERIFY=false
```

建议直接用下面命令生成随机值：

```bash
openssl rand -base64 48
openssl rand -base64 48
```

说明：

- `SESSION_SECRET` 不能弱，登录态靠它。
- `CRYPTO_SECRET` 也要单独设置，不要偷懒复用。
- `APP_BIND_IP` 和 `APP_PORT` 用于 Docker 端口映射。
- 推荐保持 `APP_BIND_IP=127.0.0.1`，这样宿主机只在本地监听 `127.0.0.1:3000`，由 Nginx 转发。

## 5. 检查并启动容器

先确认 `docker-compose.yml` 没有被改成公网绑定。

推荐形态应等价于：

```yaml
ports:
  - "127.0.0.1:3000:3000"
```

如果你使用的是环境变量版本，则应对应为：

```dotenv
APP_BIND_IP=127.0.0.1
APP_PORT=3000
```

启动：

```bash
docker compose up -d
```

检查状态：

```bash
docker compose ps
docker compose logs -f new-api
```

确认健康状态：

```bash
curl http://127.0.0.1:3000/api/status
```

返回里看到 `success: true` 即可。

如果这里不通，先不要继续配 Nginx，优先检查：

- `docker compose ps`
- `docker compose logs -f new-api`
- 宿主机 `3000` 端口是否仍绑定在 `127.0.0.1`

## 6. 配置 Nginx 反向代理

新建站点配置，例如：

```bash
sudo nano /etc/nginx/conf.d/new-api.conf
```

仓库里已经提供了 HTTP 模板，可直接拷贝后按需修改域名：

```bash
cp deploy/nginx/nginx.new-api.http.conf /etc/nginx/conf.d/new-api.conf
```

先写 HTTP 版本，确认反代通：

```nginx
server {
    listen 80;
    server_name transferapi.space;

    client_max_body_size 50m;

    location / {
        proxy_pass http://127.0.0.1:3000;
        proxy_http_version 1.1;

        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        proxy_buffering off;
        proxy_cache off;
        proxy_read_timeout 600s;
        proxy_send_timeout 600s;
    }
}
```

如果你把 `APP_PORT` 改成了别的值，这里的 `proxy_pass` 也要一起改。

检查并重载：

```bash
sudo nginx -t
sudo systemctl reload nginx
```

浏览器访问：

`http://transferapi.space`

能打开页面再继续。

## 7. 使用 Cloudflare 配置 HTTPS

如果你的域名已经托管在 Cloudflare，推荐使用：

- Cloudflare 代理开启（橙云）
- `SSL/TLS` 模式设为 `Full (strict)`
- Nginx 使用 Cloudflare Origin Certificate

不建议使用 `Flexible`，因为 Cloudflare 到源站仍是 HTTP，不适合作为正式登录站点的长期方案。

### 7.1 检查 Cloudflare DNS

确认域名记录已经指向服务器公网 IP，并且代理状态为橙云开启。

例如：

- `transferapi.space -> 你的服务器公网 IP`

### 7.2 在 Cloudflare 签发源站证书

在 Cloudflare 控制台进入：

- `SSL/TLS`
- `Origin Server`
- `Create Certificate`

生成后你会拿到两段内容：

- Origin Certificate
- Private Key

把它们保存到服务器，例如：

```bash
sudo mkdir -p /etc/nginx/ssl
sudo nano /etc/nginx/ssl/new-api-origin.pem
sudo nano /etc/nginx/ssl/new-api-origin.key
sudo chmod 600 /etc/nginx/ssl/new-api-origin.key
```

仓库里已经提供了 Cloudflare HTTPS 反代模板：

```bash
cp deploy/cloudflare/nginx.new-api.cloudflare.conf /etc/nginx/conf.d/new-api.conf
```

### 7.3 把 Cloudflare SSL 模式切到 Full (strict)

在 Cloudflare 控制台确认：

- `SSL/TLS encryption mode = Full (strict)`

### 7.4 更新 Nginx 为 HTTPS 反代

把站点配置改成下面这种形式：

```nginx
server {
    listen 80;
    server_name transferapi.space;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl http2;
    server_name transferapi.space;

    ssl_certificate /etc/nginx/ssl/new-api-origin.pem;
    ssl_certificate_key /etc/nginx/ssl/new-api-origin.key;

    client_max_body_size 50m;

    location / {
        proxy_pass http://127.0.0.1:3000;
        proxy_http_version 1.1;

        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto https;

        proxy_buffering off;
        proxy_cache off;
        proxy_read_timeout 600s;
        proxy_send_timeout 600s;
    }
}
```

检查并重载：

```bash
sudo nginx -t
sudo systemctl reload nginx
```

### 7.5 在 Cloudflare 打开 HTTPS 强制跳转

建议在 Cloudflare 控制台打开：

- `SSL/TLS -> Edge Certificates -> Always Use HTTPS`

完成后确认：

- 浏览器访问 `https://transferapi.space` 正常
- `http://transferapi.space` 自动跳转到 HTTPS

注意：

- 登录态建议始终在 HTTPS 下使用。
- 先把 HTTPS 配好，再开放给外部用户。

## 8. 首次初始化 new-api

浏览器打开你的线上地址：

`https://transferapi.space`

首次初始化时：

- 创建 `root` 管理员账号
- `SelfUseModeEnabled` 不开启
- `DemoSiteEnabled` 不开启

建议：

- root 用户名简短一些
- root 密码用强密码

初始化完成后，用 root 登录后台。

## 9. 先设置站点基础信息

登录后台后，优先设置这些内容：

### 9.1 检查登录与注册开关

确认这些开关状态：

- `PasswordLoginEnabled = 开`
- `RegisterEnabled = 开`
- `PasswordRegisterEnabled = 开`

### 9.2 关闭第三方登录

把不需要的第三方登录全部关闭：

- `GitHubOAuthEnabled = 关`
- `discord.enabled = 关`
- `oidc.enabled = 关`
- `LinuxDOOAuthEnabled = 关`
- 自定义 OAuth 提供商全部禁用

说明：

- 这份 P0 手册默认就是“公开注册 + 用户名密码登录”，不启用 OAuth。
- 如果你后面要开启 OAuth，再单独补充 `ServerAddress`、第三方平台回调地址和联调测试。

### 9.3 新用户额度

把 `QuotaForNewUser` 设为以下二选一：

- `0`
- 一个很小的值

推荐先设 `0`。

原因：

- 你现在要开公开注册
- 但 P0 又不打算上支付
- 所以最好不要让新用户默认拿到可被刷的免费额度

如果确实需要试用，再手动发额度。

### 9.4 ServerAddress

这一项在当前 P0 形态里不是强制项，但建议尽早填好。

如果你暂时不用：

- 密码找回邮件
- 第三方登录
- 支付跳转

那 `ServerAddress` 可以后面再配。

如果你想先配好，也建议填成：

```text
https://transferapi.space
```

如果你后面要启用 OAuth、Passkey、邮箱找回密码或支付跳转，这一项基本都要先配对。

## 10. 开启反机器人校验

公开注册时，这一步强烈建议做。

推荐用 Cloudflare Turnstile。

### 10.1 到 Cloudflare 创建 Turnstile

创建一个站点，得到：

- `Site Key`
- `Secret Key`

域名填写你的正式域名，例如：

`transferapi.space`

### 10.2 在后台填写 Turnstile 配置

在系统设置里填写：

- `TurnstileSiteKey`
- `TurnstileSecretKey`

然后打开：

- `TurnstileCheckEnabled = 开`

### 10.3 是否开启邮箱验证

P0 阶段建议：

- 第一阶段：先不开 `EmailVerificationEnabled`
- 如果公开注册后垃圾账号明显，再补 SMTP 并打开邮箱验证

原因很简单：

- Turnstile 已经能挡掉一部分脚本
- SMTP 会增加上线复杂度
- 你当前重点是先把公开注册跑稳

## 11. 支付相关保持关闭

P0 直接不配。

不要配置：

- Stripe
- Creem
- Waffo
- EPay

不要做的事：

- 不填支付密钥
- 不配置支付 webhook
- 不开启充值流程

## 12. 添加模型渠道

系统功能能跑，不代表用户已经能实际调用模型。

你至少要做这些：

### 12.1 创建一个测试渠道

先只加 1 到 2 个稳定渠道，不要一开始全加。

建议：

- 一个主渠道
- 一个备用渠道

### 12.2 只开放少量模型

P0 只开你确定稳定的模型。

例如：

- 一个主聊天模型
- 一个便宜备用模型

### 12.3 先用管理员或测试账号跑通

至少测试：

- 聊天请求成功
- 流式返回成功
- 日志有记录
- 额度统计正常

其中“流式返回成功”一定要从浏览器页面和实际 API 调用各测一次，用来确认 Nginx 没有把流式响应缓冲坏。

## 13. 上线前验收

按这个顺序验收：

### 13.1 基础可用性

- `https://transferapi.space` 可以打开
- 首页正常加载
- 管理员可登录
- 后台可以保存设置

### 13.2 公开注册

- 用户可以正常注册
- Turnstile 会触发
- 新用户默认额度符合预期
- 用户名密码登录正常

### 13.3 API 与模型

- 用户可创建 token
- token 可调用一个已配置模型
- 流式和非流式至少各测 1 次

### 13.4 持久化

重启后确认：

```bash
docker compose restart
```

然后检查：

- 管理员账号还在
- 注册开关状态还在
- 渠道还在
- 日志目录还在

## 14. 备份

P0 至少做这三类备份：

### 14.1 配置备份

备份：

- `.env`
- `docker-compose.yml`
- Nginx 配置文件

### 14.2 数据库备份

如果你沿用当前 Compose 里的 PostgreSQL：

```bash
docker exec postgres pg_dump -U newapi new-api > /root/new-api-$(date +%F).sql
```

按你自己的 `POSTGRES_USER` 和 `POSTGRES_DB` 调整命令。

### 14.3 应用数据备份

备份：

- `./data`
- `./logs`

建议每天至少备份一次数据库，并保存到另一台机器或对象存储。

如果你希望把 P0 需要的本地配置、数据库、`data`、`logs` 一次性打包，可以直接运行：

```bash
NGINX_CONF_PATH=/etc/nginx/conf.d/new-api.conf bash deploy/scripts/p0-backup.sh
```

## 15. 日常巡检

建议每天看一次：

```bash
docker compose ps
docker compose logs --tail=200 new-api
curl -s http://127.0.0.1:3000/api/status
```

如果你想把这几项合并成一次标准巡检，也可以直接运行：

```bash
bash deploy/scripts/p0-healthcheck.sh
```

重点观察：

- 容器是否反复重启
- 注册是否异常增长
- 渠道是否大面积失败
- 是否有明显刷号迹象

## 16. 出问题时先查什么

### 16.1 用户注册被刷

处理顺序建议：

1. 确认 `TurnstileCheckEnabled` 已开启
2. 把 `QuotaForNewUser` 调成 `0`
3. 必要时补 SMTP 并开启 `EmailVerificationEnabled`
4. 再考虑收紧注册策略

### 16.2 页面能开但登录状态异常

优先检查：

1. 是否已经通过 HTTPS 访问
2. Nginx 是否正确透传 `Host` 和 `X-Forwarded-Proto`
3. `SESSION_SECRET` 是否被改动过

### 16.3 用户能注册但不能调用模型

优先检查：

1. 是否已经给用户额度
2. 是否已创建 token
3. 渠道是否启用
4. 模型是否在该分组开放

## 17. P0 推荐最终配置

建议你的 P0 形态就是：

- 单机部署
- Docker Compose
- PostgreSQL
- Redis
- Nginx + HTTPS
- 公开注册开启
- 用户名密码登录开启
- 第三方登录关闭
- Turnstile 开启
- 支付关闭
- 新用户默认额度为 `0`
- 管理员手动发额度

## 18. 一次性执行清单

如果你只想照着做，可以按这个顺序：

1. 配域名解析到服务器
2. 安装 Docker、Docker Compose、Nginx
3. 复制 `.env.example` 到 `.env`
4. 填好密钥和数据库密码
5. `docker compose up -d`
6. 配 Nginx 反代到 `127.0.0.1:3000`
7. 在 Cloudflare 配置 `Full (strict)` + 源站证书
8. 打开站点，完成 root 初始化
9. 打开 `RegisterEnabled`
10. 打开 `PasswordRegisterEnabled`
11. 打开 `PasswordLoginEnabled`
12. 确认所有 OAuth 开关关闭
13. 配置 Turnstile 并打开 `TurnstileCheckEnabled`
14. 把 `QuotaForNewUser` 设为 `0`
15. 添加模型渠道
16. 用测试账号验证注册、登录和 API 调用
17. 做一次数据库备份
18. 正式开放给用户

## 19. P0 之后再做的事

这些都可以后面再补：

- SMTP 邮箱验证
- 2FA / Passkey 全量推广
- 更细的风控
- 支付
- 第三方登录
- 多机部署
- 更完整监控
- 自动备份到对象存储
