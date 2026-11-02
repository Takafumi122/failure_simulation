# FailureLab 要件定義書 (Phase 1 & Phase 2)

## 1. 概要と目的
**FailureLab** は、分散システム（マイクロサービスアーキテクチャ）において下流サービスの障害が上流へどのように波及（カスケード障害）するかを可視化・分析し、さらに **Retry**、**Circuit Breaker**、**Fail-Fast (Timeout)** といったレジリエンス戦略の効果と副作用（Retry Storm等）を同一障害条件下で比較できるブラウザベースのシミュレーターである。

- **Phase 1のゴール (完了)**:
  - 障害注入（停止・遅延・確率的失敗）
  - 仮想時間を用いた離散イベントシミュレーション（DES: Discrete-Event Simulation）
  - メトリクス計算（全体およびサービス別の成功率、エラー率、p95レイテンシ、呼び出し数、キュー長など）
  - ネットワーク依存関係図（`@xyflow/react`）と時系列再生UIの表示
- **Phase 2のゴール (本フェーズ)**:
  - **Retry (Exponential Backoff + Jitter)**、**Circuit Breaker (Closed / Open / Half-Open)**、**Fail-Fast (Attempt単位の短縮Timeout)** を実際の離散イベントシミュレーションに統合。
  - 同一シナリオ・同一トラフィック・同一乱数系列のもとで5つの戦略（`baseline`, `retry`, `circuit_breaker`, `retry_circuit_breaker`, `fail_fast`）を一括比較できるようにする。
  - 論理リクエスト成功率と Attempt 数を厳密に分離し、Retry Amplification Factor、CB拒否数、Breaker状態遷移タイムライン、最大Worker使用率などを実イベントから計算・可視化する。

---

## 2. システム構成と技術スタック
- **Backend**: Go (`net/http`, 標準ライブラリ中心、`container/heap`, `math/rand`, `hash/fnv` 等)
- **Frontend**: React, TypeScript, Vite, `@xyflow/react`, Recharts, Tailwind CSS, Lucide React
- **インフラ / 永続化**: DBなし（ステートレスなインメモリ計算）。実際のマイクロサービスプロセスや実ネットワーク通信障害は発生させず、すべてGoエンジン内の仮想時間上で高速・決定論的にシミュレートする。

---

## 3. 対象トポロジー
直列5ノードの呼び出しチェーンを対象とする。

`Client` -> `API Gateway` -> `Product API` -> `Recommendation` -> `Feature DB`

| ID | 表示名 | 役割 | デフォルト baseLatencyMs | デフォルト maxConcurrent | デフォルト queueLimit | デフォルト timeoutMs |
|---|---|---|---|---|---|---|
| `client` | Client | リクエスト発生源（エッジ） | 0 ms | 無制限 (0) | 0 | 2000 ms |
| `api-gateway` | API Gateway | 認証・ルーティング | 10 ms | 50 | 100 | 1200 ms |
| `product-api` | Product API | 商品詳細取得・集約 | 20 ms | 35 | 70 | 900 ms |
| `recommendation` | Recommendation | レコメンド推論・取得 | 35 ms | 25 | 50 | 650 ms |
| `feature-db` | Feature DB | 特徴量データストア | 15 ms | 20 | 40 | 450 ms |

---

## 4. Phase 1 実装の検証と Phase 2 に向けた設計整合性の記録 (Step 0)

Phase 1 実装を精査し、Phase 2 の要件に対して以下のギャップ（不整合になる箇所）を確認したため、互換性を維持したまま拡張する：

1. **乱数消費順序の干渉問題**:
   - *Phase 1の挙動*: 単一の `*rand.Rand` インスタンスから到着時刻生成・処理時間ジッター・障害判定を逐次引き出していた。このまま Retry を導入すると、Attempt 増加に伴い乱数消費位置がずれ、後続リクエストの障害判定や処理時間が戦略間で変わってしまう。
   - *Phase 2の改善*: `(seed, requestID, serviceID, attempt, purpose)` から決定論的に乱数値を算出するハッシュベースの乱数生成器（Deterministic Coordinate RNG）を導入する。これにより、どの戦略でも同じ `(requestID, serviceID, attempt)` に対して **100% 同一の到着時刻・ローカル処理時間・確率的エラー判定** が保証される。
