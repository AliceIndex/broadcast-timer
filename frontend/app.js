// デプロイ時に取得した本番環境のURL
const API_URL = "wss://h53oec3a75.execute-api.ap-northeast-1.amazonaws.com/dev";

let ws;
let isConnected = false;

// --------------------------------------------------
// 1. UI要素の取得
// --------------------------------------------------
const timecodeDisplay = document.getElementById('timecode-display');
const startBtn = document.getElementById('btn-start');
const stopBtn = document.getElementById('btn-stop');
const resetBtn = document.getElementById('btn-reset');
const statusIndicator = document.getElementById('status-indicator');

// 内部状態の保持
let currentState = {
    status: 'stopped', // 'running', 'stopped'
    timecode: '00:00:00:00'
};

// --------------------------------------------------
// 2. WebSocketの初期化と接続管理
// --------------------------------------------------
function initWebSocket() {
    ws = new WebSocket(API_URL);

    ws.onopen = () => {
        isConnected = true;
        updateStatusUI('Connected', 'var(--color-success, #28a745)');
        console.log('WebSocket Connected');
    };

    ws.onmessage = (event) => {
        try {
            const data = JSON.parse(event.data);
            handleServerMessage(data);
        } catch (error) {
            console.error('JSON Parse Error:', error);
        }
    };

    ws.onclose = () => {
        isConnected = false;
        updateStatusUI('Disconnected', 'var(--color-danger, #dc3545)');
        console.log('WebSocket Disconnected. Reconnecting in 3 seconds...');
        // ネットワーク切断時やサーバー再起動時は3秒後に自動再接続
        setTimeout(initWebSocket, 3000);
    };

    ws.onerror = (error) => {
        console.error('WebSocket Error:', error);
    };
}

// --------------------------------------------------
// 3. メッセージ送受信のハンドリング
// --------------------------------------------------
/**
 * サーバーから受信した同期データの処理
 */
function handleServerMessage(data) {
    // Go言語側（main.go）から送られてくるJSON構造に合わせてマッピングします
    // 例: { "action": "sync", "status": "running", "timecode": "00:01:23:14" }
    if (data.action === 'sync' || data.timecode) {
        currentState.status = data.status || currentState.status;
        currentState.timecode = data.timecode || currentState.timecode;

        // 画面のタイムコード表示を即座に更新
        if (timecodeDisplay) {
            timecodeDisplay.textContent = currentState.timecode;
        }
    }
}

/**
 * サーバーへ操作コマンドを送信
 */
function sendCommand(actionName) {
    if (!isConnected || ws.readyState !== WebSocket.OPEN) {
        console.warn('WebSocket is not connected.');
        return;
    }

    const payload = {
        action: actionName,
        timestamp: Date.now()
    };
    ws.send(JSON.stringify(payload));
}

// --------------------------------------------------
// 4. UI更新とイベントバインディング
// --------------------------------------------------
function updateStatusUI(message, color) {
    if (statusIndicator) {
        statusIndicator.textContent = message;
        statusIndicator.style.color = color;
    }
}

function bindEvents() {
    // ボタンが存在する画面（index.html）のみイベントを登録
    if (startBtn) {
        startBtn.addEventListener('click', () => sendCommand('start'));
    }
    if (stopBtn) {
        stopBtn.addEventListener('click', () => sendCommand('stop'));
    }
    if (resetBtn) {
        resetBtn.addEventListener('click', () => sendCommand('reset'));
    }
}

// --------------------------------------------------
// 5. 描画ループ (requestAnimationFrame)
// --------------------------------------------------
function animationLoop() {
    // 現在の実装ではWebSocketのonmessageで直接DOMを更新していますが、
    // ネットワークのジッター（遅延の揺らぎ）を吸収して、より滑らかな60fps描画を行う場合は、
    // サーバーからの最終同期時刻と現在のローカル時刻の差分を計算して、ここで補間描画を行います。

    requestAnimationFrame(animationLoop);
}

// --------------------------------------------------
// アプリケーション起動
// --------------------------------------------------
document.addEventListener('DOMContentLoaded', () => {
    initWebSocket();
    bindEvents();
    requestAnimationFrame(animationLoop);
});