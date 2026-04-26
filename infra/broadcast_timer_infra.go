package main

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsapigatewayv2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsapigatewayv2integrations"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

func NewBroadcastTimerStack(scope constructs.Construct, id string, props *awscdk.StackProps) awscdk.Stack {
	stack := awscdk.NewStack(scope, &id, props)

	// Lambda関数の定義 (事前にコンパイルした main.zip を指定)
	handler := awslambda.NewFunction(stack, jsii.String("WebSocketHandler"), &awslambda.FunctionProps{
		Runtime: awslambda.Runtime_PROVIDED_AL2(),
		Handler: jsii.String("bootstrap"),
		Code:    awslambda.Code_FromAsset(jsii.String("../backend/main.zip"), nil),
	})

	// LambdaにAPI Gateway接続を管理する権限を付与
	handler.AddToRolePolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions:   jsii.Strings([]string{"execute-api:ManageConnections"}),
		Resources: jsii.Strings([]string{"arn:aws:execute-api:*:*:*/*/*/*"}),
	}))

	// WebSocket API の作成
	webSocketApi := awsapigatewayv2.NewWebSocketApi(stack, jsii.String("BroadcastTimerAPI"), &awsapigatewayv2.WebSocketApiProps{
		ApiName: jsii.String("broadcast-timer-websocket"),
	})

	// Lambda統合の設定
	integration := awsapigatewayv2integrations.NewWebSocketLambdaIntegration(jsii.String("HandlerIntegration"), handler, nil)

	// $connect, $disconnect, $default ルートの設定
	webSocketApi.AddRoute(jsii.String("$connect"), &awsapigatewayv2.WebSocketRouteOptions{ Integration: integration })
	webSocketApi.AddRoute(jsii.String("$disconnect"), &awsapigatewayv2.WebSocketRouteOptions{ Integration: integration })
	webSocketApi.AddRoute(jsii.String("$default"), &awsapigatewayv2.WebSocketRouteOptions{ Integration: integration })

	// Prodステージでのデプロイ
	awsapigatewayv2.NewWebSocketStage(stack, jsii.String("ProdStage"), &awsapigatewayv2.WebSocketStageProps{
		WebSocketApi: webSocketApi,
		StageName:    jsii.String("prod"),
		AutoDeploy:   jsii.Bool(true),
	})

	return stack
}

func main() {
	app := awscdk.NewApp(nil)
	NewBroadcastTimerStack(app, "BroadcastTimerStack", &awscdk.StackProps{})
	app.Synth(nil)
}