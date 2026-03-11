# LINEマンガにおけるCoroutine活用
## ~課題から設計・実装・運用・学びまで~

> **発表時間**: 15-20分
> **対象**: Coroutineの基本を知っている中級者エンジニア
> **発表者**: (名前)

---

## 目次

| # | トピック | セクション | 時間 |
|---|---------|-----------|------|
| 1 | なぜCoroutineを選んだのか | 導入 | 2分 |
| 2 | MangaScope: 共通CoroutineScope設計 | 設計編 | 2分 |
| 3 | ThreadLocal伝搬: 3層コンテキスト | 設計編 | 2分 |
| 4 | Observability: 自動計測とトレーシング | Observability編 | 3分 |
| 5 | 実践パターン集 | 実践パターン編 | 3分 |
| 8 | 並列度制御テクニック | 実践パターン編 | 2分 |
| 7 | 段階的移行戦略 | 移行戦略編 | 2分 |
| 6 | CoroutineQuizTestから学ぶ落とし穴 | 落とし穴 & テスト編 | 2分 |
| 9 | テスタビリティ設計 | 落とし穴 & テスト編 | 1分 |
| - | まとめ | まとめ | 1分 |

---

# 発表の導入

### 話すこと

> 「LINEマンガは大規模なトラフィックを扱うサービスです。
> そのため、パフォーマンスやリソース効率の観点からCoroutineが必要な場面が増えてきました。
> 今日は、LINEマンガでCoroutineを使うときにどういう工夫をしているかを紹介します。」

---

# 導入 (2分)

## トピック1: なぜCoroutineを選んだのか

### 話すこと

> 「LINEマンガでは2025年4月に ADR-00022 を策定し、並行/並列処理にはCoroutineを使うことを標準化しました。
> そのきっかけとなったのは、実際に起きた障害です。」

### 障害事例: 2025-03-24 バッチサーバー負荷上昇

- **現象**: バッチサーバーの負荷上昇に伴い、マニュアルPush通知が送信できなくなった
- **根本原因**: `runBlocking` の中で `runBlocking` をネストして呼び出していた
  - 外側の `runBlocking` がスレッドプールのスレッドを占有
  - 内側の `runBlocking` が同じスレッドプールからスレッドを取得しようとしてデッドロック的状態に
- **教訓**: `Thread` を直接扱うと、リソース管理のミスが障害に直結する

### ADR-00022 の意思決定

| 観点 | Coroutine (Pros) | Thread (Cons) |
|------|------------------|---------------|
| **効率性** | 軽量でスレッドより効率が良い | OSレベルの原始的な処理で非効率 |
| **リソース管理** | 低レベルのリソース管理が不要 | 正しく管理することが難しい |
| **Kotlin統合** | Kotlinの高度な型推論と併用可能 | N/A |
| **学習コスト** | 概念がやや難しい (対策あり: QuizTest等) | N/A |

### 決定事項

```
MUST: 並行/並列処理には Coroutine (kotlinx.coroutines) を利用する
      → Thread の直接利用は禁止 (外部ライブラリ連携は例外)

SHOULD: プロジェクト共通の MangaScope を利用する
```

> **スライドポイント**: 「障害 → ADR策定 → 全社標準化」という流れを強調

---

# 設計編 (4分)

## トピック2: MangaScope — 共通CoroutineScope設計

### 話すこと

> 「ADRで『MangaScopeを使うべき』と書きましたが、そのMangaScopeとは何者でしょうか？
> SupervisorJob + Dispatchers.IO を軸に、Graceful Shutdownまでカバーしたプロダクション向けScopeです。」

### 設計の3つの柱

#### 1. SupervisorJob — 子コルーチンの障害隔離

```kotlin
// MangaScope.kt
class MangaScope(
    internal val owner: Class<*>,
    private val customContext: CoroutineContext = EmptyCoroutineContext,
    private val clock: Clock = Clock.systemDefaultZone(),
) : CoroutineScope, AutoCloseable {

    private val supervisorJob = SupervisorJob()

    override val coroutineContext: CoroutineContext
        get() {
            if (shuttingDown) {
                error("Scope is under shutting down")
            }
            return Dispatchers.IO + threadLocalContext() + customContext + supervisorJob
        }
}
```

