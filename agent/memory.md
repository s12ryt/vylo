# Memory（操作記錄）

> 本檔記錄項目所有關鍵讀寫/決策操作，供後續接手。
> 格式：日期 | 操作 | 對象 | 備註

## 2026-10-26

| 時間 | 操作 | 對象 | 備註 |
|------|------|------|------|
| 建立 | 寫入 | agent/question.md | 三輪問答確認的需求與 TDD 驗收標準（F1–F5） |
| 建立 | 寫入 | go.mod | `go mod init vylo`，Go 1.26.3 |
| 建立 | 目錄 | agent/ cmd/server/ internal/{config,vylo,cache,server} web/ | 骨架 |
| TDD | 寫入 | internal/config/{config.go,config_test.go} | RED：缺 token 報錯測試先失敗 → GREEN：Load() 實作；非法 steps 回落 2 |
| TDD | 寫入 | internal/vylo/{client.go,client_test.go} | 教訓：Vylo 錯誤 = HTTP 400 + JSON body 並存，必須先解析 body 取 message；`vylo: ` 前綴；429/網路錯誤包 retryableError（errors.As 判定）指數退避×3；mock query 斷言要用 url.Values 型別 |
| TDD | 寫入 | internal/cache/{cache.go,cache_test.go} | fakeClock 注入 now 測 TTL；錯誤不快取；50 goroutine 併發測試通過（-race 因無 gcc 跳過，見 deep_todos） |
| TDD | 寫入 | internal/server/{server.go,server_test.go} | Go 1.22 `GET /{$}` pattern；405 自動處理；fallback 不外洩 err |
| 修正 | 編輯 | internal/server/server.go | 補 import "context"（vet 揭露） |
| 建立 | 寫入 | web/index.html | 修正草稿混入的簡體字（「对我的鼓励」→「對我的鼓勵」） |
| 建立 | 寫入 | web/thanks.html | 與首頁同風格 |
| 建立 | 寫入 | web/web.go | `//go:embed index.html thanks.html` → web.FS |
| 建立 | 寫入 | cmd/server/main.go | adapter 解決 cache.GetOrShorten ≠ server.Shortener.Shorten 方法名 |
| TDD | 寫入 | internal/server/e2e_test.go | 教訓：http.Get 會自動跟隨 302 打到真 vylo.cc → 改用 CheckRedirect=ErrUseLastResponse；e2e 驗證 embed 頁+快取只調 API 一次+fallback 不洩 token |
| 建置 | 執行 | go build -o vylo-server.exe | 通過 |
| 啟動 | 程序 | vylo-server.exe（PID 43184，PORT=18080，VYLO_BASE_URL=127.0.0.1:9 模擬 Vylo 掛掉） | Start-Process -WindowStyle Hidden -PassThru |
| 驗證 | HTTP | /healthz=200 ok、/=200(4116B)、/thanks=200(3822B)、/go=302 Location=http://127.0.0.1:18080/thanks（fallback 生效）、頁面無 token 洩漏 | 全過 |
| 清理 | 程序 | Stop-Process 43184 | 完成 |
| 建立 | 寫入 | .gitignore README.md agent/{項目表,memory,deep_todos}.md | README 含 systemd 部署 |
| 回歸 | 執行 | go vet ./... + go test ./... -count=1 | 全綠 |

## 環境備註

- Windows 開發機無 gcc → `go test -race` 不可用（-race 需 cgo）；已由 CI ubuntu runner 補驗
- PowerShell：`&&` 之後不能接 `$env:X='...'` 賦值語句，需用 `;` 分隔
- PowerShell 不吃 bash 式 `GIT_MASTER=1 git` 前綴；用 `$env:GIT_MASTER='1'; git ...`
- 本機無 docker → Dockerfile 建置驗證交給 GitHub Actions runner
- git 身份：user.name=ྀ⁺уσуσ༝⁺₊໒꒱༝⁺、user.email=102228212+s12ryt@users.noreply.github.com（用戶本人）；**禁止添加任何 sisyphus 相關標記/co-author 到 git 記錄**
- gh CLI 2.93.0 可用；GitHub 帳號 s12ryt

## 2026-10-26（第二輪：CI + Docker ghcr + GitHub）

| 時間 | 操作 | 對象 | 備註 |
|------|------|------|------|
| 需求 | 記錄 | agent/question.md 第七節 | 用戶：「做CI和自動編譯ghcr的docker之後丟到github」；撤銷原「不做Docker」 |
| 建立 | 寫入 | Dockerfile | multi-stage golang:1.26-alpine→alpine:3.20；CGO_ENABLED=0 trimpath -ldflags "-s -w"；UID 10001 非 root；wget+ca-certificates；HEALTHCHECK 打 /healthz |
| 建立 | 寫入 | .dockerignore | 排除 .git/.github/agent/exe/env/README |
| 建立 | 寫入 | .github/workflows/ci.yml | checkout@v4+setup-go@v5(go-version-file)+vet+`test -race -count=1`+build |
| 建立 | 寫入 | .github/workflows/docker.yml | buildx+login(ghcr,GITHUB_TOKEN)+metadata(tags: branch/semver/sha/latest)+build-push@v6 amd64+arm64；PR 只 build 不 push；gha cache |
| 編輯 | 更新 | README.md | 頂部 CI/Docker 徽章；新增 Docker 部署段（ghcr.io/s12ryt/vylo、private 拉取需 docker login） |
| git | 執行 | git init -b main | 首次 repo；.gitignore 確認排除 vylo-server.exe |
| git | 執行 | 10 個原子提交 | chore(go.mod)→feat(config/vylo/cache/web/server/app)→docs→ci(docker)→ci(workflows)；semantic 英文；e2e 在 web 之後（import web.FS） |
| 發布 | 執行 | github create_repository s12ryt/vylo (private 起步) + git push -u origin main | gh auth 供憑證（scopes 含 workflow） |
| 變更 | 執行 | repo 改 public | 用戶指示「變成公開倉庫」；`gh repo edit --visibility public --accept-visibility-change-consequences` |
| 驗證 | 執行 | Actions：CI 1m26s ✅（vet+test -race+build 全綠，遺留 A 正式關閉）；Docker 7m39s ✅ | run 40255233746/40255233748 |
| 驗證 | 執行 | ghcr 鏡像匿名 tags/list 通過 | tags: main, latest, sha-3c14628；package 隨 repo 改 public（繼承可見性），pull 免登入；gh token 無 read:packages scope 故用 registry API 驗證 |
| 編輯 | 更新 | README.md | Docker 段改為 public 直接拉取說明 |
