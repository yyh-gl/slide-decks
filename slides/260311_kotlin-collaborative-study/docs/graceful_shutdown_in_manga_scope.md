# MangaScope における Graceful Shutdown の設計

## 1. Graceful Shutdown とは

アプリケーション終了時に、**実行中の処理を安全に完了させてから停止する**手法。対比として Forceful Shutdown（即時強制終了）がある。

```
Forceful Shutdown:  SIGKILL → プロセス即死（処理途中のデータが失われる可能性）
Graceful Shutdown:  SIGTERM → 新規受付停止 → 実行中処理の完了待ち → 停止
```

サーバーサイドでは、リクエスト処理中にプロセスが死ぬと**データ不整合**や**ユーザー体験の劣化**を招くため、Graceful Shutdown は重要な設計要素となる。

## 2. MangaScope の Graceful Shutdown 全体像

`MangaScope` は `AutoCloseable` を実装しており、`close()` で Graceful Shutdown を開始する。

**97〜99行目** — エントリポイント:

```kotlin
override fun close() {
    runBlocking { close(wait = Duration.ofSeconds(10)) }
}
```

デフォルトのタイムアウトは **10秒**。`runBlocking` で呼び出し元スレッドをブロックし、shutdown 完了を待つ。

## 3. Shutdown シーケンス詳細

`close(wait: Duration)` (101〜142行目) は、4つのフェーズで構成される。

```
                  deadline（10秒後）
                       ↓
時間軸 ─────────────────────────────────→

Phase 1: Gate Close
  shuttingDown = true
  ┌─────────────────────────────────────┐
  │ 新規タスクの起動を拒否              │
  │ （coroutineContext getter で error） │
  └─────────────────────────────────────┘

Phase 2: Drain（水抜き）
  ┌─────────────────────────────────────┐
  │ 実行中の子コルーチンを1つずつ join  │
  │ deadline を超えたら break           │
  └─────────────────────────────────────┘

Phase 3: Cancel
  supervisorJob.cancel()
  ┌─────────────────────────────────────┐
  │ 残存タスクに CancellationException  │
  └─────────────────────────────────────┘

Phase 4: Cancel Wait
  ┌─────────────────────────────────────┐
  │ キャンセル処理の完了を待機          │
  │ deadline を超えたらログ出力して終了 │
  └─────────────────────────────────────┘
```

### Phase 1: Gate Close（107行目）

```kotlin
shuttingDown = true
```

`@Volatile` な `shuttingDown` フラグを `true` にする。以降、`coroutineContext` プロパティへのアクセスは `IllegalStateException` をスローし、新規コルーチンの起動を拒否する。

```kotlin
override val coroutineContext: CoroutineContext
    get() {
        if (shuttingDown) {
            error("Scope is under shutting down")  // IllegalStateException
        }
        return Dispatchers.IO + threadLocalContext() + customContext + supervisorJob
    }
```

これにより**新しい仕事が流入しない**ことを保証する。

### Phase 2: Drain（110〜130行目）

```kotlin
while (true) {
    val first = supervisorJob.children.firstOrNull()
        ?: break  // 子がいなくなったら完了

    val timeout = select<Boolean> {
        first.onJoin { false }                                    // 子の完了
        onTimeout(deadline.toEpochMilli() - clock.millis()) { true }  // タイムアウト
    }

    if (timeout) {
        log.errorWithStackTrace { "..." }
        break
    }
}
```

ポイント:

| 要素 | 説明 |
|------|------|
| `supervisorJob.children` | SupervisorJob の直下にある全子 Job のシーケンス |
| `firstOrNull()` | 子が残っていなければ `null` → `break` でループ脱出 |
| `select` | 「子の完了」と「タイムアウト」を**非同期に競争**させる |
| タイムアウト計算 | `deadline - 現在時刻` で残り時間を動的に計算 |

`firstOrNull()` で**1つずつ**待つ設計のため、全子コルーチンが順に完了するまでループする。1つ完了するたびに次の子を取得し直す。

### Phase 3: Cancel（133行目）

```kotlin
supervisorJob.cancel()
```

Drain フェーズで完了しなかった子コルーチンに対し、`CancellationException` を発行する。コルーチンは次の **suspension point** でキャンセルを検知して終了する。

### Phase 4: Cancel Wait（134〜141行目）

```kotlin
val timeout = select<Boolean> {
    supervisorJob.onJoin { false }
    onTimeout(deadline.toEpochMilli() - clock.millis()) { true }
}
if (timeout) {
    log.errorWithStackTrace { "MangaScope($owner): Cacncelation timeout." }
}
```

