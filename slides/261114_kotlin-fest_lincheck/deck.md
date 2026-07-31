---
presentationID: "1j4Vr55oJmcPzLcfxdpAHC4cm9KFTG_UYxnntCt29mTA"
title: 261114_kotlin-fest_lincheck
breaks: true
codeBlockToImageCommand: "laminate"
defaults:
  - if: true
    freeze: false
---

<!-- {"layout": "title"} -->

# 祈るテストから、探索するテストへ

## 〜Lincheckではじめる並行処理テスト〜

---

<!-- {"layout": "section"} -->

# SECTION 1

## 自己紹介

---

<!-- {"layout": "aboutme"} -->

# 自己紹介

## ▼ 名前　

本田雄亮

## ▼ 所属企業　

LINE Digital Frontier株式会社

## ▼ Xアカウント　

@yyh_gl
![Image](images/profile.jpg)

<!-- https://x.com/yyh_gl -->

---

<!-- {"layout": "eye-catch"} -->

# 🐾 会社紹介 🐾

---

# LINE Digital Frontier株式会社

『LINEマンガ』を開発しています📚️

<!-- TODO: DL数・収益ランキング等の実績値を発表時点の最新情報に更新する -->

![Image](images/linemanga_logo.png)
![Image](images/linemanga_dl.png)
![Image](images/corp_qr.png)

<!-- https://ldfcorp.com/ -->

---

<!-- {"layout": "agenda"} -->

# アジェンダ

1. 再現しない並行処理バグ
1. なぜ再現しないのか
1. Lincheckのアプローチ
1. suspend関数もテストできる
1. 実演：バグを確実に再現する
1. まとめ

---

<!-- {"layout": "title-and-body-and-dog-comment"} -->

# サンプルコードおよび参考資料

本発表で使用するサンプルコード（Kotlin Playgroundのリンク）や
参考資料のリンクはスライドのスピーカーノートに記載しています。

お好きなタイミングでご参照ください。

## 手元で動かせるやつ、うれしい

---

<!-- {"layout": "eye-catch"} -->

# 🐾 2. 再現しない並行処理バグ 🐾

## 諦めていませんか？

<!-- time: 1:30 -->

---

<!-- {"layout": "title-and-body-and-code"} -->

# 正しく見えるコード

2つのスレッドで、カウンタをそれぞれ10万回インクリメントするだけのコード。
結果は200000になるはず。

```kotlin
var counter = 0

fun main() {
    val t1 = thread { repeat(100_000) { counter++ } }
    val t2 = thread { repeat(100_000) { counter++ } }
    t1.join()
    t2.join()
    println(counter) // 200000のはず……？
}
```

<!-- TODO: Kotlin Playgroundのリンクを追加 -->

---

<!-- {"layout": "title-and-body-and-dog-comment"} -->

# 実行してみると

同じコード・同じ入力なのに、実行するたびに結果がずれる。

```
1回目: 200000
2回目: 200000
3回目: 187562 ← ！？
4回目: 200000
```

しかも、もう一度実行すると直る（ように見える）。

## 今日は機嫌がいいみたい

<!-- TODO: 実測値に差し替える -->

---

<!-- {"layout": "title-and-body-and-conclusion"} -->

# 「祈るテスト」の日常

心当たりはないでしょうか。

- CIでだけ、たまに落ちるテスト
- 「とりあえずrerunしたら通った」
- 再現しないので調査を打ち切り、`@Ignore`や自動リトライで対処

並行処理のバグは「再現しない」という一点で、通常のバグと性質が根本的に異なる。
私たちはその検証を諦め、テストの成功を祈っている。

## テストの成否が運任せ＝「祈るテスト」

---

<!-- {"layout": "eye-catch"} -->

# 🐾 3. なぜ再現しないのか 🐾

<!-- time: 3:30 -->

---

# 疑問

`counter++`はたった1行。
1つの操作に見えるが、なぜ結果がずれるのだろうか。

