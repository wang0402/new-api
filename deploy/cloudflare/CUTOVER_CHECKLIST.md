# new-api Cloudflare 切换清单

这个目录里的文件是“待切换配置”，不会自动生效。

## 1. 先准备，不动线上

1. 复制 `env.p0.public-register.cloudflare.example` 到服务器上的临时文件，例如 `.env.next`
2. 按实际值填好域名、密码、密钥
3. `nginx.new-api.cloudflare.conf` 已预填正式域名 `transferapi.space`，如需变更再改
4. 确认 Cloudflare 已开启代理，SSL 模式为 `Full (strict)`
5. 在服务器保存好 Cloudflare Origin Certificate 和 Private Key

## 2. 维护窗口内执行

1. 备份当前 `.env`
2. 备份当前 Nginx 站点配置
3. 用 `.env.next` 替换线上 `.env`
4. 用新的 Nginx 配置替换站点配置
5. 先执行 `sudo nginx -t`
6. 再执行 `sudo systemctl reload nginx`
7. 最后执行 `docker compose up -d`

## 3. 切换后立即验证

1. `curl http://127.0.0.1:3000/api/status`
2. 浏览器访问 `https://你的域名`
3. 管理员登录
4. 测一次非流式请求
5. 测一次流式请求

## 4. 回滚预案

1. 恢复之前的 `.env`
2. 恢复之前的 Nginx 配置
3. `sudo nginx -t && sudo systemctl reload nginx`
4. `docker compose up -d`