- **SupervisorJob**: 1つの子コルーチンが失敗しても他の子コルーチンに影響しない
- **Dispatchers.IO**: JDBCなどブロッキングI/Oを安全に実行
- **customContext**: 利用者がディスパッチャーを上書き可能

#### 2. Graceful Shutdown

```kotlin
// MangaScope.kt — close メソッド
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

**状態遷移**:
```
ACTIVE → SHUTTING_DOWN → CANCELED
          (新規拒否)     (残存キャンセル)
```

#### 3. `owner` によるトレーサビリティ

```kotlin
val scope = MangaScope(javaClass)  // ← ownerとして自クラスを渡す
```

- メトリクス・ログ・Zipkin spanに `owner` 情報が付与される
- 「どのクラスが起動したコルーチンか」が常に追跡可能

> **スライドポイント**: 「プロダクションでは生のCoroutineScopeを使わない。障害隔離・Graceful Shutdown・トレーサビリティがセットで必要。」

---

## トピック3: ThreadLocal伝搬 — 3層コンテキスト

### 話すこと

> 「Coroutineに移行するとき、最大の課題は『ThreadLocalが消える』問題です。
> LINEマンガでは3つのThreadLocalを伝搬させています。」

### 3層のThreadLocal伝搬

```kotlin
// MangaScope.kt
private fun threadLocalContext() = MDCContext() +              // 層1: SLF4J MDC
    MangaThreadContextElement() +                              // 層2: アプリ独自コンテキスト
    zipkinThreadLocal.asContextElement()                       // 層3: Zipkin トレースコンテキスト
```

| 層 | 対象 | 仕組み | 用途 |
|----|------|--------|------|
| **MDCContext** | SLF4J MDC | kotlinx-coroutines-slf4j 標準 | ログにリクエストIDやユーザーID等を付与 |
| **MangaThreadContextElement** | アプリ独自ThreadLocal | カスタム `ThreadContextElement` 実装 | ビジネスロジック用のコンテキスト |
| **Zipkin TraceContext** | 分散トレーシング | `asContextElement()` | リクエストをまたいだトレース追跡 |

### MangaThreadContextElement の実装

```kotlin
// MangaScope.kt
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

**ポイント**:
- `getCopyForPropagation()` でコピーを作成 → スレッド間で安全
- suspend/resume のたびに `updateThreadContext` / `restoreThreadContext` が呼ばれる
- Zipkin の ThreadLocal はリフレクションで取得 (ライブラリ側がpublic APIを提供していないため)

```kotlin
// MangaScope.kt — Zipkin ThreadLocal の取得
private val zipkinThreadLocal: ThreadLocal<TraceContext> = run {
    val declaredField = ThreadLocalCurrentTraceContext::class.java.getDeclaredField("DEFAULT")
    declaredField.trySetAccessible()
    declaredField.get(ThreadLocalCurrentTraceContext::class.java) as ThreadLocal<TraceContext>
}
```

> **スライドポイント**: 「Coroutine導入 = ThreadLocal伝搬の設計が必須。これを怠るとログが追えない・トレースが切れる」

---

# Observability編 (3分)

## トピック4: 自動計測とトレーシング

### 話すこと

> 「MangaScopeだけでは十分ではありません。個々のコルーチンの実行時間・成功/失敗を観測する仕組みが必要です。
> `asyncWithMetrics` / `launchWithMetrics` がその答えです。」

### asyncWithMetrics / launchWithMetrics

```kotlin
// MangaScope.kt
fun <T> CoroutineScope.asyncWithMetrics(
    name: String,                                      // コルーチン識別名 (メトリクス用)
    annotation: Map<String, String> = emptyMap(),      // Zipkin span 追加タグ
    context: CoroutineContext = EmptyCoroutineContext,
    start: CoroutineStart = CoroutineStart.DEFAULT,
    block: suspend CoroutineScope.() -> T,
): Deferred<T>
```

