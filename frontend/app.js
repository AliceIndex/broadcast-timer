// デプロイ時に取得した本番環境のURL
const API_URL = "wss://2cz26o6t9k.execute-api.ap-northeast-1.amazonaws.com/prod"; //[cite: 1]


let ws; //[cite: 1]
let isConnected = false; //[cite: 1]

// --------------------------------------------------
// 1. UI要素の取得[cite: 1]
// --------------------------------------------------
// index.html と monitor.html の両方でこのIDを使用します
const timecodeDisplay = document.getElementById('timecode-display'); //[cite: 1]
const startBtn = document.getElementById('btn-start'); //[cite: 1]
const stopBtn = document.getElementById('btn-stop'); //[cite: 1]
const resetBtn = document.getElementById('btn-reset'); //[cite: 1]
const statusIndicator = document.getElementById('status-indicator'); //[cite: 1]

// 内部状態の保持[cite: 1]
let currentState = { //[cite: 1]
    status: 'stopped', // 'running', 'stopped'[cite: 1]
    timecode: '00:00:00:00' //[cite: 1]
};

// --------------------------------------------------
// 2. WebSocketの初期化と接続管理[cite: 1]
// --------------------------------------------------
function initWebSocket() { //[cite: 1]
    ws = new WebSocket(API_URL); //[cite: 1]

    ws.onopen = () => { //[cite: 1]
        isConnected = true; //[cite: 1]
        updateStatusUI('Connected', 'var(--color-success, #28a745)'); //[cite: 1]
        console.log('WebSocket Connected'); //[cite: 1]
    };

    ws.onmessage = (event) => { //[cite: 1]
        try { //[cite: 1]
            const data = JSON.parse(event.data); //[cite: 1]
            handleServerMessage(data); //[cite: 1]
        } catch (error) { //[cite: 1]
            console.error('JSON Parse Error:', error); //[cite: 1]
        }
    };

    ws.onclose = () => { //[cite: 1]
        isConnected = false; //[cite: 1]
        updateStatusUI('Disconnected', 'var(--color-danger, #dc3545)'); //[cite: 1]
        console.log('WebSocket Disconnected. Reconnecting in 3 seconds...'); //[cite: 1]
        // ネットワーク切断時やサーバー再起動時は3秒後に自動再接続[cite: 1]
        setTimeout(initWebSocket, 3000); //[cite: 1]
    };

    ws.onerror = (error) => { //[cite: 1]
        console.error('WebSocket Error:', error); //[cite: 1]
    };
}

// ★追加: モーター（タイマー）を管理するための変数
let clockInterval = null;

// --------------------------------------------------
// 3. メッセージ送受信のハンドリング
// --------------------------------------------------
function handleServerMessage(data) {
    if (data.action === 'sync' || data.timecode) {
        // 修正1: data.status ではなく data.state に直す！
        currentState.state = data.state || currentState.state;
        
        // サーバーから送られてきた基準時間（スタートした瞬間の時間）を保存
        currentState.reference_utc = data.reference_utc || currentState.reference_utc;

        if (currentState.state === 'running') {
            // ★修正2-A: スタートの合図が来たら、時計のモーターを回す！
            startClockMotor();
        } else if (currentState.state === 'reset') {
            // ② ★追加：リセット処理
            stopClockMotor();
            if (timecodeDisplay) {
                // 初期値のタイムコードに強制上書き（環境に合わせて変えてください）
                timecodeDisplay.textContent = "00:00:00:00"; 
            }
        } else {
            // ストップの合図ならモーターを止める
            stopClockMotor();
            // 止まった瞬間のタイムコードを1回だけ画面に更新しておく
            if (timecodeDisplay && data.timecode) {
                timecodeDisplay.textContent = data.timecode;
            }
        }
    }
}

// ★ 時計の針を動かし続けるモーター関数
function startClockMotor() {
    // すでにモーターが動いていたら二重起動を防ぐ
    if (clockInterval) clearInterval(clockInterval);

    // 約33ミリ秒ごとに画面を更新し続ける
    clockInterval = setInterval(() => {
        // 1. スタートした基準時間から、今何ミリ秒経ったかを計算
        const elapsedMs = Date.now() - currentState.reference_utc;

        // 2. ミリ秒をプロ仕様のタイムコードに変換（FPSとDF設定は送信データに合わせる）
        // ※今回は送信データの fps: 30.0, is_df: false に合わせています
        const currentTimecode = calculateTimecode(elapsedMs, 30.0, false);

        // 3. 画面の文字を書き換える
        if (timecodeDisplay) {
            timecodeDisplay.textContent = currentTimecode;
        }
    }, 33);
}


