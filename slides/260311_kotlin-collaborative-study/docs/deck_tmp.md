---
presentationID: 1GhH-gb2mbuQvKv3EgFAiBFmNsF0zMahdCwemrJmAmhM
title: 260311_kotlin-collaborative-study
breaks: true
codeBlockToImageCommand: "laminate"
defaults:
  - if: page >= 99999
    freeze: false
---

<!-- {"layout": "title", "freeze": true} -->

# LINEマンガを支えるCoroutine

## Kotlinで挑む3社3様の技術課題<br>株式会社コドモン 東京オフィス

---

<!-- {"layout": "eye-catch", "freeze": true} -->

# 自己紹介

---

<!-- {"layout": "profile", "freeze": true} -->

▼ 名前

本田 雄亮<br>

▼ 所属企業

LINE Digital Frontier株式会社<br>

▼ Xアカウント

@yyh_gl

![Image](images/profile.jpg)

---

<!-- {"layout": "content"} -->

# 今日話すこと

LINEマンガは5500万DLを突破し、
多くのユーザーにご利用いただいているサービスです。

そんなLINEマンガでは、パフォーマンスやリソース効率の観点から
非同期処理が有効に作用する場面が多々あります。

今日は、非同期処理においてLINEマンガで考えていることや工夫点を紹介します。

---

<!-- {"layout": "content"} -->

# アジェンダ

TBD

---

<!-- {"layout": "content"} -->

# 非同期処理に関する決まりごと

LINEマンガでは、非同期処理について以下のようなルールを決めている。

- **MUST**: Coroutine（kotlinx.coroutines）を使用
- **SHOULD**: `MangaScope`を使用

※ ADRとして定義

---

<!-- {"layout": "eye-catch"} -->

# 本発表の主役『MangaScope』

---

<!-- {"layout": "content"} -->

# MangaScopeとは？

LINEマンガで使用されている共通`CoroutineScope`。
Coroutine使用時には本Scopeの使用が推奨されている。

▼ `MangaScope`に関する4つのポイント

1. **SupervisorJob**: 1つの子コルーチンが失敗しても他の子コルーチンに<br>                       影響しない
2. **Dispatchers.IO**: JDBCなどブロッキングI/Oを安全に実行
3. **Observability**: 各種メトリクスやログ出力、トレーシングシステムを<br>                     サポート
4. **Graceful Shutdown**: 安全なシャットダウン

---

<!-- {"layout": "content"} -->

# Graceful Shutdownに対応

```kotlin
suspend fun close(wait: Duration) {
    log.info { "MangaScope($owner): Shutting down. deadline = $deadline" }

    // 1. 新規タスクをブロック
    shuttingDown = true

    // 2. 実行中タスクの完了を待つ (タイムアウト付き)
    while (true) {
        val first = supervisorJob.children.firstOrNull() ?: break
        val timeout = select<Boolean> {
            first.onJoin { false }
            onTimeout(deadline.toEpochMilli() - clock.millis()) { true }
        }
        if (timeout) break
    }

    // 3. 残存タスクにキャンセルを送信
    supervisorJob.cancel()
}
```

```text
ACTIVE → SHUTTING_DOWN → CANCELED
          (新規拒否)     (残存キャンセル)
```

---

<!-- {"layout": "eye-catch"} -->

# Coroutineに潜む課題

---

<!-- {"layout": "content"} -->

# ThreadLocal消失問題

Coroutineに移行するとき、最大の課題は「ThreadLocalが消える」こと。

LINEマンガでは3つのThreadLocalを伝搬させている。

| 層 | 対象 | 用途 |
|----|------|------|
| **MDCContext** | SLF4J MDC | ログにリクエストIDやユーザーID等を付与 |
| **MangaThreadContextElement** | アプリ独自ThreadLocal | ビジネスロジック用のコンテキスト |
| **Zipkin TraceContext** | 分散トレーシング | リクエストをまたいだトレース追跡 |

---

<!-- {"layout": "content"} -->

# MangaThreadContextElement の実装

```kotlin
internal class MangaThreadContextElement(
    val value: MutableMap<String, Any?> = MangaThreadContextHolder.getCopyForPropagation(),
) : AbstractCoroutineContextElement(Key), ThreadContextElement<MutableMap<String, Any?>?> {

    override fun updateThreadContext(context: CoroutineContext): MutableMap<String, Any?> {
        val oldState = MangaThreadContextHolder.getCopyForPropagation()
        MangaThreadContextHolder.set(value)
        return oldState  // 復元用に保存
    }

    override fun restoreThreadContext(context: CoroutineContext, oldState: MutableMap<String, Any?>?) {
        MangaThreadContextHolder.set(oldState ?: emptyMap())
    }
}
```