### 自動で行われること

```
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

### 実装の内部構造

```kotlin
// MangaScope.kt — 内部コンテキスト構築
private fun CoroutineScope.buildInternalContext(
    name: String,
    annotation: Map<String, String>,
    context: CoroutineContext,
): MangaCoroutineInternalContext {
    val ownerName = if (this is MangaScope) owner.name else "UNKNOWN"
    return MangaCoroutineInternalContext(
        ownerName = ownerName,
        stopwatch = Stopwatch.createStarted(),
        span = currentTracer?.startScopedSpan("coroutine: $name")?.also { span ->
            span.tag("owner", ownerName)
            span.tag("name", name)
            annotation.forEach { key, value -> span.tag(key, value) }
        },
        coroutineContext = CoroutineName("$ownerName#$name") + context,
        coroutineStats = coroutineStatsMap
            .computeIfAbsent(MetricsKey(ownerClass = ownerName, name = name)) { CoroutineStats(it) },
    )
}
```

### カーディナリティ爆発防止の命名規則

```kotlin
// OK: 固定文字列をnameに使う
asyncWithMetrics(name = "fetchUser") { ... }

// OK: パラメータはannotationに入れる
asyncWithMetrics(
    name = "fetchUser",
    annotation = mapOf("bot_core_id" to botCoreId),  // ← Zipkin tagに入る
) { ... }

// NG: パラメータをnameに入れる → メトリクスのカーディナリティが爆発
asyncWithMetrics(name = "fetchUser_$userId") { ... }  // ← METRICS_PREFIX.xxx のタグが無限に増える
```

> **スライドポイント**: 「計測のない並列処理はブラックボックス。asyncWithMetricsは計測・トレース・命名を1行で提供する」

---

# 実践パターン編 (5分)

## トピック5: 実践パターン集

### 話すこと

> 「ここからは、LINEマンガで実際に使われているCoroutineの4つの実践パターンを紹介します。」

### パターン1: Fan-out並列 — ホーム画面コンポーネント生成

ホーム画面は20以上のコンポーネント（バナー、ランキング、おすすめ等）で構成される。
各コンポーネントの生成を並列に実行。

```kotlin
// ComponentGeneratorExecutor.kt — getGeneratedComponentMapV2
fun getGeneratedComponentMapV2(
    sectionTypeInOrder: List<HomeTitleCondition>,
    params: GenerateParams,
    blockFilterContext: BlockProductFilterContext,
    parallelism: Int,
): Map<SectionType, List<HomeComponent>> {
    val componentMap = ConcurrentHashMap<SectionType, List<HomeComponent>>()

    val context = CoroutineName("getGeneratedComponentMapV2") +
        MDCContext() +
        DataSourceTypeContextHolder.TYPE_STORE.asContextElement(DataSourceType.SLAVE)

    // limitedParallelism で並列度を制御
    runBlocking(Dispatchers.IO.limitedParallelism(parallelism) + context) {
        withTimeout(executionTimeout) {
            for ((index, section) in sectionTypeInOrder.withIndex()) {
                launchWithMetrics(name = "getComponentsV2#${section.sectionType}") {
                    componentMap[section.sectionType] =
                        getComponentsV2(section, params, blockFilterContext, coordinator)
                }
            }
        }
    }
    return componentMap
}
```

**ポイント**:
- `limitedParallelism(parallelism)` でDBコネクション等のリソースを保護
- `withTimeout` で全体のタイムアウトを設定
- `ConcurrentHashMap` で結果を安全に集約
- `launchWithMetrics` で各コンポーネントの生成時間を個別に計測

### パターン2: asyncプリフェッチ — RequestContext生成

リクエストの初期化時に、独立したデータソースからの取得を並列化。

```kotlin
// RequestContextGenerator.kt — authenticated
fun authenticated(request: HttpServletRequest, ...): RequestContext {
    return runBlocking {
        // ...同期的な初期化処理...

        // 独立したデータ取得を並列で開始
        val userTargetingMetadataDeferred = asyncWithMetrics("userTargetingMetadata") {
            userTargetingMetadataRepository.get(memberId = memberRecord.id)
        }
        val targetingAdditionalMetadataDeferred = asyncWithMetrics("targetingAdditionalMetadata") {
            targetingAdditionalMetadataGenerator.generate(memberRecord, libraGroups)
        }

        // ...他の同期処理を実行...

        // 結果を合流
        val userTargetingMetadata = userTargetingMetadataDeferred.await()
        val targetingAdditionalMetadata = targetingAdditionalMetadataDeferred.await()

        // RequestContext を構築
        RequestContext(...)
    }
}
```

**ポイント**:
- `async` で即座にバックグラウンド実行を開始
- その間に他の同期処理を実行 (レイテンシ短縮)
- `await()` で結果を合流

### パターン3: バッチ処理 — BatchQueueingContext

大量データの chunk 分割 + パイプライン処理。

```kotlin
// BatchQueueingContext.kt — 利用イメージ
batching(
    batchingName = "importProducts",
    scope = mangaScope,
    parallelism = 4,       // 0=逐次, 1=パイプライン, >1=並列パイプライン
    chunkSize = 100,
    batching = { chunk -> repository.bulkInsert(chunk) },
) {
    // このブロック内で addTask を呼ぶと自動的にchunk化される
    products.forEach { product ->
        addTask(product)
    }
}
// ← ブロック終了時に自動flush + scope.close(wait)
```

**内部の仕組み**:
```
addTask() → LinkedBlockingQueue
    ↓ (chunkSize に達したら)
