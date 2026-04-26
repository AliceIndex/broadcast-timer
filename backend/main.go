package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/apigatewaymanagementapi"
)

type ActionMessage struct {
	Action       string  `json:"action"`
	State        string  `json:"state"` // "running", "stopped", "reset"
	ReferenceUTC int64   `json:"reference_utc"`
	BaseFrames   int64   `json:"base_frames"`
	FPS          float64 `json:"fps"`
	IsDF         bool    `json:"is_df"`
}

func handler(ctx context.Context, req events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
	// WebSocket配信用エンドポイントの構築
	endpoint := fmt.Sprintf("https://%s/%s", req.RequestContext.DomainName, req.RequestContext.Stage)
	sess := session.Must(session.NewSession())
	svc := apigatewaymanagementapi.New(sess, aws.NewConfig().WithEndpoint(endpoint))

	// [注] 本番環境ではここで DynamoDB を用いて ConnectionID の保存/削除/取得を行います
	if req.RequestContext.EventType == "CONNECT" || req.RequestContext.EventType == "DISCONNECT" {
		return events.APIGatewayProxyResponse{StatusCode: 200}, nil
	}

	var msg ActionMessage
	if err := json.Unmarshal([]byte(req.Body), &msg); err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 400}, nil
	}

	// クライアント同期用にアクション名を上書き
	msg.Action = "sync"
	responseData, _ := json.Marshal(msg)

	// [注] 本番環境では DynamoDB から取得した全 ConnectionID に対してループで送信します
	// ここでは送信元にのみエコーバックする処理を記述しています
	_, err := svc.PostToConnection(&apigatewaymanagementapi.PostToConnectionInput{
		ConnectionId: aws.String(req.RequestContext.ConnectionID),
		Data:         responseData,
	})
	if err != nil {
		fmt.Println("Broadcast Error:", err)
	}

	return events.APIGatewayProxyResponse{StatusCode: 200}, nil
}

func main() {
	lambda.Start(handler)
}