2. **Attempt単位のTimeoutと論理リクエスト全体の期限の区別**:
   - *Phase 1の挙動*: 子スパンのデッドラインは `min(parentDeadlineMs, now + targetService.timeoutMs)` のみで計算していた。
   - *Phase 2の改善*: 呼び出し元が設定する `PerAttemptTimeoutMs`（Fail-Fast時など）と、上流スパン全体の `ParentDeadlineMs` を明確に区別する。1回目の Attempt が短い `PerAttemptTimeoutMs` でタイムアウトしても、親スパンの `ParentDeadlineMs` に余裕があれば Retry 戦略は次の Attempt をスケジュールできる。
3. **Retryable / Non-Retryable エラーの分離と CB Open 時の Retry 抑止**:
   - 下流の確率的失敗 (`fault_error`, `base_error`)、停止 (`fault_outage`)、キュー溢れ (`queue_overflow`)、Attempt タイムアウト (`timeout`) は Retryable とする。
   - 一方、Circuit Breaker による拒否 (`circuit_open`) は **Non-Retryable** とし、`retry_circuit_breaker` 戦略において Breaker が Open の場合は追加の Retry を即座に中止して Fail-Fast させる。
4. **Timeout / キャンセル後の下流処理と二重完了防止**:
   - 上流スパンが先にタイムアウトした場合でも、既に下流で開始された Attempt は下流ワーカーを即座には強制終了できない（一般的なHTTP/RPCにおいてクライアントが切断してもサーバー側クエリが走り続ける現象、いわゆる Orphaned Work）モデルを採用する。ただし、遅れて完了した下流スパンが親スパンを二重に完了させたり、ワーカーを二重解放したりしないよう `Span.Completed` および `HoldingWorker` フラグで厳密にガードする。

---

## 5. Phase 2 機能要件

### 5.1 Retry 戦略 (`retry`)
- **適用境界**: デフォルトでは `recommendation -> feature-db` の呼び出し境界に適用し、設定（`resilience.targetEdge` または呼び出しペア指定）により任意の依存関係へ適用可能とする。
- **パラメータ**:
  - `maxRetries`: 最大リトライ回数（デフォルト `3`、初回呼び出しを含め最大 `1 + maxRetries` Attempts）
  - `initialBackoffMs`: 初回バックオフ基準時間（デフォルト `25ms`）
  - `maxBackoffMs`: 最大バックオフ上限（デフォルト `250ms`）
  - `backoffMultiplier`: 指数倍率（デフォルト `2.0`）
  - `jitter`: Jitter有無（デフォルト `true`、Equal Jitter: `[0.5 * expBackoff, 1.0 * expBackoff]` をSeedから決定論的に算出）
- **仮想時間とリソース影響**:
  - Retry のバックオフ待機時間は `RETRY_BACKOFF_COMPLETE` イベントとして仮想時間で進める。待機中も呼び出し元（`Recommendation`）のワーカー枠は保持され続ける。
  - Retry ごとに下流（`Feature DB`）への新規 `REQUEST_ARRIVAL` イベントを発行し、下流のワーカー・キュー・メトリクスに実負荷を与える。
  - 次の Attempt 開始予定時刻 `now + backoffMs` が呼び出し元の `DeadlineMs` を超える場合は、無駄な Retry を行わず即座に失敗を返す。

