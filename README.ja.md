# kuiperbelt
===

WebSocketとHTTP/1.1の相互変換プロキシサーバ

## 概要

kuiperbeltは以下の機能を提供します。

* WebSocketの接続に対するエンドポイント
* WebSocketの接続要求を登録された他のHTTPサーバにプロキシする一連の手続き
* 指定したWebSocketの接続に対してHTTP APIによるメッセージの送信
* HTTP APIによる接続の切断
* 接続数やメッセージ数などの状態API

上記の機能によってkuiperbeltは次に挙げるようなケースで有効に働きます。

* Preforkモデルでアプリケーションサーバを構築するケースでの容易なWebSocketの接続維持
* アプリケーションサーバの更新などによって起こるクライアントへの接続の切断が許容できない場合のWebSocketコネクションの維持

## 使用方法

### 1. インストール

Goをインストールした状態かつ`$GOPATH/bin`が$PATHに含まれている状態で以下のコマンドを入力します。

```sh
$ go get github.com/mackee/kuiperbelt/cmd/ekbo
```

このコマンドが成功すると`ekbo`コマンドが使えるようになります。

### 2. 設定ファイルの記述

作業ディレクトリを用意し、以下のような設定ファイルをYAMLで記述します。

```yaml
---
# kuiperbeltは12345番ポートで起動します
port: 12345
# 接続時コールバックはローカルホストの12346番ポートへプロキシします
callback:
  connect: "http://localhost:12346/connect"
```

例ではこれを`config.yml`で保存します。

### 3. アプリケーションサーバの実装

アプリケーションサーバでは`/connect`のエンドポイントを実装します。

#### 接続情報

このエンドポイントへはクライアントがkuiperbeltへ接続してきた際のquery stringやheaderがそのままプロキシされてきます。
また、それに追加して2つのヘッダが添付されます。

* `X-Kuiperbelt-Session`
  * 接続確立以後に接続に対してメッセージを送信する際に用いる識別子です
  * 前述の設定ファイルで`session_header`というキーを設定することでヘッダの名前を変えることが出来ます
* `X-Kuiperbelt-Endpoint`
  * kuiperbelt自体のendpoint情報 例: `localhost:12345`
  * 複数のkuiperbeltサーバを用いて冗長性を確保した構成にする場合にどの接続がどのサーバへ接続されたかを識別するために用います
  * 冗長化構成の場合では前述の`X-Kuiperbelt-Session`の値とセットでデータベースなどに保存することを推奨します

ここでは手動でメッセージを送るため、

#### 認証

`/connect`にコールバックが来る時点ではクライアントへの接続はWebSocketへUpgradeはされていません。
そこでこのリクエストに対して200以外のレスポンスを返すことによりWebSocketの接続を拒否することができます。
この機構を利用し認証を実装することが出来ます。

#### 参考実装

[_example/app.psgi](https://github.com/mackee/kuiperbelt/blob/master/_example/app.psgi)Perl/Plackでの参考実装があります。

#### 4. kuiperbeltの起動

作業ディレクトリ内で以下のコマンドを実行することによりkuiperbeltが起動します。

```sh
$ ekbo -config=config.yml
```

#### 5. WebSocketでの接続

ここでは[wscat](https://github.com/websockets/wscat)を用いて接続を試みてみます。インストールはリンク先のドキュメントを参照してください。