flush() → launchWithMetrics で非同期バッチ実行
    ↓ (parallelism で並列度制御)
close() → 残タスクflush → scope.close(wait) で完了待ち
```

### パターン4: バックグラウンドジョブ — DynamicRateLimiter

アプリケーション起動中に永続的に動くバックグラウンドコルーチン。

```kotlin
// DynamicRateLimiter.kt
class DynamicRateLimiter(...) : RateLimiter, AutoCloseable {
    companion object {
        private val SCOPE = MangaScope(DynamicRateLimiter::class.java)

        private fun startBackgroundJob(weakReference: WeakReference<DynamicRateLimiter>): Job {
            return SCOPE.launchWithMetrics("backgroundJob") {
                while (true) {
                    val rateLimiter = weakReference.get() ?: break  // GC対応
                    delay(rateLimiter.updateInterval.toKotlinDuration())
                    rateLimiter.updateRate()
                }
            }
        }
    }

    override fun close() {
        runBlocking { backgroundJob.cancelAndJoin() }
    }
}
```

**ポイント**:
- `WeakReference` でGCによるリソースリークを防止
- companion object の `SCOPE` を共有 (クラス全体で1つ)
- `cancelAndJoin()` で安全に停止

---

## トピック8: 並列度制御テクニック

### 話すこと

> 「並列処理は速度を上げますが、無制限に並列化するとDBやAPIに過負荷をかけます。
> LINEマンガでは3つの並列度制御パターンを使い分けています。」

### テクニック1: `Dispatchers.IO.limitedParallelism(n)`

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
- **実例**: ホーム画面コンポーネント生成、バッチインポート処理

### テクニック2: `java.util.concurrent.Semaphore` (ParallelRunner)

```kotlin
// ParallelRunner.kt
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

### テクニック3: `DistributedSemaphore` (Redis ベース)

```kotlin
// DistributedSemaphore.kt
interface DistributedSemaphore<KEY> {
    suspend fun <T> withPermit(key: KEY, action: suspend () -> T): T
    suspend fun acquire(key: KEY): Permit
    suspend fun tryAcquire(key: KEY): Permit?
}
```

- **用途**: 複数サーバーインスタンス間での同時実行制御
- **特徴**:
  - Redis上で permit を管理
  - Heartbeat デーモン (コルーチン) で状態維持
  - Exponential backoff によるリトライ
  - `stateExpire` でクラッシュしたノードの permit を自動回収