```kotlin
counter++
```

---

# 答え

`counter++`は1つの操作ではない。
実際には3つのステップに分かれて実行される。

```kotlin
counter++
// 実際にはこの3ステップに分かれる
// ① counterの現在値を読む   (read)
// ② 読んだ値に1を足す       (add)
// ③ 結果をcounterへ書き戻す (write)
```

---

<!-- {"layout": "title-and-body-without-dog"} -->

# 解説：消えるインクリメント

2つのスレッドのステップが、次の順序で交互に実行（インターリービング）されると
片方のインクリメントが消えてしまう。

| 順 | Thread 1 | Thread 2 | counter |
|---|---|---|---|
| 1 | read → 0 | | 0 |
| 2 | | read → 0 | 0 |
| 3 | add → 1 | | 0 |
| 4 | | add → 1 | 0 |
| 5 | write 1 | | 1 |
| 6 | | write 1 | **1** |

2回インクリメントしたはずなのに、結果は1。
これがロストアップデートと呼ばれる不具合。

---

<!-- {"layout": "title-and-body-and-conclusion"} -->

# 実行順序の組み合わせ爆発

インターリービングの総数は膨大。
2スレッドが各3ステップを実行するだけでも20通り、各10ステップなら184756通りにもなる。
実際のコードではさらに桁違いの数になる。

通常のテストは1回の実行につき、無数にある順序のうちの1つしか試せない。
バグを踏む順序を引き当てるかどうかは運次第。
だからリトライしても再現しないし、リトライで通っても不具合が直ったわけではない。

## 1回の実行は、無数の順序のうちのたった1つ

<!-- 20 = C(6,3)、184756 = C(20,10) -->

---

<!-- {"layout": "eye-catch"} -->

# 🐾 4. Lincheckのアプローチ 🐾

## 祈りから、探索へ

<!-- time: 6:00 -->

---

# Lincheckとは

JetBrains製の並行処理テストフレームワーク。

- kotlinx.coroutines自体の品質保証に組み込まれ、Mutex・Semaphore・Channelといった
  コアの検証に実際に使われている（CAV 2023採録論文）
- JCToolsなど著名な並行ライブラリでも、未知の不具合を発見した実績がある
- 最新はLincheck 3系（2026年5月に3.6がリリース）

Kotlinのcoroutinesの品質を、影で支えているツールと言える。

<!-- https://github.com/JetBrains/lincheck -->
<!-- TODO: CAV 2023論文へのリンクを追加 -->

---

# 疑問

バグを検出するには、まず「正しい」を定義する必要がある。
並行コードが「正しい」とは、そもそもどういうことだろうか。

---

# 答え

並行に実行した結果が、操作を何らかの順序で1つずつ逐次実行した結果と一致すること。

この性質を線形化可能性（linearizability）と呼ぶ。

---

<!-- {"layout": "title-and-body-without-dog"} -->

# 解説：線形化可能性

各操作が、実行期間中のどこか一点で「一瞬で」起きたとみなせるなら
その並行実行は正しい。

先ほどのカウンタの例で言うと、`inc()`と`inc()`を並行に呼んだとき
「1回目→2回目」の順でも「2回目→1回目」の順でも、逐次実行なら結果は必ず2になる。

結果が1になった実行は、どんな順序の逐次実行とも一致しない。
つまり線形化可能性を満たしておらず、バグと判断できる。

---

<!-- {"layout": "title-and-body-and-conclusion"} -->

# 「結果の列挙」から「仕様」へ

JVMの類似ツールJCStressでは、起こりうる実行結果を人間が列挙する。

```java
// JCStress: 許容する結果を人間が書き並べる
@Outcome(id = "1, 1", expect = FORBIDDEN,  desc = "ロストアップデート")
@Outcome(id = "1, 2", expect = ACCEPTABLE, desc = "Thread1が先")
@Outcome(id = "2, 1", expect = ACCEPTABLE, desc = "Thread2が先")
```

