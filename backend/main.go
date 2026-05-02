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
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

// ActionMessage はフロントエンドから届くメッセージの構造体です
type ActionMessage struct {
	Action        string  `json:"action"`
	RoomID        string  `json:"room_id"`       // ルーム識別子
	Pin           string  `json:"pin"`           // 4桁パスワード
	State         string  `json:"state"`         // running, reset, etc.
	ReferenceUTC  int64   `json:"reference_utc"` // 同期用の基準時刻
	BaseFrames    int64   `json:"base_frames"`   // 通算フレーム
	FPS           float64 `json:"fps"`           // フレームレート
	IsDF          bool    `json:"is_df"`         // ドロップフレームフラグ
	StartTimecode string  `json:"start_timecode"` // 開始オフセット時刻
}

// RoomState はDynamoDB(RoomStatesTable)に保存する部屋の状態です
type RoomState struct {
	RoomID        string  `json:"room_id" dynamodbav:"room_id"`
	RoomPin       string  `json:"room_pin" dynamodbav:"room_pin"`
	State         string  `json:"state" dynamodbav:"state"`
	ReferenceUTC  int64   `json:"reference_utc" dynamodbav:"reference_utc"`
	FPS           float64 `json:"fps" dynamodbav:"fps"`
	IsDF          bool    `json:"is_df" dynamodbav:"is_df"`
	StartTimecode string  `json:"start_timecode" dynamodbav:"start_timecode"`
}

func handler(ctx context.Context, req events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
	// 環境変数からテーブル名を取得[cite: 1, 2]
	connTableName := os.Getenv("CONNECTIONS_TABLE")
	roomTableName := os.Getenv("ROOM_STATES_TABLE")
	
	sess := session.Must(session.NewSession())
	db := dynamodb.New(sess)
	connectionID := req.RequestContext.ConnectionID

	// 1. 接続・切断時の基本処理
	if req.RequestContext.EventType == "CONNECT" {
		_, err := db.PutItem(&dynamodb.PutItemInput{
			TableName: aws.String(connTableName),
			Item: map[string]*dynamodb.AttributeValue{
				"connectionId": {S: aws.String(connectionID)},
			},
		})
		return events.APIGatewayProxyResponse{StatusCode: 200}, err
	}

	if req.RequestContext.EventType == "DISCONNECT" {
		_, _ = db.DeleteItem(&dynamodb.DeleteItemInput{
			TableName: aws.String(connTableName),
			Key: map[string]*dynamodb.AttributeValue{
				"connectionId": {S: aws.String(connectionID)},
			},
		})
		return events.APIGatewayProxyResponse{StatusCode: 200}, nil
	}

	// 2. メッセージ受信時の処理[cite: 1]
	var msg ActionMessage
	if err := json.Unmarshal([]byte(req.Body), &msg); err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 400}, nil
	}

	// API Gateway Management API クライアントの初期化[cite: 1]
	endpoint := fmt.Sprintf("https://%s/%s", req.RequestContext.DomainName, req.RequestContext.Stage)
	apigw := apigatewaymanagementapi.New(sess, aws.NewConfig().WithEndpoint(endpoint))

	// アクションに応じた分岐処理
	switch msg.Action {
	case "join":
		return handleJoin(db, apigw, connTableName, roomTableName, connectionID, msg)
	case "leave":
		return handleLeave(db, connTableName, connectionID)
	default:
		// running, reset, sync などの同期・操作処理
		return handleSync(db, apigw, connTableName, roomTableName, msg)
	}
}