```
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

> **スライドポイント**: 「単一プロセス → Semaphore, 分散環境 → DistributedSemaphore。正しいツールを正しい粒度で。」

---

# 移行戦略編 (2分)

## トピック7: 段階的移行戦略

### 話すこと

> 「既存のJavaベースのExecutorServiceコードをすべて一度にCoroutineに書き換えるのは非現実的です。
> LINEマンガでは、デコレータパターンを活用した段階的な移行を行いました。」

### MangaExecutors — デコレータによるコンテキスト伝搬

既存のExecutorServiceもMDC/MangaThreadContext/Zipkinの伝搬が必要。
MangaExecutorsはデコレータチェーンでこれを実現。

```kotlin
// MangaExecutors.kt — commonWrap
private fun commonWrap(original: ExecutorService, ...): ExecutorService {
    // Layer 1: Micrometer メトリクス監視
    val statsMonitored = ExecutorServiceMetrics.monitor(Metrics.globalRegistry, original, name, tags)

    // Layer 2: MDC (SLF4J) コンテキスト伝搬
    val mdcAwareExecutorService = MdcAwareExecutorDecorator(statsMonitored)

    // Layer 3: MangaThreadContext 伝搬
    val mangaThreadContextAware = MangaThreadContextAwareExecutorDecorator(mdcAwareExecutorService)

    // Layer 4: Zipkin span 生成
    return ZipkinExecutorDecorator(mangaThreadContextAware, ownerClass.simpleName + '#' + name)
}
```

**デコレータの積層構造**:
```
呼び出し元
  └→ ZipkinExecutorDecorator       ← Zipkin span 自動生成
      └→ MangaThreadContextAware   ← アプリコンテキスト伝搬
          └→ MdcAwareExecutor      ← ログコンテキスト伝搬
              └→ ExecutorServiceMetrics ← メトリクス計測
                  └→ 実際の ThreadPoolExecutor
```

### 段階的移行アプローチ

```
Phase 1: MangaExecutors (既存Executor + デコレータ)
  └─ 既存コードはそのまま動作しつつ、コンテキスト伝搬を確保

Phase 2: MangaScope + asyncWithMetrics (新規コード)
  └─ 新規実装からCoroutineに移行、ThreadLocal伝搬はCoroutineContext側で対応

Phase 3: 既存コードの段階的書き換え
  └─ ParallelRunner等の抽象化を経由して、呼び出し側に影響なく内部をCoroutine化
```

### MangaExecutors と MangaScope の対比

| 機能 | MangaExecutors (旧) | MangaScope (新) |
|------|---------------------|-----------------|
| MDC伝搬 | MdcAwareExecutorDecorator | MDCContext() |
| ThreadContext伝搬 | MangaThreadContextAwareExecutorDecorator | MangaThreadContextElement |
| Zipkin伝搬 | ZipkinExecutorDecorator | zipkinThreadLocal.asContextElement() |
| メトリクス | ExecutorServiceMetrics | asyncWithMetrics / launchWithMetrics |
| Graceful Shutdown | 手動管理 | MangaScope.close() |

> **スライドポイント**: 「同じ3層のコンテキスト伝搬を、ExecutorService版とCoroutine版の両方で提供。既存コードを壊さずに移行できる。」

---

# 落とし穴 & テスト編 (3分)

## トピック6: CoroutineQuizTestから学ぶハマりポイント

### 話すこと

> 「Coroutineの落とし穴を学ぶために、LINEマンガではCoroutineQuizTestというコードクイズを用意しています。
> Coroutineを本番で使い始める前に、このクイズに答えられることを推奨しています。」

### Quiz 1: runBlocking + async の並列実行

```kotlin
fun testDelay() {
    val runBlockingResult = runBlocking {
        val result1 = async {
            doSomething("Task1")  // 3秒かかる (Thread.sleep x 3)
            1
        }
        val result2 = async {
            doSomething("Task2")  // 3秒かかる
            2
        }
        val total = listOf(result1, result2).awaitAll().sum()
        return@runBlocking total
    }
}

