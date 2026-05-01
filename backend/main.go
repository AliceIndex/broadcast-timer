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
	// ★ ここが目印になります！
	fmt.Println("=== LAMBDA INVOKED ===")
	fmt.Printf("種類: %s | ID: %s\n", req.RequestContext.EventType, req.RequestContext.ConnectionID)

	tableName := os.Getenv("CONNECTIONS_TABLE")
	sess := session.Must(session.NewSession())
	db := dynamodb.New(sess)

	// 1. 接続時の処理
	if req.RequestContext.EventType == "CONNECT" {
		fmt.Println("[CONNECT] データベースにIDを登録します...")
		_, err := db.PutItem(&dynamodb.PutItemInput{
			TableName: aws.String(tableName),
			Item: map[string]*dynamodb.AttributeValue{
				"connectionId": {S: aws.String(req.RequestContext.ConnectionID)},
			},
		})
		if err != nil {
			fmt.Println("[CONNECT ERROR] 登録失敗:", err)
			return events.APIGatewayProxyResponse{StatusCode: 500}, err
		}
		fmt.Println("[CONNECT SUCCESS] 登録完了！")
		return events.APIGatewayProxyResponse{StatusCode: 200}, nil
	}

	// 2. 切断時の処理
	if req.RequestContext.EventType == "DISCONNECT" {
		fmt.Println("[DISCONNECT] データベースからIDを削除します...")
		_, _ = db.DeleteItem(&dynamodb.DeleteItemInput{
			TableName: aws.String(tableName),
			Key: map[string]*dynamodb.AttributeValue{
				"connectionId": {S: aws.String(req.RequestContext.ConnectionID)},
			},
		})
		return events.APIGatewayProxyResponse{StatusCode: 200}, nil
	}

	// 3. メッセージ受信時 (STARTボタンを押した時の処理)
	fmt.Println("[MESSAGE] 受信データ:", req.Body)
	var msg ActionMessage
	if err := json.Unmarshal([]byte(req.Body), &msg); err != nil {
		fmt.Println("[MESSAGE ERROR] JSON変換失敗:", err)
		return events.APIGatewayProxyResponse{StatusCode: 400}, nil
	}

	msg.Action = "sync"
	responseData, _ := json.Marshal(msg)

	endpoint := fmt.Sprintf("https://%s/%s", req.RequestContext.DomainName, req.RequestContext.Stage)
	apigw := apigatewaymanagementapi.New(sess, aws.NewConfig().WithEndpoint(endpoint))

	fmt.Println("[MESSAGE] データベースから全員のリストを取得します...")
	scanOut, err := db.Scan(&dynamodb.ScanInput{
		TableName: aws.String(tableName),
	})
	if err != nil {
		fmt.Println("[MESSAGE ERROR] リスト取得失敗:", err)
		return events.APIGatewayProxyResponse{StatusCode: 500}, err
	}

	successCount := 0
	for _, item := range scanOut.Items {
		targetID := *item["connectionId"].S
		fmt.Println("[MESSAGE] 送信先:", targetID)
		
		_, err := apigw.PostToConnection(&apigatewaymanagementapi.PostToConnectionInput{
			ConnectionId: aws.String(targetID),
			Data:         responseData,
		})
		
		if err != nil {
			fmt.Println("[MESSAGE ERROR] 送信失敗。名簿から削除します 理由:", err)
			db.DeleteItem(&dynamodb.DeleteItemInput{
				TableName: aws.String(tableName),
				Key: map[string]*dynamodb.AttributeValue{
					"connectionId": {S: aws.String(targetID)},
				},
			})
		} else {
			successCount++
		}
	}
	
	fmt.Printf("[MESSAGE SUCCESS] 全 %d 台への一斉送信完了！\n", successCount)
	return events.APIGatewayProxyResponse{StatusCode: 200}, nil
}

func main() {
	lambda.Start(handler)
}