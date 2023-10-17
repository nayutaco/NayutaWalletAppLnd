# CloseChecker

## Abstract

モバイルアプリで Revoked Commitment Transaction によるクローズを検知したい。  
Watchtower は個人レベルで運用することはできるが、会社として運用するには難しい。  
LND が起動していない状態でチャネルクローズを検知できるように Electrum Server を使ってブロックチェーンを監視する。

## Electrum Server

### Using method

* ServerPeers
* GetTransaction(verbose)
* GetHistory

GetHistory はアドレスから送金したトランザクションを取得するメソッドだが、Electrum Server API では直接アドレスを扱わず scriptHash を SHA256 した値を使っている。

### Find and connect Server

`closechecker.getElectrumClient()`

1. ハードコーディングしたElectrum Serverからランダムに1つ選ぶ
2. 接続する
  a. 接続に失敗したら 1 に戻る(リトライは1〜3で30回まで)
3. Server一覧を取得
  a. エラーが発生したら 1 に戻る(リトライは1〜3で30回まで)
4. ハードコーディングしたサーバと取得したサーバを合わせてランダムに1つ選ぶ
  a. 接続に失敗したら 4 に戻る(リトライは4〜5で30回まで)
5. `GetTransaction()`を実施
  a. エラーが発生したら 4 に戻る(リトライは4〜5で30回まで)
6. 接続成功

* Memo
  * `GetTransaction()`をサポートしていないサーバがあったため確認している
  * onionノードは対応していないためリストから除外している

### Hard coded Electrum Server list

1. [Electrum](https://github.com/spesmilo/electrum/tree/master/electrum) からハードコーディングされた Electrum Server 一覧をダウンロードする。
  * https://raw.githubusercontent.com/spesmilo/electrum/018a83078c93cfb5c14507d1bc62bd5baa2af825/electrum/servers.json
  * https://raw.githubusercontent.com/spesmilo/electrum/2af59e32b2c03961aa57d6d6872f6099ed8890f5/electrum/servers_testnet.json
2. `conv.sh` を修正して実行する。
3. 出力されたファイルはまだ golang で使えるようになっていないので、手動で置き換える。
4. mainnet, testnet でそれぞれ配列を作り electrum-servers.go ファイルを更新する。

## Closed check

LNDのバックエンドとしてNeutrinoを使っているため、ChannelPointがspentになっただけでは検知できない。チャネルを閉じたトランザクションがconfirmされる必要がある。  

1. DBの先頭から順に取得する(key=チェックするchannelPoint, value=sighash)
  a. sighashが空欄の場合は計算してDBを更新する
2. `GetHistory(sighash)`で送金されたTXID一覧を取得
3. TXID の Height が存在する場合
  a. `GetTransaction(TXID)`
  b. トランザクションのVINが条件を満たすか確認
    * VINの数は1つ
    * VINのTXIDがchannelPointのTXIDと一致する
  c. 一致する場合、そのチャネルは閉じられたと判断する
