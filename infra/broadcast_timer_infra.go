package main

import (
	"fmt"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsapigatewayv2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsapigatewayv2integrations"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsdynamodb" // ← 【修正3】DynamoDB用のインポートを追加しました
	"github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

type TimerStackProps struct {
	awscdk.StackProps
	EnvName string
}

func NewBroadcastTimerStack(scope constructs.Construct, id string, props *TimerStackProps) awscdk.Stack {
	stack := awscdk.NewStack(scope, &id, &props.StackProps)

	// リソース名に環境名を付与
	funcName := fmt.Sprintf("WebSocketHandler-%s", props.EnvName)
	apiName := fmt.Sprintf("BroadcastTimerAPI-%s", props.EnvName)

	// Lambda関数の定義
	handler := awslambda.NewFunction(stack, jsii.String("WebSocketHandler"), &awslambda.FunctionProps{
		Runtime:      awslambda.Runtime_PROVIDED_AL2(),
		FunctionName: jsii.String(funcName),
		Handler:      jsii.String("bootstrap"),
		Code:         awslambda.Code_FromAsset(jsii.String("../backend/main.zip"), nil),
	})

	// IAM権限の付与
	handler.AddToRolePolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions:   jsii.Strings("execute-api:ManageConnections"),
		Resources: jsii.Strings("arn:aws:execute-api:*:*:*/*/*/*"),
	}))

	// WebSocket APIの作成
	webSocketApi := awsapigatewayv2.NewWebSocketApi(stack, jsii.String("BroadcastTimerAPI"), &awsapigatewayv2.WebSocketApiProps{
		ApiName: jsii.String(apiName),
	})

	// Lambda統合の設定
	integration := awsapigatewayv2integrations.NewWebSocketLambdaIntegration(jsii.String("HandlerIntegration"), handler, nil)

	// ルートの設定
	webSocketApi.AddRoute(jsii.String("$connect"), &awsapigatewayv2.WebSocketRouteOptions{Integration: integration})
	webSocketApi.AddRoute(jsii.String("$disconnect"), &awsapigatewayv2.WebSocketRouteOptions{Integration: integration})
	webSocketApi.AddRoute(jsii.String("$default"), &awsapigatewayv2.WebSocketRouteOptions{Integration: integration})

	// ステージの作成
	awsapigatewayv2.NewWebSocketStage(stack, jsii.String("Stage"), &awsapigatewayv2.WebSocketStageProps{
		WebSocketApi: webSocketApi,
		StageName:    jsii.String(props.EnvName),
		AutoDeploy:   jsii.Bool(true),
	})

	// URLを出力
	apiEndpoint := fmt.Sprintf("wss://%s.execute-api.%s.amazonaws.com/%s", *webSocketApi.ApiId(), *stack.Region(), props.EnvName)
	awscdk.NewCfnOutput(stack, jsii.String("WebSocketURL"), &awscdk.CfnOutputProps{
		Value: jsii.String(apiEndpoint),
	})

	// 【修正1】returnの前にDynamoDBの処理を移動しました
	// DynamoDBテーブルの作成
	connectionsTable := awsdynamodb.NewTable(stack, jsii.String("ConnectionsTable"), &awsdynamodb.TableProps{
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("connectionId"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		BillingMode:   awsdynamodb.BillingMode_PAY_PER_REQUEST,
		RemovalPolicy: awscdk.RemovalPolicy_DESTROY,
	})

	// 【修正2】権限と環境変数を渡す対象を "handler" に修正しました
	connectionsTable.GrantFullAccess(handler)
	handler.AddEnvironment(jsii.String("CONNECTIONS_TABLE"), connectionsTable.TableName(), nil)

	// スタックの構築がすべて終わってから返す
	return stack
}

func main() {
	app := awscdk.NewApp(nil)

	envContext := app.Node().TryGetContext(jsii.String("env"))
	envName := "dev"
	if envContext != nil {
		envName = envContext.(string)
	}

	stackName := fmt.Sprintf("BroadcastTimerStack-%s", envName)

	NewBroadcastTimerStack(app, stackName, &TimerStackProps{
		StackProps: awscdk.StackProps{},
		EnvName:    envName,
	})

	app.Synth(nil)
}