Lincheckは列挙しない。
「対象クラスを1スレッドで動かした結果」＝逐次仕様（sequential specification）との
等価性で、正しさを自動判定する。

## 正しさは「列挙」ではなく「仕様」で語る

---

# 疑問

Lincheckのテストはどう書くのだろうか。
膨大なインターリービングを、1つずつテストケースに書くのだろうか。

---

<!-- {"layout": "title-and-body-and-code"} -->

# 答え：「何を」だけ宣言する

テストしたい操作に`@Operation`を付けるだけでよい。
シナリオそのものは書かない。

```kotlin
import org.jetbrains.lincheck.datastructures.ModelCheckingOptions
import org.jetbrains.lincheck.datastructures.Operation

class CounterTest {
    private var counter = 0

    @Operation
    fun inc() = ++counter

    @Operation
    fun get() = counter

    @Test
    fun modelCheckingTest() = ModelCheckingOptions().check(this::class)
}
```

<!-- TODO: パッケージ名・APIシグネチャをLincheck 3.6の実物で検証する -->

---

<!-- {"layout": "title-and-body-and-dog-comment"} -->

# 解説：残りは全部Lincheckがやる

宣言した操作から、Lincheckが以下を行う。

1. シナリオ生成：操作の並行な組み合わせを自動生成
2. 実行：さまざまなインターリービングで実行
3. 判定：結果が逐次仕様と等価か（線形化可能か）を自動チェック

「どうテストするか」はフレームワークの仕事。
開発者は「何をテストするか」だけを書けばよい。

## テストシナリオ、1つも書いてない

---

# 疑問

非決定な実行順序を、Lincheckはどうやって「試す」のだろうか。
結局のところ運任せなのではないか。

---

# 戦略①：ストレステスト

生成したシナリオを大量に（デフォルトで数万回規模）実行し
多様なインターリービングを引き当てにいく戦略。

- 実際のスレッドで実行するため、実環境に近い挙動を確認できる
- ただし確率的な探索であり、網羅性は保証されない

```kotlin
@Test
fun stressTest() = StressOptions().check(this::class)
```

---

# 戦略②：モデルチェッキング

Lincheckがスレッドの実行順序そのものを制御し
コンテキストスイッチをどこで起こすかを系統的に探索する戦略。

- 運に頼らない。同じ不具合を毎回・確実に検出できる
- 失敗時は「どの順序で壊れたか」のトレースを出力できる（後ほど実演）

```kotlin
@Test
fun modelCheckingTest() = ModelCheckingOptions().check(this::class)
```

---

<!-- {"layout": "title-and-body-and-conclusion"} -->

# 2つの戦略の使い分け

| | ストレステスト | モデルチェッキング |
|---|---|---|
| 探索方法 | 大量実行（確率的） | 順序を制御して系統的に探索 |
| 再現性 | 低い | 高い（決定的） |
| 失敗トレース | なし | あり |
| 向いている用途 | 実環境に近い挙動の確認 | ロジックの不具合検出・デバッグ |

基本はモデルチェッキングで探索し、補完としてストレステストを回すのが定石。

## 祈らない。探索する

---

<!-- {"layout": "eye-catch"} -->

# 🐾 5. suspend関数もテストできる 🐾

## coroutinesを持つKotlinだからこそ

<!-- time: 11:00 -->

---

<!-- {"layout": "title-and-body-and-code"} -->

# 疑問

Kotlinの並行処理と言えばcoroutines。
`Channel`の`send`/`receive`の「正しさ」は、どう定義すればよいだろうか。

相手がいなければ`receive()`はサスペンドする。
これは失敗ではなく、仕様どおりの正しい挙動。
しかし逐次実行では`receive()`が永遠にサスペンドしてしまい
「逐次実行と等価」という線形化可能性の定義が、そのままでは通用しない。