// handleJoin は入室とパスワード照合を処理します[cite: 1]
func handleJoin(db *dynamodb.DynamoDB, apigw *apigatewaymanagementapi.ApiGatewayManagementApi, connTable, roomTable, connID string, msg ActionMessage) (events.APIGatewayProxyResponse, error) {
	// 部屋情報を取得
	result, err := db.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(roomTable),
		Key: map[string]*dynamodb.AttributeValue{
			"room_id": {S: aws.String(msg.RoomID)},
		},
	})
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 500}, err
	}

	var room RoomState
	if result.Item == nil {
		// 部屋が存在しない場合は新規作成（送られた設定を初期値とする）
		room = RoomState{
			RoomID:        msg.RoomID,
			RoomPin:       msg.Pin,
			State:         "reset",
			FPS:           msg.FPS,
			IsDF:          msg.IsDF,
			StartTimecode: msg.StartTimecode,
		}
		av, _ := dynamodbattribute.MarshalMap(room)
		db.PutItem(&dynamodb.PutItemInput{
			TableName: aws.String(roomTable),
			Item:      av,
		})
	} else {
		// 部屋が存在する場合はパスワード照合
		dynamodbattribute.UnmarshalMap(result.Item, &room)
		if msg.Pin != room.RoomPin {
			// パスワード不一致エラー
			return events.APIGatewayProxyResponse{StatusCode: 403}, nil
		}
	}

	// 照合成功：ConnectionsTableを更新して room_id を紐付ける[cite: 2]
	db.PutItem(&dynamodb.PutItemInput{
		TableName: aws.String(connTable),
		Item: map[string]*dynamodb.AttributeValue{
			"connectionId": {S: aws.String(connID)},
			"room_id":      {S: aws.String(msg.RoomID)},
		},
	})

	// 入室した本人に現在の部屋の状態を同期データとして送る[cite: 1]
	resData, _ := json.Marshal(room)
	apigw.PostToConnection(&apigatewaymanagementapi.PostToConnectionInput{
		ConnectionId: aws.String(connID),
		Data:         resData,
	})

	return events.APIGatewayProxyResponse{StatusCode: 200}, nil
}

// handleSync は操作コマンドを同じ部屋の全員に一斉送信します[cite: 1]
func handleSync(db *dynamodb.DynamoDB, apigw *apigatewaymanagementapi.ApiGatewayManagementApi, connTable, roomTable string, msg ActionMessage) (events.APIGatewayProxyResponse, error) {
	// 1. 部屋の最新状態を RoomStatesTable に保存[cite: 2]
	// 操作（Start/Reset）が走るたびに、後から来た人のために状態を更新しておく
	roomUpdate := RoomState{
		RoomID:        msg.RoomID,
		RoomPin:       msg.Pin,
		State:         msg.State,
		ReferenceUTC:  msg.ReferenceUTC,
		FPS:           msg.FPS,
		IsDF:          msg.IsDF,
		StartTimecode: msg.StartTimecode,
	}
	av, _ := dynamodbattribute.MarshalMap(roomUpdate)
	db.PutItem(&dynamodb.PutItemInput{
		TableName: aws.String(roomTable),
		Item:      av,
	})

	// 2. 同じルームに属する接続IDのみを取得 (FilterExpressionを使用)[cite: 1, 2]
	scanOut, err := db.Scan(&dynamodb.ScanInput{
		TableName:        aws.String(connTable),
		FilterExpression: aws.String("room_id = :r"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":r": {S: aws.String(msg.RoomID)},
		},
	})
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 500}, err
	}

	// 3. 同じ部屋のメンバー全員に送信[cite: 1]
	resData, _ := json.Marshal(msg)
	for _, item := range scanOut.Items {
		targetID := *item["connectionId"].S
		_, err := apigw.PostToConnection(&apigatewaymanagementapi.PostToConnectionInput{
			ConnectionId: aws.String(targetID),
			Data:         resData,
		})
		
		// 接続が切れている場合は名簿から削除[cite: 1]
		if err != nil {
			db.DeleteItem(&dynamodb.DeleteItemInput{
				TableName: aws.String(connTable),
				Key: map[string]*dynamodb.AttributeValue{
					"connectionId": {S: aws.String(targetID)},
				},
			})
		}
	}

	return events.APIGatewayProxyResponse{StatusCode: 200}, nil
}

// handleLeave は部屋からの退出処理を行います[cite: 1]
func handleLeave(db *dynamodb.DynamoDB, connTable, connID string) (events.APIGatewayProxyResponse, error) {
	// 接続IDは残したまま、room_id の紐付けだけを消去する
	_, err := db.PutItem(&dynamodb.PutItemInput{
		TableName: aws.String(connTable),
		Item: map[string]*dynamodb.AttributeValue{
			"connectionId": {S: aws.String(connID)},
		},
	})
	return events.APIGatewayProxyResponse{StatusCode: 200}, err
}

func main() {
	lambda.Start(handler)
}