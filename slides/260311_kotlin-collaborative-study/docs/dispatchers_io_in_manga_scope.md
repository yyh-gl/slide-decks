# MangaScope における Dispatchers.IO の特徴

## 1. Dispatchers.IO とは

Kotlin Coroutines が提供する標準ディスパッチャの1つで、**ブロッキングI/O処理に最適化されたスレッドプール**上でコルーチンを実行する。

```
Dispatchers.Default  → CPU集約型（スレッド数 = CPUコア数）
Dispatchers.IO       → I/O集約型（スレッド数 = max(64, CPUコア数)）
Dispatchers.Main     → UIスレッド（Android/JavaFX用、サーバーでは通常不使用）
Dispatchers.Unconfined → 呼び出しスレッドで即実行（再開時は別スレッドの可能性あり）
```

## 2. MangaScope で Dispatchers.IO を選択した理由

**94行目**: `return Dispatchers.IO + threadLocalContext() + customContext + supervisorJob`

KDocにも明記されている:

> This scope is based on `Dispatchers.IO` to avoid unexpected event loop blocking.

LINE Manga サーバーで実行される典型的な処理:

| 処理 | 種類 | 例 |
|------|------|-----|
| DB アクセス | ブロッキングI/O | MySQL/Redis へのクエリ |
| 外部API呼び出し | ブロッキングI/O | LINE Platform API, 決済API |
| ファイル操作 | ブロッキングI/O | 画像処理、S3アクセス |

これらはすべて **スレッドをブロックする**ため、`Dispatchers.Default`（CPUコア数分のスレッド）では枯渇リスクがある。`Dispatchers.IO` なら最大64スレッド（デフォルト）で並行I/Oを捌ける。

## 3. Dispatchers.Default との内部的な関係

重要な実装詳細として、`Dispatchers.IO` と `Dispatchers.Default` は**同じスレッドプールを共有**している。

```
┌─────────────────────────────────────────┐
│          共有スレッドプール               │
│                                         │
│  ┌─────────────────┐ ┌───────────────┐  │
│  │ Default 用枠    │ │ IO 用枠       │  │
│  │ (CPUコア数)     │ │ (最大64)      │  │
│  └─────────────────┘ └───────────────┘  │
│                                         │
│  スレッド自体は共有・再利用される         │
└─────────────────────────────────────────┘
```

- スレッドの生成コストを抑えつつ、論理的にはCPU処理とI/O処理のスレッド枠を分離
- `Dispatchers.IO` のスレッドがCPU処理を奪い尽くすことを防ぐ設計

## 4. スレッド数の制御

`Dispatchers.IO` のスレッド上限はシステムプロパティで変更可能:

```
-Dkotlinx.coroutines.io.parallelism=128
```

デフォルトは `max(64, CPUコア数)`。大量の並行I/Oが必要な場合は調整を検討する。

## 5. MangaScope のコンテキスト構成

94行目の `+` 演算子でコンテキスト要素を合成している:

```kotlin
Dispatchers.IO                // どのスレッドプールで実行するか
+ threadLocalContext()        // ThreadLocal の伝播（MDC, Zipkin, MangaThreadContext）
+ customContext               // 利用者が追加するカスタムコンテキスト
+ supervisorJob               // 子コルーチンの障害分離
```

`+` は**後から足した要素が同じKeyの要素を上書き**する。Dispatcher は `ContinuationInterceptor` キーを使うため、`customContext` に別の Dispatcher を入れると `Dispatchers.IO` が上書きされる点に注意。

## 6. ThreadLocal 伝播との関係

`Dispatchers.IO` はコルーチンを**異なるスレッドで実行・再開**するため、ThreadLocal の値が失われる問題がある。MangaScope では `threadLocalContext()` で3つの ThreadLocal を明示的に伝播している:

```kotlin
private fun threadLocalContext() = MDCContext() +              // SLF4J MDC（ログ出力用）
    MangaThreadContextElement() +                              // アプリ固有のコンテキスト
    zipkinThreadLocal.asContextElement()                        // 分散トレーシング
```

これにより、スレッドが切り替わっても**ログのトレースID**や**リクエストコンテキスト**が正しく引き継がれる。

## 7. 発表で強調すべき点

| 観点 | 内容 |
|------|------|
| **なぜ IO か** | サーバーサイドの処理は大半がブロッキングI/O。Default だとスレッド枯渇する |
| **Default との違い** | スレッド数の上限が異なる。ただし同じプールを共有している |
| **ThreadLocal 問題** | IO ディスパッチャはスレッドを切り替えるため、ThreadLocal が消える。明示的な伝播が必須 |
| **Spring WebFlux との比較** | WebFlux はイベントループ（ノンブロッキング）。MangaScope は従来のブロッキング呼び出しをコルーチンで並列化するアプローチ |
| **チューニング** | `kotlinx.coroutines.io.parallelism` で上限調整可能。モニタリング指標と合わせて判断する |

## 8. よくある落とし穴（発表ネタ向き）

- **CPU集約処理を Dispatchers.IO で実行しない** — I/O用スレッドを占有してしまい、本来のI/O処理が待たされる。CPU処理には `Dispatchers.Default` を使う
- **`withContext(Dispatchers.IO)` の多用** — すでに `Dispatchers.IO` 上で動いているコルーチン内で `withContext(Dispatchers.IO)` を呼んでも無駄。コンテキスト切り替えのオーバーヘッドだけ発生する
- **limitedParallelism との使い分け** — 特定のリソース（例: コネクションプール上限10）に合わせてスレッド数を制限したい場合は `Dispatchers.IO.limitedParallelism(10)` が有効