// ★追加: 時計を止める関数
function stopClockMotor() {
    if (clockInterval) {
        clearInterval(clockInterval);
        clockInterval = null;
    }
}


function sendCommand(actionName, state) { //[cite: 1]
    // ★ 1. まずボタンが反応しているかブラウザのコンソールに出す
    console.log("ボタンが押されました！ 送信しようとしているアクション:", actionName);

    if (!isConnected || ws.readyState !== WebSocket.OPEN) {
        // ★ 2. もし接続が原因で送れないなら、その理由を叫ぶ
        console.warn('送信を中止しました: WebSocketがまだ繋がっていません。状態:', ws ? ws.readyState : '未定義');
        return;
    }

    // Lambdaのテストケースで成功した「body」の中身と完全に一致させる
    const payload = {
        "action": actionName, // ★ここがAPI Gatewayのルート判定に使われます
        "state": state,
        "reference_utc": Date.now(),
        "base_frames": 0,
        "fps": 30.0,
        "is_df": false
    };

    console.log("送信データ:", payload);
    ws.send(JSON.stringify(payload));
}

// --------------------------------------------------
// 4. UI更新とイベントバインディング[cite: 1]
// --------------------------------------------------
function updateStatusUI(message, color) { //[cite: 1]
    if (statusIndicator) { //[cite: 1]
        statusIndicator.textContent = message; //[cite: 1]
        statusIndicator.style.color = color; //[cite: 1]
    }
}

function bindEvents() { //[cite: 1]
    // コントローラー画面のボタンイベント[cite: 1]
    if (startBtn) { //[cite: 1]
        startBtn.addEventListener('click', () => sendCommand('start', 'running')); //[cite: 1]
    }
    if (stopBtn) { //[cite: 1]
        stopBtn.addEventListener('click', () => sendCommand('stop', 'stopped')); //[cite: 1]
    }
    if (resetBtn) { //[cite: 1]
        resetBtn.addEventListener('click', () => sendCommand('reset', 'reset')); //[cite: 1]
    }

    // モニター画面専用：ダブルクリックでフルスクリーン切り替え[cite: 3]
    window.addEventListener('dblclick', () => { //[cite: 3]
        if (!document.fullscreenElement) { //[cite: 3]
            document.documentElement.requestFullscreen(); //[cite: 3]
            if (navigator.wakeLock) {
                navigator.wakeLock.request('screen').catch(console.error); // スリープ防止[cite: 3]
            }
        } else {
            if (document.exitFullscreen) {
                document.exitFullscreen(); //[cite: 3]
            }
        }
    });
}
// --------------------------------------------------
// タイムコード計算エンジン (timecode.goからの移植)
// --------------------------------------------------
function calculateTimecode(elapsedMs, fps, isDropFrame) {
    // 1. 経過時間(ミリ秒)から、通算フレーム数を計算
    let frames = Math.floor(elapsedMs * fps / 1000);

    const fpsRound = Math.round(fps);

    // 2. ドロップフレーム(DF)補正
    if (isDropFrame && (fpsRound === 30 || fpsRound === 60)) {
        let dropPerMinute = 2;
        if (fpsRound === 60) {
            dropPerMinute = 4;
        }
        const totalMinutes = Math.floor(frames / (fpsRound * 60));
        // 10分ごとの例外処理を含むドロップフレーム補正
        const dropFrames = dropPerMinute * (totalMinutes - Math.floor(totalMinutes / 10));
        frames += dropFrames;
    }

    // 3. 24時間ロールオーバー処理
    frames = frames % (fpsRound * 3600 * 24);

    // 4. 時・分・秒・フレームの割り出し
    const f = frames % fpsRound;
    const s = Math.floor(frames / fpsRound) % 60;
    const m = Math.floor(frames / (fpsRound * 60)) % 60;
    const h = Math.floor(frames / (fpsRound * 3600)) % 24;

    // 5. フォーマット（区切り文字）
    const sep = isDropFrame ? ";" : ":";

    // 2桁のゼロ埋め用ヘルパー関数
    const pad = (num) => String(num).padStart(2, '0');

    // HH:mm:ss:ff の形式で返す
    return `${pad(h)}:${pad(m)}:${pad(s)}${sep}${pad(f)}`;
}

// --------------------------------------------------
// アプリケーション起動[cite: 1]
// --------------------------------------------------
document.addEventListener('DOMContentLoaded', () => { //[cite: 1]
    initWebSocket(); //[cite: 1]
    bindEvents(); //[cite: 1]
});
