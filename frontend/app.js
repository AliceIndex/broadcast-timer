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
        currentState.state = data.state || currentState.state;
        currentState.reference_utc = data.reference_utc || currentState.reference_utc;
        
        // ★サーバーから配られた「設定」と「下駄（base_frames）」を保存
        currentState.fps = data.fps || 30.0;
        currentState.is_df = data.is_df || false;
        currentState.base_frames = data.base_frames || 0; 

        if (currentState.state === 'running') {
            startClockMotor();
        } else if (currentState.state === 'reset') {
            stopClockMotor();
            if (timecodeDisplay) {
                // ★リセット時は、下駄のフレーム数(base_frames)を文字に戻して表示する
                timecodeDisplay.textContent = framesToTimecode(currentState.base_frames, currentState.fps, currentState.is_df);
            }
        } else {
            stopClockMotor();
            if (timecodeDisplay && data.timecode) {
                timecodeDisplay.textContent = data.timecode;
            }
        }
    }
}

function startClockMotor() {
    if (clockInterval) clearInterval(clockInterval);

    clockInterval = setInterval(() => {
        // 1. スタートしてから何ミリ秒経ったか
        const elapsedMs = Date.now() - currentState.reference_utc;
        
        // 2. そのミリ秒をフレーム数に変換
        const elapsedFrames = Math.floor(elapsedMs * currentState.fps / 1000);
        
        // 3. ★「配られた下駄(base_frames)」＋「経過フレーム数」
        const totalFrames = currentState.base_frames + elapsedFrames;

        // 4. 合計フレーム数を文字列にして画面に表示
        const currentTimecode = framesToTimecode(totalFrames, currentState.fps, currentState.is_df);

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


function sendCommand(actionName, state) {
    if (!isConnected || ws.readyState !== WebSocket.OPEN) return;

    // UIから設定値を取得
    const selectedFps = parseFloat(document.getElementById('fpsSelect').value);
    const startTimeStr = document.getElementById('startTimeInput').value;

    // ★ブラウザ側で、開始時間(01:00:00:00)を、下駄となるフレーム数(base_frames)に変換する
    const baseFrames = tcToFrames(startTimeStr, selectedFps);

    const payload = {
        "action": actionName,
        "state": state,
        "reference_utc": Date.now(),
        "base_frames": baseFrames, // ★文字列ではなく、計算済みのフレーム数を送る！
        "fps": selectedFps,
        "is_df": selectedFps === 29.97 || selectedFps === 59.94
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

// タイムコード文字列(HH:mm:ss:ff)を、指定FPSでの通算フレーム数に変換する
function tcToFrames(tc, fps) {
    const parts = tc.replace(';', ':').split(':').map(Number);
    if (parts.length !== 4) return 0;
    const [h, m, s, f] = parts;
    const fpsRound = Math.round(fps);
    return (((h * 3600) + (m * 60) + s) * fpsRound) + f;
}

// 通算フレーム数を、プロ仕様のタイムコード文字列に変換する (timecode.goのJS版)
function framesToTimecode(frames, fps, isDropFrame) {
    const fpsRound = Math.round(fps);

    if (isDropFrame && (fpsRound === 30 || fpsRound === 60)) {
        let dropPerMinute = 2;
        if (fpsRound === 60) dropPerMinute = 4;
        const totalMinutes = Math.floor(frames / (fpsRound * 60));
        const dropFrames = dropPerMinute * (totalMinutes - Math.floor(totalMinutes / 10));
        frames += dropFrames;
    }

    frames = frames % (fpsRound * 3600 * 24);

    const f = frames % fpsRound;
    const s = Math.floor(frames / fpsRound) % 60;
    const m = Math.floor(frames / (fpsRound * 60)) % 60;
    const h = Math.floor(frames / (fpsRound * 3600)) % 24;

    const sep = isDropFrame ? ";" : ":";
    const pad = (num) => String(num).padStart(2, '0');

    return `${pad(h)}:${pad(m)}:${pad(s)}${sep}${pad(f)}`;
}

// URLから room と pin を取得
const urlParams = new URLSearchParams(window.location.search);
const roomID = urlParams.get('room');
const roomPIN = urlParams.get('pin');
const mode = urlParams.get('mode'); // 'new' か 'join'

// もし情報がなければトップに戻す
if (!roomID || !roomPIN) {
    window.location.href = 'index.html';
}

// 画面に表示
document.getElementById('display-room-id').textContent = roomID;
document.getElementById('display-room-pin').textContent = roomPIN;

// WebSocket接続が確立した時の処理を修正
ws.onopen = () => {
    isConnected = true;
    console.log('Connected to Server');

    // ★入室（または作成）をサーバーに知らせる
    const joinPayload = {
        action: "join",
        room_id: roomID,
        pin: roomPIN,
        mode: mode // 新規なら初期化、既存ならロードをサーバーに促す
    };
    ws.send(JSON.stringify(joinPayload));
};

// --------------------------------------------------
// アプリケーション起動[cite: 1]
// --------------------------------------------------
document.addEventListener('DOMContentLoaded', () => { //[cite: 1]
    initWebSocket(); //[cite: 1]
    bindEvents(); //[cite: 1]
});
