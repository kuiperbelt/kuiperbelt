# kuiperbelt
===

WebSocketとHTTP/1.1の相互変換プロキシサーバ

## 概要

kuiperbeltは以下の機能を提供します。

* WebSocketの接続に対するエンドポイント
* WebSocketの接続要求を登録された他のHTTPサーバにプロキシする一連の手続き
* 指定したWebSocketの個々の接続に対してHTTP APIによるメッセージの送信および接続の切断
* 接続数やメッセージ数などの状態API

上記の機能によってkuiperbeltは次に挙げるようなケースで有効に働きます。

* Preforkモデルでアプリケーションサーバを構築するケースでの容易なWebSocketの接続維持
* アプリケーションサーバの更新などによって起こるクライアントへの接続の切断が許容できない場合のWebSocketコネクションの維持

## 使用方法

### 1. インストール

Goをインストールした状態かつ`$GOPATH/bin`が$PATHに含まれている状態で以下のコマンドを入力します。

```sh
$ go get github.com/kuiperbelt/kuiperbelt/cmd/ekbo
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
また、それに追加して以下のヘッダが添付されます。

* `X-Kuiperbelt-Endpoint`
  * kuiperbelt自体のendpoint情報 例: `localhost:12345`
  * 複数のkuiperbeltサーバを用いて冗長性を確保した構成にする場合にどの接続がどのサーバへ接続されたかを識別するために用います
  * 冗長化構成の場合では後述の`X-Kuiperbelt-Session`の値とセットでデータベースなどに保存することを推奨します

さらに、実装したアプリケーションサーバからkuiperbeltへレスポンスを返す際に、以下のヘッダを添付する必要があります。

* `X-Kuiperbelt-Session`
  * 接続確立以後に接続に対してメッセージを送信する際に用いる識別子です
  * 前述の設定ファイルで`session_header`というキーを設定することでヘッダの名前を変えることが出来ます
  * 同一のクライアントが再接続した場合には`X-Kuiperbelt-Session`を使いまわさずに新規に発行してください
    * クライアントからの接続が正しく切断されなかったケースで使いまわすと正しい接続に対してメッセージが送れないことがあります

この解説では手動で接続の際に上記のヘッダの情報を確認し、また接続のための識別子を送るため、nc(netcat)で待ち受けます。

```sh
$ ( echo "HTTP/1.0 200 Ok\nX-Kuiperbelt-Session: alice"; echo; echo "Hello Kuiperbelt" ) | nc -l 12346
```

以上のサーバで`alice`という名前で接続に対してメッセージを送ることが出来るようになります。

実際には`alice`のような名前ではなくUUIDのようにユニークに生成できる方法で`X-Kuiperbelt-Session`を設定してください。

#### 認証

`/connect`にコールバックが来る時点ではクライアントへの接続はWebSocketへUpgradeはされていません。
そこでこのリクエストに対して200以外のレスポンスを返すことによりWebSocketの接続を拒否することができます。
この機構を利用し認証を実装することが出来ます。

#### 参考実装

[_example/app.psgi](https://github.com/kuiperbelt/kuiperbelt/blob/master/_example/app.psgi)Perl/Plackでの参考実装があります。

#### 4. kuiperbeltの起動

作業ディレクトリ内で以下のコマンドを実行することによりkuiperbeltが起動します。

```sh
$ ekbo -config=config.yml
```

#### 5. WebSocketでの接続

ここでは[wscat](https://github.com/websockets/wscat)を用いて接続を試みてみます。インストールはリンク先のドキュメントを参照してください。

kuiperbeltは12346ポートで立っているので以下のコマンドで接続します。

```sh
$ wscat --connect http://localhost:12345/connect
```

接続に成功すると以下のようなメッセージが来ます。

```
$ wscat --connect http://localhost:12345/connect
connected (press CTRL+C to quit)
< Hello Kuiperbelt
```

前述のncで実装したサーバにおいて、接続時に`Hello Kuiperbelt`という文字列をbodyに含めて送信するようにしていました。このように接続時に返すメッセージを`/connect`エンドポイントのレスポンスに含めることが出来ます。

#### 6. メッセージの送信

確立された接続に対してHTTP APIでメッセージを送信することが出来ます。

```sh
$ curl -XPOST -H'X-Kuiperbelt-Session: alice' -d 'How are you doing?' http://localhost:12345/send
```

正常に送信できた場合には以下のようなJSON形式のレスポンスが返却されます。
```json
{"result":"OK"}
```

指定した`X-Kuiperbelt-Session`が存在しない場合には以下のようなエラーレスポンスが返却されます。
```json
{"errors":[{"error":"session is not found.","session":"bob"}],"result":"NG"}
```

同じメッセージを複数の接続に対して送信したい場合は、`X-Kuiperbelt-Session`ヘッダを複数個添付して送信します。

```sh
$ curl -XPOST -H'X-Kuiperbelt-Session: alice' -H'X-Kuiperbelt-Session: bob' -d 'How are you doing?' http://localhost:12345/send
```

もし複数指定した`X-Kuiperbelt-Session`のうち一部の識別子が存在しない場合には以下のような形でレスポンスが帰ってきます。

これはaliceとbobに対して送信したものの、bobに対応する接続が無かった際の例です。

```json
{"errors":[{"error":"session is not found.","session":"bob"}],"result":"OK"}
```

送信されたメッセージが存在するので`result`は`OK`になります。

すべての`X-Kuiperbelt-Session`が存在する場合にしか送信されないようにするためには、`StrictBroadcast`をconfig.ymlで有効にします。

```yaml
# config.yml
---
strict_broadcast: true
```

#### 7. 接続の切断

サーバサイドからの接続の切断は`/close`というAPIにリクエストすることで出来ます。

```sh
$ curl -XPOST -H'X-Kuiperbelt-Session: alice' -d 'Bye' http://localhost:12345/close
```

これも`/send`と同様にbodyにメッセージを含めることで切断前の最後のメッセージを送信することが出来ます。

## その他の機能

* 状態API `/stats`
  * エラー数やメッセージ数などの情報がJSON形式で得られます
  * 例: `{"connections":1,"total_connections":5,"total_messages":12,"connect_errors":1,"message_errors":0}`
* 切断コールバック
  * config.ymlの`callback.close`にエンドポイントを登録することでクライアントサイド要因での切断時に通知を受けることが出来ます
  * 切断された接続の識別子は`X-Kuiperbelt-Session`に記載されます

## 実装予定の機能

* cluster mode
* upstream proxy

## ライセンス

[The MIT License](https://github.com/kuiperbelt/kuiperbelt/blob/master/LICENCE)

Copyright (c) 2015 TANIWAKI Makoto / (c) 2015 [KAYAC Inc.](https://github.com/kayac)

## 著者

* [mackee](https://github.com/mackee)
* [shogo82148](https://github.com/shogo82148)
* [fujiwara](https://github.com/fujiwara)
