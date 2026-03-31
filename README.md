# concrnt 2.0 実験場

## だいじなこと

- 小さな仕様を組み合わせて構成する
  - 仕様は[CIPs (Concrnt Improvement Proposals)](https://github.com/concrnt/CIPs-translated)へ
- 再実装しやすく・シンプルに
  - webサーバーがstatic hostでも成り立つように
    - アーカイブサーバーもそうだし
    - 書き換え時だけ動的に動いてs3とかに書き込むようなやつでもいいね
      - lambdaとかで動けるとすごい

## リリースまでのTODO
- [x] ackの実装
  - [ ] 対象ユーザーが外部ドメインユーザーだった場合に転送する
- [x] subkeyの実装・検証
- [x] proxy実装
  - [x] webクライアント向け
- [x] policyの評価
- [ ] 他サーバーとのrealtime通信
  - 横に並べても問題が起きないようにしたい
    - リーダーインスタンスを決めてそこから受信するとか
      - k8sだったらleaseが使える
- [ ] timeline読み込みをキャッシュに載せる
- [ ] 通知周り
  - webpushはともかくiOSやAndroidのpushはどうする？
  - -> webpushだけにして、iosやandroidへはリレーサーバーを利用するようにする
    - https://github.com/mastodon/webpush-apn-relay
- [ ] 引っ越し機能の実装
  - [ ] 自分のログの出力API
  - [ ] 引っ越し先のサーバーへのインポートAPI
- [ ] alias機能の実装

## リリース後
- [ ] NATSとredis pubsubを切り替えられるように
- [ ] valkey対応？
- [ ] batchエンドポイント

## まだ考え中なこと
- マイグレーションとか
  - そのままリソースをimportしてしまう手法
    - good: ユーザーの対応がほぼ不要
    - bad: 再度引っ越しとのときに引き継がれない
  - v0-\>v1と同様に全部export-\>importする手法
    - good: 確実に引き継げる
    - bad: ユーザーの対応が必要

