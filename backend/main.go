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

type ActionMessage struct {
	Action        string  `json:"action"`
	RoomID        string  `json:"room_id"`
	Pin           string  `json:"pin"`
	State         string  `json:"state"`
	ReferenceUTC  int64   `json:"reference_utc"`
	BaseFrames    int64   `json:"base_frames"`
	FPS           float64 `json:"fps"`
	IsDF          bool    `json:"is_df"`
}

type RoomState struct {
	RoomID       string  `json:"room_id" dynamodbav:"room_id"`
	RoomPin      string  `json:"room_pin" dynamodbav:"room_pin"`
	State        string  `json:"state" dynamodbav:"state"`
	ReferenceUTC int64   `json:"reference_utc" dynamodbav:"reference_utc"`
	BaseFrames   int64   `json:"base_frames" dynamodbav:"base_frames"` // 数値で管理
	FPS          float64 `json:"fps" dynamodbav:"fps"`
	IsDF         bool    `json:"is_df" dynamodbav:"is_df"`
}

func handler(ctx context.Context, req events.APIGatewayWebsocketProxyRequest) (events.APIGatewayProxyResponse, error) {
	fmt.Printf("--- [LOG] Lambda Invoked (ConnectionID: %s, EventType: %s) ---\n", req.RequestContext.ConnectionID, req.RequestContext.EventType)

	connTableName := os.Getenv("CONNECTIONS_TABLE")
	roomTableName := os.Getenv("ROOM_STATES_TABLE")
	
	sess := session.Must(session.NewSession())
	db := dynamodb.New(sess)
	connectionID := req.RequestContext.ConnectionID

	if req.RequestContext.EventType == "CONNECT" {
		fmt.Println("[CONNECT] Registering connection ID...")
		_, err := db.PutItem(&dynamodb.PutItemInput{
			TableName: aws.String(connTableName),
			Item: map[string]*dynamodb.AttributeValue{
				"connectionId": {S: aws.String(connectionID)},
			},
		})
		return events.APIGatewayProxyResponse{StatusCode: 200}, err
	}

	if req.RequestContext.EventType == "DISCONNECT" {
		fmt.Println("[DISCONNECT] Removing connection ID...")
		_, _ = db.DeleteItem(&dynamodb.DeleteItemInput{
			TableName: aws.String(connTableName),
			Key: map[string]*dynamodb.AttributeValue{
				"connectionId": {S: aws.String(connectionID)},
			},
		})
		return events.APIGatewayProxyResponse{StatusCode: 200}, nil
	}

	// ★ 400エラー特定のための最重要ログ
	fmt.Printf("[DEBUG] Raw Request Body: %s\n", req.Body)

	var msg ActionMessage
	if err := json.Unmarshal([]byte(req.Body), &msg); err != nil {
		fmt.Printf("[ERROR] JSON Unmarshal failed: %v\n", err) // ここで型の不一致がわかります
		return events.APIGatewayProxyResponse{StatusCode: 400}, nil
	}
	fmt.Printf("[DEBUG] Action: %s, RoomID: %s\n", msg.Action, msg.RoomID)

	endpoint := fmt.Sprintf("https://%s/%s", req.RequestContext.DomainName, req.RequestContext.Stage)
	apigw := apigatewaymanagementapi.New(sess, aws.NewConfig().WithEndpoint(endpoint))

	switch msg.Action {
	case "join":
		fmt.Println("[ACTION] Handling 'join'...")
		return handleJoin(db, apigw, connTableName, roomTableName, connectionID, msg)
	case "leave":
		fmt.Println("[ACTION] Handling 'leave'...")
		return handleLeave(db, connTableName, connectionID)
	default:
		fmt.Printf("[ACTION] Handling Timer Operation: %s\n", msg.Action)
		return handleSync(db, apigw, connTableName, roomTableName, msg)
	}
}

