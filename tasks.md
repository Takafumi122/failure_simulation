# FailureLab 実装タスク一覧 (Phase 1, Phase 2 & Phase 3)

## Phase 1: 離散イベントシミュレーション基盤とトポロジー可視化 (完了)
- [x] 要件定義・アーキテクチャ設計・タスク一覧の作成
- [x] Go 離散イベントシミュレーションエンジン (`container/heap`)・障害注入・メトリクス計算・APIの実装
- [x] Phase 1 ユニットテストの実装とパス確認
- [x] React + TypeScript + `@xyflow/react` + Recharts フロントエンドの実装

## Phase 2: Retry / Circuit Breaker / Fail-Fast 比較シミュレーション (完了)
- [x] 座標ベース決定論的乱数生成器 (`internal/engine/rng.go`) の実装
- [x] Retry (Exponential Backoff + Jitter)、Circuit Breaker (`CLOSED`/`OPEN`/`HALF_OPEN`)、Fail-Fast Timeout の実装
- [x] 5戦略一括比較 API とフロントエンド比較表・グラフ・CBタイムラインの実装
- [x] シナリオ A〜I 自動テストの実装とパス確認

## Phase 3: Portfolio Release (完了)
- [x] **1. 品質確認・不変条件検証**:
  - [x] 成功/失敗別レイテンシ (`successP95LatencyMs` / `failureP95LatencyMs`) の分離計算と Fail-Fast 検証テスト
  - [x] truncated 結果の明示警告表示と未完了リクエストの失敗計上
  - [x] 全プリセット×全5戦略での不変条件テスト (`TestSimulationInvariants`: ワーカー数非負・リークなし、二重完了なし、Retry上限遵守)
- [x] **2. シナリオギャラリー**:
  - [x] `Healthy System`, `Transient Failure`, `Prolonged Outage`, `Retry Storm`, `Circuit Breaker Recovery` の5カードをワンクリックで読み込み・比較できる [`ScenarioGallery.tsx`](frontend/src/components/ScenarioGallery.tsx) の実装
- [x] **3. UI改善**:
  - [x] 初見ユーザー向けガイド、Retry / Circuit Breaker / Fail-Fast の各説明文の追加
  - [x] 実測メトリクスのみから算出される「Key Observations from Measured Simulation Data」の追加
- [x] **4. 再現性**:
  - [x] 設定 JSON のエクスポート・インポート機能（フロントエンド入力バリデーション付き）の実装
  - [x] Engine Version (`v3.0.0-des`)、Seed、RPS、Duration のヘッダー表示
- [x] **5. 自動テスト・CI**:
  - [x] GitHub Actions ワークフロー ([`.github/workflows/ci.yml`](.github/workflows/ci.yml)) の作成
  - [x] Playwright E2E テスト ([`frontend/e2e/failurelab.spec.ts`](frontend/e2e/failurelab.spec.ts)) の実装とパス確認
- [x] **6. 公開準備**:
  - [x] マルチステージ [`Dockerfile`](Dockerfile)、[`render.yaml`](render.yaml)、[`fly.toml`](fly.toml) の整備
  - [x] `PORT` / `STATIC_DIR` / `CORS_ALLOWED_ORIGINS` 環境変数対応、`/healthz` 追加、リクエストボディ上限 (64KB)、IP別トークンバケットレート制限の実装
- [x] **7. GitHub README**:
  - [x] 英語中心のポートフォリオ品質 [`README.md`](README.md) の作成（DES採用理由、Coordinate RNG採用理由、Retry Storm vs. CB のトレードオフ、実測実験結果テーブルを含む）
