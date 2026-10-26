# Vylo 賺錢網站

[![CI](https://github.com/s12ryt/vylo/actions/workflows/ci.yml/badge.svg)](https://github.com/s12ryt/vylo/actions/workflows/ci.yml)
[![Docker](https://github.com/s12ryt/vylo/actions/workflows/docker.yml/badge.svg)](https://github.com/s12ryt/vylo/actions/workflows/docker.yml)

一個極簡的「點擊幫我賺錢」網站：訪客點擊按鈕 → 被導向 [Vylo](https://vylo.cc)（AdLinkFly 相容廣告短網址服務）產生的短鏈 → 看完廣告層級後抵達感謝頁，站主因而獲得分潤。

## 特性

- **單一執行檔部署**：前端頁面以 `embed` 打進二進位，無任何外部依賴（純 Go 標準庫）
- **API token 只存在後端**：訪客永遠看不到 Vylo token，錯誤細節也不會外洩
- **快取**：同一組（目標網址, 廣告層級）只調用一次 Vylo API（TTL 12 小時），避免觸發 120 req/min 限流
- **自動重試**：Vylo 回 429 或網路錯誤時指數退避重試（最多 3 次）
- **降級 fallback**：Vylo 完全故障時，`/go` 直接 302 到目標頁，網站不會掛

## 路由

| 路由 | 說明 |
|------|------|
| `GET /` | 首頁（大按鈕） |
| `GET /go` | 取短鏈並 302 跳轉（Vylo 故障時 fallback 到 TARGET_URL） |
| `GET /thanks` | 感謝頁（可再點一次） |
| `GET /healthz` | 健康檢查，回 `ok` |

## 環境變數

| 變數 | 必填 | 預設 | 說明 |
|------|------|------|------|
| `VYLO_API_TOKEN` | ✅ | — | Vylo API token（Dashboard → Tools & API 取得） |
| `VYLO_BASE_URL` | | `https://vylo.cc` | Vylo 服務地址 |
| `VYLO_STEPS` | | `2` | 廣告層級 1–3，非法值自動回落 2 |
| `TARGET_URL` | | `http://localhost:{PORT}/thanks` | 廣告跳完後的落地頁 |
| `PORT` | | `8080` | 監聽埠 |

## 本地開發

```bash
# 跑測試（config / vylo / cache / server / e2e 全綠）
go test ./...

# 啟動
VYLO_API_TOKEN=你的token go run ./cmd/server
```

建議把環境變數放進 `.env`（已被 .gitignore 排除），但 systemd 部署請直接用 `EnvironmentFile`。

## 部署到 VPS（Docker，推薦）

鏡像由 GitHub Actions 自動建置並推送至 GHCR：`ghcr.io/s12ryt/vylo`（支援 `linux/amd64`、`linux/arm64`；標籤：`latest`、`main`、`vX.Y.Z`、`sha-xxxxxxx`）。

> 鏡像繼承 repo 可見性（目前為 **private**）。VPS 上需先登入：建立具 `read:packages` 權限的 [Personal Access Token](https://github.com/settings/tokens)，然後 `docker login ghcr.io -u s12ryt -p <PAT>`。若把 package 設為 public 則免登入。

```bash
docker run -d \
  --name vylo \
  --restart unless-stopped \
  -p 8080:8080 \
  -e VYLO_API_TOKEN=你的token \
  -e TARGET_URL=https://你的域名/thanks \
  ghcr.io/s12ryt/vylo:latest

docker inspect --format '{{.State.Health.Status}}' vylo   # 應為 healthy
```

映像檔特性：multi-stage 建置、非 root 使用者（UID 10001）運行、內建 `HEALTHCHECK`（打 `/healthz`）。

## 部署到 VPS（systemd，二進制直跑）

### 1. 編譯（在開發機或 VPS 上）

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o vylo-server ./cmd/server
```

### 2. 放置檔案

```bash
sudo install -m 755 vylo-server /usr/local/bin/vylo-server
sudo useradd --system --no-create-home --shell /usr/sbin/nologin vylo 2>/dev/null || true
```

### 3. 環境變數檔 `/etc/vylo.env`（權限 600，勿提交到 git）

```ini
VYLO_API_TOKEN=你的token
PORT=8080
VYLO_STEPS=2
TARGET_URL=https://你的域名/thanks
```

```bash
sudo chmod 600 /etc/vylo.env
sudo chown root:root /etc/vylo.env
```

### 4. 建立單元 `/etc/systemd/system/vylo.service`

```ini
[Unit]
Description=Vylo earn site
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=vylo
EnvironmentFile=/etc/vylo.env
ExecStart=/usr/local/bin/vylo-server
Restart=on-failure
RestartSec=3
# 安全加固
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

### 5. 啟動

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now vylo
curl -s http://127.0.0.1:8080/healthz   # 應輸出 ok
journalctl -u vylo -f                   # 看日誌
```

前端再以 Nginx/Caddy 反代 80/443 到 8080 即可（記得把 `TARGET_URL` 設為正式域名）。

## 專案結構

```
cmd/server/          # 入口：組裝依賴並啟動
internal/config/     # 環境變數載入與預設值
internal/vylo/       # Vylo API client（JSON 解析、錯誤透傳、429 重試）
internal/cache/      # 短鏈記憶體快取（TTL、RWMutex）
internal/server/     # HTTP 路由與 handler（含 e2e 測試）
web/                 # 前端頁面（embed 崁入）
agent/               # 開發記錄（需求、項目表、memory）
```
