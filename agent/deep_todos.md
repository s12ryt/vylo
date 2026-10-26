# Deep Todos（項目完整歷史任務）

> 狀態：✅ 完成 | 🔄 進行中 | ⏸ 暫緩 | ❌ 取消

## 已完成

| # | 任務 | 狀態 | 備註 |
|---|------|------|------|
| 1 | 需求確認（三輪問答）+ agent/question.md 驗收標準 | ✅ | 簡易賺錢網站/Go/單按鈕/steps=2 |
| 2 | 環境檢查 + `go mod init vylo` + 目錄骨架 | ✅ | Go 1.26.3 |
| 3 | TDD config 模組 | ✅ | 4 測試 |
| 4 | TDD vylo client | ✅ | 5 測試；Vylo 錯誤 body 解析教訓 |
| 5 | TDD cache 模組 | ✅ | 6 測試；fakeClock 測 TTL |
| 6 | TDD server handler | ✅ | 7 測試；405/404/fallback |
| 7 | 前端 index.html / thanks.html + embed | ✅ | 純 CSS、零依賴、繁中 |
| 8 | cmd/server/main.go + adapter | ✅ | cache→server 介面轉接 |
| 9 | e2e 測試（全真依賴 + embed 頁 + fallback） | ✅ | 2 測試；no-redirect client 教訓 |
| 10 | 本地啟動驗證（真 exe、模擬 Vylo 故障） | ✅ | 4 路由全過、fallback 302、無 token 洩漏 |
| 11 | .gitignore / README（systemd 部署）/ agent 記錄 | ✅ | |
| 12 | 需求變更：CI + Docker(ghcr) + GitHub 發布 | ✅ | question.md 第七節；撤銷「不做 Docker」 |
| 13 | Dockerfile + .dockerignore（multi-stage、非 root、HEALTHCHECK） | ✅ | 本機無 docker，建置驗證交給 Actions |
| 14 | .github/workflows/{ci.yml,docker.yml} | ✅ | ci 含 `-race`（補遺留 A）；docker 推 ghcr 多平台 |
| 15 | git init + 10 個原子提交 + 建 repo s12ryt/vylo + push | ✅ | semantic 英文；無 sisyphus 標記 |

## 未盡事項 / 遺留

| # | 事項 | 狀態 | 說明 |
|---|------|------|------|
| A | `go test -race ./...` | ✅ | 已改由 CI（ubuntu-latest）每次 push/PR 跑 `-race -count=1` 補驗（Windows 無 gcc 跑不了） |
| B | 真實 Vylo token 端到端 | ⏸ | 部署時填入正式 VYLO_API_TOKEN，點一次 /go 確認真短鏈跳轉 |
| C | Nginx/Caddy 反代 + HTTPS | ⏸ | 部署階段；記得 TARGET_URL 改正式域名 |
| D | VPS 實機 `docker run` ghcr 鏡像 | ⏸ | 鏡像已驗證 public（匿名 tags/list 通過：main/latest/sha-3c14628），pull 免登入 |

## 明確不做（依需求確認）

統計後台、多目標輪換。（Docker 已於第二輪需求變更納入）