private fun doSomething(taskName: String) {
    Thread.sleep(1_000)  // ← blocking!
    Thread.sleep(1_000)
    Thread.sleep(1_000)
}
```

**Q: このコードの実行時間は？**

**A: 約6秒** (3秒ではない!)

- `runBlocking` はデフォルトで **単一スレッド** のディスパッチャーを使う
- `Thread.sleep` はスレッドをブロックする
- 2つの `async` は同じスレッドを使おうとするので、結局逐次実行になる
- **対策**: `Dispatchers.IO` を使うか、`delay()` を使う

### Quiz 2: `Job()` の罠 — 親子関係の切断

```kotlin
fun testRunBlockingWaitsChildCoroutine() {
    // パターンA: runBlocking が子コルーチンの完了を待つ
    runBlocking {
        launch {
            delay(1_000)
            log.info { "Launch finishing" }  // ← 出力される
        }
    }
    // runBlocking はすべての子コルーチンの完了を待ってから戻る

    // パターンB: Job() で親子関係を切断
    runBlocking(Dispatchers.IO + CoroutineName("dedicated Job()")) {
        launch(Job()) {  // ← 新しいJobを渡すと親子関係が切れる！
            delay(1_000)
            log.info { "Launch finishing" }  // ← 出力されない可能性あり
        }
    }
    // runBlocking は子コルーチンを待たずに即座に戻る
}
```

**ポイント**: `launch(Job())` は親子関係を切断する。構造化並行性 (Structured Concurrency) が崩壊し:
- 親が子の完了を待たない
- 親のキャンセルが子に伝搬しない
- → **リソースリーク** の原因

### よくある落とし穴まとめ

| 落とし穴 | 原因 | 対策 |
|---------|------|------|
| 並列のはずが逐次 | `runBlocking` のデフォルトディスパッチャー + `Thread.sleep` | `Dispatchers.IO` + `delay()` |
| 子が完了しない | `launch(Job())` で親子切断 | `Job()` を渡さない |
| ThreadLocalが消える | Coroutine のスレッド切り替え | `MangaScope` の ThreadLocal伝搬 |
| ネストした `runBlocking` | スレッドプールの枯渇 → デッドロック | `coroutineScope { }` を使う |

---

## トピック9: テスタビリティ設計

### 話すこと

> 「最後に、Coroutineを使ったコードのテストしやすさについて。
> LINEマンガではClock注入や @VisibleForTesting を活用しています。」

### Clock注入 — 時間に依存するコードのテスト

```kotlin
// MangaScope のコンストラクタ
class MangaScope(
    internal val owner: Class<*>,
    private val customContext: CoroutineContext = EmptyCoroutineContext,
    private val clock: Clock = Clock.systemDefaultZone(),  // ← テスト時に差し替え可能
)
```

```kotlin
// テスト例 (概念)
val fixedClock = Clock.fixed(Instant.parse("2025-01-01T00:00:00Z"), ZoneId.of("Asia/Tokyo"))
val scope = MangaScope(javaClass, clock = fixedClock)
// → close() の deadline 計算等が決定論的にテスト可能
```

### @VisibleForTesting — 内部状態の検証

```kotlin
// MangaScope.kt
@VisibleForTesting
internal val coroutineStatsMap = ConcurrentHashMap<MetricsKey, CoroutineStats>()