```kotlin
val ch = Channel<Int>() // capacity = 0（ランデブー）

// receiveが先に呼ばれたら？
// → sendが来るまでサスペンドするのが「正しい」
```

---

# 答え

Lincheckは線形化可能性をサスペンドする操作へ拡張している
（dual data structuresの理論）。

操作を「要求の登録」と「完了」の2つに分けて捉えることで
途中でサスペンドし、あとで再開して完了する操作も逐次仕様と突き合わせられる。

<!-- TODO: dual data structures論文へのリンクを追加 -->

---

<!-- {"layout": "title-and-body-and-code"} -->

# 解説：書き方は変わらない

開発者から見れば、`@Operation`をsuspend関数に付けるだけ。
理論の難しさはLincheckが吸収してくれる。

```kotlin
class RendezvousChannelTest {
    private val ch = Channel<Int>()

    @Operation
    suspend fun send(value: Int) = ch.send(value)

    @Operation
    suspend fun receive() = ch.receive()

    @Test
    fun modelCheckingTest() = ModelCheckingOptions().check(this::class)
}
```

<!-- TODO: サスペンドしたまま終わった操作の結果表示形式を実物で検証する -->

---

<!-- {"layout": "title-and-body-and-conclusion"} -->

# Kotlinだから生まれた検証技術

- JCStressにsuspend関数は扱えない。「サスペンドすることこそ正しい」操作は
  結果の列挙では表現できない
- coroutinesという言語機能を持つKotlinだからこそ必要になり、生まれた検証技術
- そしてkotlinx.coroutines自体（Mutex・Semaphore・Channel）が
  実際にこの仕組みでテストされている

## coroutinesの品質は、Lincheckが支えている

---

<!-- {"layout": "eye-catch"} -->

# 🐾 6. 実演 🐾

## 冒頭のバグを確実に再現する

<!-- time: 14:30 -->

---

<!-- {"layout": "title-and-body-without-dog"} -->

# 実演でお見せするもの

1. 冒頭のカウンタの不具合をLincheckで検出する——「たまに」ではなく毎回
2. 失敗トレースを読む——どの実行順序で壊れたのか
3. IntelliJ IDEAでトレースをステップ実行する

（ここからライブデモ。以降のスライドはデモ用のバックアップを兼ねる）

---

<!-- {"layout": "title-and-body-and-code"} -->

# デモコード：任意の並行コードをテストする

Lincheck 3で追加された`Lincheck.runConcurrentTest`を使うと
`@Operation`スタイルにせず、任意の並行コードをそのままテストできる。

```kotlin
import org.jetbrains.lincheck.Lincheck
import kotlin.concurrent.thread

class CounterBugTest {
    @Test
    fun test() = Lincheck.runConcurrentTest {
        var counter = 0
        val t1 = thread { counter++ }
        val t2 = thread { counter++ }
        t1.join()
        t2.join()
        assertEquals(2, counter) // 毎回ここで不具合を検出できる
    }
}
```

冒頭のコードでは10万回回して「たまに」結果がずれた。
Lincheckなら各スレッド1回のインクリメントでも、壊れる順序を探索して確実に検出する。

<!-- TODO: Lincheck.runConcurrentTestの正確なパッケージ・シグネチャを実物で検証する -->

---

<!-- {"layout": "title-and-body-and-dog-comment"} -->

# 失敗トレースを読む

Lincheckは失敗を検出すると、どのインターリービングで壊れたかを出力する。

```
= Concurrent test failed =
The following interleaving leads to the error:
| Thread 1          | Thread 2          |
|                    | counter.READ: 0   |
|                    | switch            |
| counter.READ: 0    |                   |
| counter.WRITE(1)   |                   |
|                    | counter.WRITE(1)  |
```

Thread 2が読んだ直後にThread 1へスイッチし、両者が同じ0を読んでいた。
バグの経緯がそのまま書いてある。