func handleJoin(db *dynamodb.DynamoDB, apigw *apigatewaymanagementapi.ApiGatewayManagementApi, connTable, roomTable, connID string, msg ActionMessage) (events.APIGatewayProxyResponse, error) {
	fmt.Printf("[JOIN] Checking room: %s\n", msg.RoomID)
	
	result, err := db.GetItem(&dynamodb.GetItemInput{
		TableName: aws.String(roomTable),
		Key: map[string]*dynamodb.AttributeValue{
			"room_id": {S: aws.String(msg.RoomID)},
		},
	})
	if err != nil {
		fmt.Printf("[JOIN ERROR] GetItem failed: %v\n", err)
		return events.APIGatewayProxyResponse{StatusCode: 500}, err
	}

	var room RoomState
	if result.Item == nil {
		fmt.Printf("[JOIN] Room %s not found. Creating new room...\n", msg.RoomID)
		room = RoomState{
			RoomID:        msg.RoomID,
			RoomPin:       msg.Pin,
			State:         "reset",
			FPS:           msg.FPS,
			IsDF:          msg.IsDF,
			BaseFrames:    msg.BaseFrames,
		}
		av, _ := dynamodbattribute.MarshalMap(room)
		db.PutItem(&dynamodb.PutItemInput{
			TableName: aws.String(roomTable),
			Item:      av,
		})
	} else {
		fmt.Printf("[JOIN] Room %s found. Verifying PIN...\n", msg.RoomID)
		dynamodbattribute.UnmarshalMap(result.Item, &room)
		if msg.Pin != room.RoomPin {
			fmt.Printf("[JOIN FAILED] PIN mismatch for room %s (Expected: %s, Received: %s)\n", msg.RoomID, room.RoomPin, msg.Pin)
			return events.APIGatewayProxyResponse{StatusCode: 403}, nil
		}
		fmt.Println("[JOIN SUCCESS] PIN verified.")
	}

	fmt.Printf("[JOIN] Binding connection %s to room %s\n", connID, msg.RoomID)
	db.PutItem(&dynamodb.PutItemInput{
		TableName: aws.String(connTable),
		Item: map[string]*dynamodb.AttributeValue{
			"connectionId": {S: aws.String(connID)},
			"room_id":      {S: aws.String(msg.RoomID)},
		},
	})

	syncMsg := ActionMessage{
		Action:       "sync",
		RoomID:       room.RoomID,
		State:        room.State,
		ReferenceUTC: room.ReferenceUTC,
		BaseFrames:   room.BaseFrames,
		FPS:          room.FPS,
		IsDF:         room.IsDF,
	}
	
	resData, _ := json.Marshal(syncMsg)
	apigw.PostToConnection(&apigatewaymanagementapi.PostToConnectionInput{
		ConnectionId: aws.String(connID),
		Data:         resData,
	})

	return events.APIGatewayProxyResponse{StatusCode: 200}, nil
}

func handleSync(db *dynamodb.DynamoDB, apigw *apigatewaymanagementapi.ApiGatewayManagementApi, connTable, roomTable string, msg ActionMessage) (events.APIGatewayProxyResponse, error) {
	fmt.Printf("[SYNC] Updating room %s state to: %s (Ref: %d)\n", msg.RoomID, msg.State, msg.ReferenceUTC)
	
	// DBへの保存
	roomUpdate := RoomState{
		RoomID:       msg.RoomID,
		RoomPin:      msg.Pin,
		State:        msg.State,
		ReferenceUTC: msg.ReferenceUTC,
		BaseFrames:   msg.BaseFrames,
		FPS:          msg.FPS,
		IsDF:         msg.IsDF,
	}
	av, _ := dynamodbattribute.MarshalMap(roomUpdate)
	db.PutItem(&dynamodb.PutItemInput{TableName: aws.String(roomTable), Item: av})

	// 同じルームのメンバーを取得
	scanOut, _ := db.Scan(&dynamodb.ScanInput{
		TableName:        aws.String(connTable),
		FilterExpression: aws.String("room_id = :r"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{
			":r": {S: aws.String(msg.RoomID)},
		},
	})

	// ★ クライアントに送り返す前のアクション名を「sync」に強制上書き
	msg.Action = "sync"
	resData, _ := json.Marshal(msg)

	success := 0
	for _, item := range scanOut.Items {
		targetID := *item["connectionId"].S
		_, err := apigw.PostToConnection(&apigatewaymanagementapi.PostToConnectionInput{
			ConnectionId: aws.String(targetID),
			Data:         resData,
		})
		
		if err != nil {
			db.DeleteItem(&dynamodb.DeleteItemInput{
				TableName: aws.String(connTable),
				Key: map[string]*dynamodb.AttributeValue{"connectionId": {S: aws.String(targetID)}},
			})
		} else {
			success++
		}
	}
	fmt.Printf("[SYNC] Broadcast complete. Success: %d/%d\n", success, len(scanOut.Items))

	return events.APIGatewayProxyResponse{StatusCode: 200}, nil
}

func handleLeave(db *dynamodb.DynamoDB, connTable, connID string) (events.APIGatewayProxyResponse, error) {
	fmt.Printf("[LEAVE] Removing room binding for connection: %s\n", connID)
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