@VisibleForTesting
internal class MangaThreadContextElement(...)
```

- `internal` + `@VisibleForTesting` でテストコードからのみアクセス可能
- 本番コードからの誤用を防ぎつつテスタビリティを確保

### ADRで定められたテストの原則

- **AssertJ** を使用 (`assertThat(result).isEqualTo(expected)`)
- 時間は `Clock` 注入 (`Instant.now()` の直接呼び出し禁止)
- スリープは `Sleeper` ユーティリティ (`Thread.sleep` 禁止)

> **スライドポイント**: 「テスタビリティはCoroutine導入時に設計段階から組み込む」

---

# まとめ (1分)

## LINEマンガにおけるCoroutine活用の全体像

```
┌────────────────────────────────────────────────────────────────────┐
│                         ADR-00022                                  │
│          「並行/並列処理にはCoroutineを使う」                          │
├────────────────────────────────────────────────────────────────────┤
│                                                                    │
│  ┌─ 基盤層 ──────────────────────────────────────────────────┐     │
│  │  MangaScope          SupervisorJob + Dispatchers.IO       │     │
│  │  ThreadLocal伝搬     MDC + MangaThreadContext + Zipkin    │     │
│  │  Observability       asyncWithMetrics / launchWithMetrics │     │
│  └────────────────────────────────────────────────────────────┘     │
│                                                                    │
│  ┌─ パターン層 ──────────────────────────────────────────────┐     │
│  │  Fan-out並列      ホーム画面コンポーネント生成              │     │
│  │  asyncプリフェッチ  RequestContext生成                      │     │
│  │  バッチ処理        BatchQueueingContext                    │     │
│  │  バックグラウンド   DynamicRateLimiter, HeartbeatDaemon    │     │
│  └────────────────────────────────────────────────────────────┘     │
│                                                                    │
│  ┌─ 制御層 ──────────────────────────────────────────────────┐     │
│  │  limitedParallelism    単一プロセス内の並列度制御            │     │
│  │  Semaphore             サイズ未知シーケンスの並列度制御      │     │
│  │  DistributedSemaphore  分散環境の並列度制御                 │     │
│  │  DynamicRateLimiter    動的レート制御                       │     │
│  └────────────────────────────────────────────────────────────┘     │
│                                                                    │
│  ┌─ 移行層 ──────────────────────────────────────────────────┐     │
│  │  MangaExecutors (デコレータパターン)                        │     │
│  │  → 段階的に Coroutine へ移行                                │     │
│  └────────────────────────────────────────────────────────────┘     │
│                                                                    │
│  ┌─ 品質層 ──────────────────────────────────────────────────┐     │
│  │  CoroutineQuizTest  落とし穴の事前学習                      │     │
│  │  Clock注入          テスタビリティ設計                      │     │
│  │  @VisibleForTesting 内部状態の検証                          │     │
│  └────────────────────────────────────────────────────────────┘     │
│                                                                    │
└────────────────────────────────────────────────────────────────────┘
```

## Key Takeaways

1. **障害から学ぶ**: 実際のインシデントがADR策定の原動力になった
2. **基盤を整える**: 生のCoroutineScopeは使わない。MangaScopeで障害隔離・GracefulShutdown・トレーサビリティを確保
3. **ThreadLocalは消える**: 3層のコンテキスト伝搬を設計段階から組み込む
4. **計測なき並列はブラックボックス**: asyncWithMetrics でメトリクス・トレースを自動化
5. **段階的に移行**: デコレータパターンで既存と新規を共存させる
6. **落とし穴を共有**: CoroutineQuizTestでチームの学習コストを下げる

---

## 参考資料

| 資料 | パス |
|------|------|
| ADR-00022 | `adr/ADR-00022_use_coroutine_for_run_concurrent_parallel_process.md` |
| MangaScope | `commons/parallel/src/main/java/.../coroutine/MangaScope.kt` |
| CoroutineQuizTest | `commons/parallel/src/test/java/.../CoroutineQuizTest.kt` |
| ComponentGeneratorExecutor | `external-api/src/main/java/.../home/ComponentGeneratorExecutor.kt` |
| BatchQueueingContext | `commons/parallel/src/main/java/.../batching/BatchQueueingContext.kt` |
| ParallelRunner | `commons/parallel/src/main/java/.../parallel_runner/ParallelRunner.kt` |
| DistributedSemaphore | `commons/parallel/src/main/java/.../sempahore/DistributedSemaphore.kt` |
| DynamicRateLimiter | `commons/parallel/src/main/java/.../rate_limiter/DynamicRateLimiter.kt` |
| MangaExecutors | `commons/parallel/src/main/java/.../executors/MangaExecutors.kt` |
