package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/apigatewaymanagementapi"
	"github.com/aws/aws-sdk-go/service/dynamodb"
)

type ActionMessage struct {
	Action       string  `json:"action"`
	State        string  `json:"state"`
	ReferenceUTC int64   `json:"reference_utc"`
	BaseFrames   int64   `json:"base_frames"`
	FPS          float64 `json:"fps"`
	IsDF         bool    `json:"is_df"`
}

func handler(ctx context.Context, req events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
	// 環境変数からDynamoDBのテーブル名を取得
	tableName := os.Getenv("CONNECTIONS_TABLE")
	sess := session.Must(session.NewSession())
	db := dynamodb.New(sess)

	connectionID := req.RequestContext.ConnectionID
	eventType := req.RequestContext.EventType

	// 1. 新規接続時：名簿（DynamoDB）にIDを記録
	if eventType == "CONNECT" {
		_, err := db.PutItem(&dynamodb.PutItemInput{
			TableName: aws.String(tableName),
			Item: map[string]*dynamodb.AttributeValue{
				"connectionId": {S: aws.String(connectionID)},
			},
		})
		if err != nil {
			fmt.Println("Connect DB Error:", err)
			return events.APIGatewayProxyResponse{StatusCode: 500}, err
		}
		return events.APIGatewayProxyResponse{StatusCode: 200}, nil
	}

	// 2. 切断時：名簿からIDを削除
	if eventType == "DISCONNECT" {
		_, _ = db.DeleteItem(&dynamodb.DeleteItemInput{
			TableName: aws.String(tableName),
			Key: map[string]*dynamodb.AttributeValue{
				"connectionId": {S: aws.String(connectionID)},
			},
		})
		return events.APIGatewayProxyResponse{StatusCode: 200}, nil
	}

	// 3. メッセージ受信時：全員にブロードキャスト
	var msg ActionMessage
	if err := json.Unmarshal([]byte(req.Body), &msg); err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 400}, nil
	}

	msg.Action = "sync"
	responseData, _ := json.Marshal(msg)

	endpoint := fmt.Sprintf("https://%s/%s", req.RequestContext.DomainName, req.RequestContext.Stage)
	apigw := apigatewaymanagementapi.New(sess, aws.NewConfig().WithEndpoint(endpoint))

	// 名簿（DynamoDB）から現在繋がっている全員のIDを取得
	scanOut, err := db.Scan(&dynamodb.ScanInput{
		TableName: aws.String(tableName),
	})
	if err != nil {
		fmt.Println("DB Scan Error:", err)
		return events.APIGatewayProxyResponse{StatusCode: 500}, err
	}

	// 全員に向けてループ送信
	for _, item := range scanOut.Items {
		targetID := *item["connectionId"].S
		_, err := apigw.PostToConnection(&apigatewaymanagementapi.PostToConnectionInput{
			ConnectionId: aws.String(targetID),
			Data:         responseData,
		})
		// 送信失敗（既にブラウザを閉じている等）の場合は名簿から削除
		if err != nil {
			fmt.Printf("Stale connection removed: %s\n", targetID)
			db.DeleteItem(&dynamodb.DeleteItemInput{
				TableName: aws.String(tableName),
				Key: map[string]*dynamodb.AttributeValue{
					"connectionId": {S: aws.String(targetID)},
				},
			})
		}
	}

	return events.APIGatewayProxyResponse{StatusCode: 200}, nil
}

func main() {
	lambda.Start(handler)
}