- `getCopyForPropagation()` でコピーを作成 → スレッド間で安全
- suspend/resume のたびに `updateThreadContext` / `restoreThreadContext` が呼ばれる

---

<!-- {"layout": "eye-catch"} -->

# Observability

---

<!-- {"layout": "content"} -->

# asyncWithMetrics / launchWithMetrics

個々のコルーチンの実行時間・成功/失敗を観測する仕組み。

TODO: ownerで「どのクラスが起動したコルーチンか」が常に追跡可能なことを追記

```kotlin
fun <T> CoroutineScope.asyncWithMetrics(
    name: String,                                      // コルーチン識別名 (メトリクス用)
    annotation: Map<String, String> = emptyMap(),      // Zipkin span 追加タグ
    context: CoroutineContext = EmptyCoroutineContext,
    start: CoroutineStart = CoroutineStart.DEFAULT,
    block: suspend CoroutineScope.() -> T,
): Deferred<T>
```

---

<!-- {"layout": "content"} -->

# asyncWithMetrics が自動で行うこと

```text
asyncWithMetrics("fetchUser") 呼び出し
  │
  ├─ 1. Zipkin span 生成: "coroutine: fetchUser"
  │     └─ tag: owner=クラス名, name=fetchUser, + annotation
  │
  ├─ 2. CoroutineName 設定: "クラス名#fetchUser"
  │
  ├─ 3. メトリクス記録:
  │     ├─ kotlin.coroutine.started   (Counter)
  │     ├─ kotlin.coroutine.completed (Counter)
  │     ├─ kotlin.coroutine.inflight  (Gauge)
  │     └─ kotlin.coroutine.timer     (Timer: 実行時間)
  │
  └─ 4. エラー時: span.error(e) でZipkinに記録
```

計測のない並列処理はブラックボックス。asyncWithMetricsは計測・トレース・命名を1行で提供する。

---

<!-- {"layout": "content"} -->

# カーディナリティ爆発防止の命名規則

```kotlin
// OK: 固定文字列をnameに使う
asyncWithMetrics(name = "fetchUser") { ... }

// OK: パラメータはannotationに入れる
asyncWithMetrics(
    name = "fetchUser",
    annotation = mapOf("bot_core_id" to botCoreId),  // ← Zipkin tagに入る
) { ... }

// NG: パラメータをnameに入れる → メトリクスのカーディナリティが爆発
asyncWithMetrics(name = "fetchUser_$userId") { ... }
```

---

<!-- {"layout": "eye-catch"} -->

# 並列度制御テクニック

---

<!-- {"layout": "content"} -->

# テクニック1: limitedParallelism

```kotlin
// 最もシンプルな並列度制御
runBlocking(Dispatchers.IO.limitedParallelism(4)) {
    items.forEach { item ->
        launch { process(item) }
    }
}
```

- **用途**: 同一スコープ内のコルーチン並列数を制限
- **特徴**: Dispatchers.IO のスレッドプールのサブセットとして動作

---

<!-- {"layout": "content"} -->

# テクニック2: Semaphore (ParallelRunner)

```kotlin
override fun <T> forEach(sequence: Sequence<T>, action: suspend (T) -> Unit) {
    runBlocking(context = scope.coroutineContext) {
        val semaphore = Semaphore(numThread)  // JVM Semaphore
        for (item in sequence) {
            if (!isActive) break

            semaphore.acquire()  // スレッドをブロックして待機
            val job = launchWithMetrics(name) { action(item) }
            job.invokeOnCompletion { cause ->
                if (cause != null) {
                    cancel("Cancelled by unexpected exception", cause)
                }
                semaphore.release()
            }
        }
    }
}
```

- **用途**: サイズ未知の `Sequence` に対する並列実行制御
- **特徴**: エラー時は scope 全体をキャンセル (fail-fast)

---

<!-- {"layout": "content"} -->

# テクニック3: DistributedSemaphore (Redis ベース)

```kotlin
interface DistributedSemaphore<KEY> {
    suspend fun <T> withPermit(key: KEY, action: suspend () -> T): T
    suspend fun acquire(key: KEY): Permit
    suspend fun tryAcquire(key: KEY): Permit?
}
```

```text
┌─Server A─┐   ┌─Server B─┐   ┌─Server C─┐
│ acquire() │   │ acquire() │   │ acquire() │
└─────┬─────┘   └─────┬─────┘   └─────┬─────┘
      │               │               │
      └───────────────┼───────────────┘
                      ▼
              ┌──── Redis ────┐
              │ maxPermit = 2 │
              │ A: active     │
              │ B: active     │
              │ C: waiting... │
              └───────────────┘
```

