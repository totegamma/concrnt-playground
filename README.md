# concrnt 2.0 実験場

## だいじなこと

- 小さな仕様を組み合わせて構成する
  - 仕様は[CIPs (Concrnt Improvement Proposals)](https://github.com/concrnt/CIPs-translated)へ
- 再実装しやすく・シンプルに
  - webサーバーがstatic hostでも成り立つように
    - アーカイブサーバーもそうだし
    - 書き換え時だけ動的に動いてs3とかに書き込むようなやつでもいいね
      - lambdaとかで動けるとすごい

## TODO
- リソースの検証
- subkeyの検証
- deleteとか
- association未テスト
- policyの評価
  - そもそもpolicy自体の仮実装はしたものの全くテストされてない
- 他サーバーとのrealtime通信
  - 横に並べても問題が起きないようにしたい
    - リーダーインスタンスを決めてそこから受信するとか
      - k8sだったらleaseが使える
- NATSとredis pubsubを切り替えられるように

## まだ考え中なこと
- マイグレーションとか
  - そのままリソースをimportしてしまう手法
    - good: ユーザーの対応がほぼ不要
    - bad: 再度引っ越しとのときに引き継がれない
  - v0-\>v1と同様に全部export-\>importする手法
    - good: 確実に引き継げる
    - bad: ユーザーの対応が必要