## 犯人の自白付き

<!-- TODO: 実際の失敗トレース出力に差し替える -->

---

<!-- {"layout": "title-and-body-and-conclusion"} -->

# 導入の敷居は下がっている

依存を1行足すだけで使い始められる。

```kotlin
dependencies {
    testImplementation("org.jetbrains.lincheck:lincheck:3.6") // 座標は要検証
}
```

IntelliJ IDEAとの統合で、失敗トレースをデバッガでステップ実行できる。
Lincheck 3で「並行データ構造の作者のためのツール」から「一般開発者のツール」へ進化した。

## 明日から自分のコードに使える

<!-- TODO: IntelliJ IDEA統合の正式名称・手順を確認し、スクリーンショットを追加 -->

---

<!-- {"layout": "eye-catch"} -->

# 🐾 7. まとめ 🐾

<!-- time: 18:00 -->

---

<!-- {"layout": "agenda"} -->

# 本日の振り返り

- 並行処理のバグは「再現しない」——原因はインターリービングの組み合わせ爆発
- Lincheckのアプローチ
  - 正しさは線形化可能性（逐次仕様との等価性）で定義する
  - テストは「何を」だけ宣言する
  - 実行順序はストレステストとモデルチェッキングで探索する
- suspend関数も検証できる——coroutinesを持つKotlinだから生まれた技術
- 失敗トレースで「どの順序で壊れたか」まで分かる

---

<!-- {"layout": "title-and-body-and-conclusion"} -->

# 並行処理に向き合う3つの考え方

1. 正しさの定義から始める——テストの前に「何が正しいか」を言葉にする
2. 「何を」テストするかを宣言する——「どう」はフレームワークに任せる
3. 非決定性を祈りではなく探索で扱う——運任せをやめ、制御下に置く

この考え方は、Lincheckを使わない場面でも、並行処理と向き合う武器になる。

## 祈るテストから、探索するテストへ

---

# 終わりのあいさつ

次に「たまに落ちるテスト」に出会ったとき、rerunボタンの前で祈る以外の選択肢を
思い出してもらえたら嬉しいです。

並行処理のバグは怖いものですが、「正しさを定義し、探索する」という道具があれば
きちんと向き合えます。
まずは自分のプロジェクトの並行処理コードに、Lincheckのテストを1つ書いてみてください。
きっと発見があるはずです。

---

<!-- {"layout": "eye-catch"} -->

# 🐾 Thank you 🐾

## ご清聴ありがとうございました！

---

<!-- {"layout": "eye-catch"} -->

# 🐾 補足資料 🐾

---

# 導入手順の詳細

<!-- TODO: Gradle依存追加の全量、JVM起動フラグの要否、JUnit 4/5どちらで動くかを実物で検証して記載する -->

---

# Lincheckの注意点・制約

- モデルチェッキングが前提とするメモリモデルの制約。だからこそストレステストとの併用が推奨される
- 実行時間：探索するため、通常のユニットテストより時間がかかる
- I/Oや外部リソースを含むコードは、そのままでは対象にしにくい

<!-- TODO: メモリモデル前提の最新状況、invocations数調整などCIでの運用上の工夫を検証して記載する -->

---

# 線形化可能性、もう少しだけ

- Herlihy & Wing（1990）による原典
- dual data structures（Scherer & Scott）
- Lincheckが対応する一貫性モデル

<!-- TODO: 対応する一貫性モデルの最新状況を検証して記載する -->

---

# 参考資料

- Lincheck公式リポジトリ・ドキュメント
- CAV 2023論文「Lincheck: A Practical Framework for Testing Concurrent Data Structures on JVM」
- KotlinConf 2023ワークショップ
- JCStress
- kotlinx.coroutinesにおけるLincheckテストの実例
- IntelliJ IDEAトレースデバッガのドキュメント

<!-- TODO: 各項目のURLを追加する -->
