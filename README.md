# Broadcast Timer

リアルタイムで同期する放送用タイマーシステムです。
AWSのサーバーレスアーキテクチャを活用し、複数デバイス間で低遅延のタイムコード同期を実現します。

## 🚀 機能の紹介 (Features)

- **リアルタイム同期**
  WebSocketを利用し、インターネット経由で複数のデバイス（PC、スマートフォンなど）のタイマー表示を瞬時に同期します。
- **役割に合わせた独立UI**
  - **コントローラー (`index.html`)**: タイマーの操作（Start / Stop / Reset）を行う管理用画面です。
  - **モニター (`monitor.html`)**: 操作系を持たず、現在のタイムコードを表示することに特化した演者・スタッフ用画面です。
- **サーバーレスアーキテクチャ**
  AWS API Gateway と Lambda を組み合わせることで、常時起動するサーバー（EC2など）を排除。リクエスト発生時のみ稼働するため、個人開発において実質ゼロコストでの運用が可能です。
- **完全自動化された CI/CD パイプライン**
  GitHub Actionsを用いて、フロントエンドとバックエンドのデプロイを完全に自動化・分離しています。
  - **Frontend**: コードの変更を検知し、GitHub Pagesへ自動ホスティング。
  - **Backend**: OIDC認証を用いたセキュアな通信で、AWS CDKによるインフラストラクチャの自動更新（差分デプロイ）を実行。

## 🛠️ 技術スタック (Tech Stack)

- **Frontend**: HTML5, Vanilla JavaScript, CSS
- **Backend**: Go (AWS Lambda)
- **Infrastructure**: AWS CDK for Go, Amazon API Gateway (WebSocket API)
- **CI/CD**: GitHub Actions, GitHub Pages

## ©️ Copyright

Copyright &copy; 2026 AliceIndex. All rights reserved.