単一プロセス → Semaphore, 分散環境 → DistributedSemaphore。正しいツールを正しい粒度で。

---

<!-- {"layout": "content"} -->

# テスタビリティ設計

```kotlin
// Clock注入 — テスト時に差し替え可能
class MangaScope(
    internal val owner: Class<*>,
    private val customContext: CoroutineContext = EmptyCoroutineContext,
    private val clock: Clock = Clock.systemDefaultZone(),
)

// テスト例
val fixedClock = Clock.fixed(Instant.parse("2025-01-01T00:00:00Z"), ZoneId.of("Asia/Tokyo"))
val scope = MangaScope(javaClass, clock = fixedClock)
```

```kotlin
// @VisibleForTesting — 内部状態の検証
@VisibleForTesting
internal val coroutineStatsMap = ConcurrentHashMap<MetricsKey, CoroutineStats>()
```

テスタビリティはCoroutine導入時に設計段階から組み込む。

---

<!-- {"layout": "eye-catch"} -->

# まとめ

---

<!-- {"layout": "content"} -->

# LINEマンガにおけるCoroutine活用の全体像

```text
┌──────────────────────────────────────────────────────────────┐
│                       ADR-00022                              │
│        「並行/並列処理にはCoroutineを使う」                      │
├──────────────────────────────────────────────────────────────┤
│  ┌─ 基盤層 ──────────────────────────────────────────┐       │
│  │  MangaScope / ThreadLocal伝搬 / Observability     │       │
│  └────────────────────────────────────────────────────┘       │
│  ┌─ パターン層 ──────────────────────────────────────┐       │
│  │  Fan-out並列 / asyncプリフェッチ / バッチ処理       │       │
│  └────────────────────────────────────────────────────┘       │
│  ┌─ 制御層 ──────────────────────────────────────────┐       │
│  │  limitedParallelism / Semaphore / DistributedSemaphore │  │
│  └────────────────────────────────────────────────────┘       │
│  ┌─ 移行層 ──────────────────────────────────────────┐       │
│  │  MangaExecutors → 段階的に Coroutine へ移行        │       │
│  └────────────────────────────────────────────────────┘       │
│  ┌─ 品質層 ──────────────────────────────────────────┐       │
│  │  CoroutineQuizTest / Clock注入 / @VisibleForTesting │      │
│  └────────────────────────────────────────────────────┘       │
└──────────────────────────────────────────────────────────────┘
```

---

<!-- {"layout": "content"} -->

# Key Takeaways

1. **障害から学ぶ**: 実際のインシデントがADR策定の原動力になった
1. **基盤を整える**: 生のCoroutineScopeは使わない。MangaScopeで障害隔離・GracefulShutdown・トレーサビリティを確保
1. **ThreadLocalは消える**: 3層のコンテキスト伝搬を設計段階から組み込む
1. **計測なき並列はブラックボックス**: asyncWithMetrics でメトリクス・トレースを自動化
1. **段階的に移行**: デコレータパターンで既存と新規を共存させる
1. **落とし穴を共有**: CoroutineQuizTestでチームの学習コストを下げる


TODO ↓どっかに追記
もちろんCoroutineが簡単だとは一概に言えないが、
Threadよりは比較的扱いやすいと思う。


---

<!-- {"layout": "eye-catch"} -->

# ご清聴ありがとうございました

---

<!-- {"layout": "eye-catch"} -->

# Coroutine（kotlinx.coroutines）を使用

---

<!-- {"layout": "eye-catch"} -->

# つまり、<br>Threadは（直接）使用しない

---

<!-- {"layout": "content"} -->

# Threadは扱いが難しい

`java.lang.Thread`や`java.util.concurrent`, `kotlin.concurrent`などのThreadには罠が多い。<br>

よくあるのはThreadの閉じ忘れ。
1箇所でも閉じ忘れるとスレッドリソースがリークし続ける。
リークは即座にエラーにならず**静かに蓄積**する。
そして、ある日突然、CPU負荷上昇やメモリ枯渇として顕在化。

---

<!-- {"layout": "content"} -->

# Coroutineなら防げるの？

**Structured Concurrency**が閉じ忘れを構造的に防止する。

- 親スコープの終了時に子コルーチンが**自動キャンセル**される
  - 閉じ忘れが**原理的に起きにくい**

さらに

- `MangaScope.close()`による**Graceful Shutdown**で残存タスクを一括管理
- `asyncWithMetrics`でリソースリークを**早期検知**可能

`MangaScope`と`asyncWithMetrics`とは…？