`supervisorJob` 自体の join を待ち、全子コルーチンのキャンセル処理が完了するのを確認する。ここでもタイムアウトを設け、無限待ちを防止している。

## 4. タイムアウト設計

Phase 2〜4 は**同一の deadline を共有**している。これにより、全体のタイムアウトが保証される。

```
|← ──────── 最大10秒 ──────── →|
|  Phase 2 (Drain)  | Phase 3+4 |
|   可変長           | 残り時間   |
```

- Phase 2 で8秒使った場合、Phase 3+4 には2秒しか残らない
- Phase 2 で全タスクが即完了すれば、Phase 3+4 にほぼ10秒使える
- どのフェーズでタイムアウトしても、エラーログを出力してプロセスは終了に向かう

## 5. `select` による非同期タイムアウトの仕組み

`kotlinx.coroutines.selects.select` は、複数のサスペンド操作を**競合的に待機**し、最初に完了したものの結果を返す。

```kotlin
val timeout = select<Boolean> {
    first.onJoin { false }           // 子が完了したら false
    onTimeout(remainingMs) { true }  // 時間切れなら true
}
```

これは Go 言語の `select` ステートメントに似た概念で、`Thread.sleep` + ポーリングよりも効率的かつ正確。

## 6. `AutoCloseable` と Spring の連携

`MangaScope` は `AutoCloseable` を実装しているため:

- **Spring Bean として登録**した場合、ApplicationContext の shutdown 時に自動で `close()` が呼ばれる
- **try-with-resources / use** でも利用可能

```kotlin
// Spring Bean として登録（推奨）
@Bean
fun mangaScope() = MangaScope(SomeService::class.java)
// → Spring shutdown 時に自動で close() が呼ばれる

// 手動管理
MangaScope(javaClass).use { scope ->
    runBlocking(scope.coroutineContext) {
        launch { /* work */ }
    }
}
```

## 7. 状態遷移図

KDoc に記載されている状態遷移:

```
ACTIVE ──────→ SHUTTING_DOWN ──────→ CANCELED
  │                 │                    │
  │ close() 呼出   │ 子タスク完了待ち   │ supervisorJob.cancel()
  │                 │ 新規タスク拒否     │ 全子コルーチン停止
  │                 │                    │
  ▼                 ▼                    ▼
 通常稼働         排水中               停止完了
```

| 状態 | shuttingDown | supervisorJob | 新規タスク | 実行中タスク |
|------|:---:|:---:|:---:|:---:|
| ACTIVE | `false` | active | 受付可 | 実行中 |
| SHUTTING_DOWN | `true` | active | 拒否 | 完了待ち |
| CANCELED | `true` | cancelled | 拒否 | キャンセル済 |

## 8. 発表で強調すべき点

| 観点 | 内容 |
|------|------|
| **なぜ Graceful か** | 処理途中のリクエストを安全に完了させ、データ不整合を防ぐ |
| **4フェーズ設計** | Gate Close → Drain → Cancel → Cancel Wait の段階的停止 |
| **deadline 共有** | 全フェーズで同じ deadline を使い、全体のタイムアウトを保証 |
| **select の活用** | ポーリングではなく非同期競合で効率的にタイムアウト判定 |
| **@Volatile** | `shuttingDown` フラグのスレッド間可視性を保証。ただしアトミック性は保証しない（後述の注意点参照） |
| **Spring 連携** | `AutoCloseable` により、Spring shutdown 時に自動呼び出し |

## 9. よくある落とし穴（発表ネタ向き）

- **suspension point がないコルーチンはキャンセルされない** — Phase 3 で `cancel()` を呼んでも、CPU集約ループのように suspension point を持たないコルーチンは停止しない。`yield()` や `ensureActive()` を挿入するか、`isActive` を定期的にチェックする必要がある
- **`shuttingDown` の競合** — `@Volatile` はスレッド間の可視性を保証するが、「チェックしてからタスク起動」の間にフラグが変わる可能性がある（check-then-act 問題）。ただし、この場合は新規タスクが1つ漏れる程度で実害は小さい
- **Drain フェーズの `firstOrNull()`** — `children` は snapshot ではなくライブなシーケンスのため、子が完了するたびに内容が変わる。`firstOrNull()` で1つずつ処理するのは正しいアプローチ
- **`runBlocking` の注意** — `close()` は `runBlocking` で呼び出し元をブロックするため、イベントループスレッド上で呼ぶとデッドロックのリスクがある。Spring の shutdown hook は通常別スレッドで実行されるため問題にならない
