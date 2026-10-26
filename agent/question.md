# Vylo 賺錢網站 — 需求記錄（TDD 驗收標準）

> 日期：2026-10-26
> 由用戶問答確認的需求，作為本項目開發與測試驗收的唯一標準。

## 一、項目定位

基於 Vylo（vylo.cc，AdLinkFly 兼容之廣告短網址服務）做一個**極簡賺錢網站**：

- 頁面只有 **一個精美的「點擊幫我賺錢」按鈕** 與 **精美背景**。
- 訪客點擊按鈕 → 進入 Vylo 廣告短鏈（經過 steps 頁廣告）→ 到達目標頁（感謝頁）。
- 站長賺取每次瀏覽的廣告分成。

## 二、用戶決策（問答結果）

| 決策項 | 用戶選擇 |
|---|---|
| 項目方向 | 一個簡易的賺錢網站 |
| 網站內容 | 只有一個精美的「點擊幫我賺錢」按鈕 + 精美背景 |
| 部署 | 用戶自有 VPS |
| 後端技術棧 | **Go**（標準庫優先，單二進制部署） |
| 廣告層級 steps | **2**（可配置項） |

## 三、Vylo API 要點（來源：https://vylo.cc/developers）

- 縮短端點：`GET https://vylo.cc/api?api={TOKEN}&url={URL}&steps={1..3}`
- 成功回應：`{"status":"success","shortenedUrl":"https://vylo.cc/Ab3x9K"}`（HTTP 200）
- 失敗回應：`{"status":"error","message":"..."}`（HTTP 401/403/400）
- 限流：每 IP 每分鐘 120 請求，超過回 **429**
- token 保密：任何人持 token 可在帳號上建連結 → **token 只能存在後端環境變數**

## 四、功能需求（驗收標準）

### F1 配置模組（internal/config）
- [ ] 從環境變數讀取：
  - `VYLO_API_TOKEN`（必填，缺失時啟動報錯）
  - `VYLO_BASE_URL`（可選，預設 `https://vylo.cc`，測試時可指向 mock）
  - `VYLO_STEPS`（可選，預設 `2`，非法值回落 `2`）
  - `TARGET_URL`（可選，預設為本站 `/thanks` 感謝頁之絕對 URL）
  - `PORT`（可選，預設 `8080`）

### F2 Vylo client（internal/vylo）
- [ ] `Shorten(ctx, longURL, steps)` 組正確查詢參數並解析 JSON 回應
- [ ] 成功時回傳 `shortenedUrl`
- [ ] `status=error` 時回傳含 `message` 的錯誤
- [ ] 非 200/429 狀態碼回傳明確錯誤
- [ ] **429 限流時自動退避重試**（最多 3 次）

### F3 短鏈快取（internal/cache）
- [ ] 相同 `(longURL, steps)` 只呼叫 Vylo API 一次，之後用快取
- [ ] 快取項有 TTL（預設 12h，可配置），過期後重新呼叫
- [ ] 併發安全（sync.RWMutex）

### F4 HTTP server（internal/server）
- [ ] `GET /`：精美首頁（按鈕 + 背景）
- [ ] `GET /go`：核心端點——取得短鏈（快取優先）後 **302 跳轉**到 `shortenedUrl`
- [ ] **Fallback：Vylo API 失敗時直接 302 到 TARGET_URL**（寧可不賺不能讓訪客掛掉）
- [ ] `GET /thanks`：感謝頁（廣告跳完的落地頁）
- [ ] `GET /healthz`：回 200 `ok`（部署健康檢查用）
- [ ] token 不得出現在任何回應/頁面原始碼中

### F5 前端（web/）
- [ ] `index.html`：精美背景（漸層動畫）、置中大按鈕「點擊幫我賺錢」，點擊 → `/go`
- [ ] `thanks.html`：感謝頁，精美風格一致，有「再幫點一次」按鈕回 `/go`
- [ ] 無外部資源依賴（離線可用、載入快）
- [ ] 響應式（手機/桌面）

## 五、非功能需求

- [ ] Go 版本：穩定版（≥1.21），僅用標準庫（不引入第三方依賴）
- [ ] 所有模組有對應 `_test.go`，`go test ./...` 全綠
- [ ] `go vet` 通過
- [ ] `.gitignore` 排除 `.env`、二進制
- [ ] README 記錄部署方式（systemd 範例）

## 六、明確不做（本輪範圍外）

- 不做統計後台 / 點擊分析
- 不做多目標輪換
- ~~不做 Docker 化（VPS 直接跑二進制）~~ → **已於 2026-10-26 需求變更撤銷，見下方第七節**

## 七、需求變更記錄（2026-10-26，第二輪）

用戶追加需求（原文）：「做CI和自動編譯ghcr的docker之後丟到github」

### 新增驗收標準

- C1 CI（`.github/workflows/ci.yml`）：push main / PR 觸發；`go vet` + `go test -race`（補驗 Windows 本機跑不了的 race detector）+ `go build`
- C2 Docker（`Dockerfile` + `.dockerignore`）：multi-stage（golang:1.26-alpine → alpine:3.20）；`CGO_ENABLED=0` 靜態編譯；非 root（UID 10001）；含 ca-certificates（連 vylo.cc）與 wget（healthcheck）；`HEALTHCHECK` 打 `/healthz`
- C3 鏡像發布（`.github/workflows/docker.yml`）：push main / tag `v*` / PR 觸發；推 `ghcr.io/s12ryt/vylo`；多平台 amd64+arm64；標籤 latest/main/semver/sha；PR 只建置不推送；GHA cache
- C4 GitHub 發布：repo `s12ryt/vylo`（private 起步，可隨時改 public）；原子化 commit 歷史；README 含徽章與 Docker 部署段
- C5 token 安全不變：GITHUB_TOKEN 權限最小化（contents:read + packages:write）；token 仍只走環境變數，不進鏡像層（`ENV` 只有 PORT）