### 5.2 Circuit Breaker 戦略 (`circuit_breaker`)
- **適用境界**: 呼び出し境界（`caller -> callee`、デフォルト `recommendation -> feature-db`）ごとに独立した状態機械を持つ。
- **状態機械 (Closed -> Open -> Half-Open -> Closed)**:
  - **Closed**: 通常状態。下流呼び出しを許可する。失敗時に連続失敗カウンタ `consecutiveFailures` をインクリメントし、成功時に `0` にリセットする。`consecutiveFailures >= consecutiveFailuresThreshold`（デフォルト `5`）に達したら **Open** へ遷移する。
  - **Open**: 遮断状態。下流への呼び出しを即座に拒否（`ReasonCircuitOpen`）し、下流へのイベントを発行しない。Open 遷移時刻から `openTimeoutMs`（デフォルト `3000ms`）経過後の最初の呼び出し判定時に **Half-Open** へ遷移する。
  - **Half-Open**: 試行状態。同時に試行可能なリクエスト数を `halfOpenMaxCalls`（デフォルト `3`）に制限する（枠を超える同時リクエストは `ReasonCircuitOpen` で拒否）。
    - 試行リクエストが1つでも失敗したら即座に **Open** へ戻す（タイマーリセット）。
    - 試行リクエストが連続して `halfOpenSuccessThreshold`（デフォルト `2`、または `halfOpenMaxCalls`）回成功したら **Closed** へ復帰する。
- **状態遷移記録**:
  - すべての状態遷移（時刻、境界名、`fromState -> toState`、理由）を `BreakerTransition` およびイベントログ (`CIRCUIT_STATE_CHANGE`) に記録する。

### 5.3 Retry + Circuit Breaker 複合戦略 (`retry_circuit_breaker`)
- Retry と Circuit Breaker を同時に有効化する。
- 一時的なスパイクエラーに対しては Retry で救済しつつ、下流が深刻な停止・高遅延に陥った場合は Circuit Breaker が Open となり、**Breaker Open 中は追加の Retry を一切発生させない（Retry Storm の防止）**。

### 5.4 Timeout / Fail-Fast 戦略 (`fail_fast`)
- 下流呼び出し（デフォルト `recommendation -> feature-db`）における Attempt 単位のタイムアウト（`failFastTimeoutMs`、デフォルト `120ms`）を通常（`450ms`）より短く設定する。
- 下流が遅延障害を起こした際、ワーカーが長時間拘束される前に素早くタイムアウトさせて上流の待機時間を短縮する。

### 5.5 5戦略の同一条件比較 (`POST /api/simulations/compare`)
- 1回のリクエストで `["baseline", "retry", "circuit_breaker", "retry_circuit_breaker", "fail_fast"]`（または指定された戦略リスト）を同一シナリオに対して実行し、結果を横並びで返す。
- Phase 1 と同様、`strategies` 未指定時はデフォルトで5戦略すべて（またはリクエスト指定分）を返却し、`results[0]` が `baseline` となるため Phase 1 のテスト・互換性も完全に維持される。

### 5.6 メトリクス要件 (Step 5)
各戦略について実イベントから以下を算出する：
- **論理リクエスト成功率 (`successRate`) / エラー率 (`errorRate`)**: クライアントが発行した論理リクエスト単位で計算し、Attempt数と混同しない。
- **レイテンシ (`avgLatencyMs`, `p50LatencyMs`, `p95LatencyMs`, `p99LatencyMs`)**: 論理リクエストのEnd-to-End所要時間から算出。
- **総下流呼び出し数 (`totalDownstreamCalls`)**: 全バックエンドサービスへの総呼び出し Attempt 数。
- **Retry回数 (`totalRetries`)**: 初回呼び出し（Attempt 1）を除いた追加リトライ総数。
- **Retry Amplification Factor (`retryAmplificationFactor`)**: 対象境界（または全体）における `総Attempt数 / 初回リクエスト数`（Retryなしなら `1.00`）。
- **Circuit Breaker 拒否数 (`circuitBreakerRejections`)**: CB Open / Half-Open 制限により下流を呼ばずに即座に拒否された回数（下流自体の失敗とは別カウント）。
- **Timeout数 (`timeoutCount`)**、**Queue溢れ数 (`queueOverflowCount`)**。
- **最大Worker使用率 (`maxWorkerUtilization`)**、**最大Queue長 (`maxQueueLength`)**。
- **Breaker状態遷移イベント一覧 (`breakerTransitions`)**。
- **イベント上限到達時の明示ステータス**: `executionMeta.truncated = true` および `executionMeta.status = "truncated"`（正常完了時は `"completed"`）を返し、上限打ち切り時は未完了リクエストを成功扱いしない。
