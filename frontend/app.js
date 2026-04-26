class BroadcastTimer {
    constructor(wsUrl, onSyncCallback) {
        this.socket = new WebSocket(wsUrl);
        this.onSync = onSyncCallback;
        this.state = {
            running: false,
            referenceUTC: 0,
            baseFrames: 0,
            fps: 29.97,
            isDF: true
        };

        this.socket.onmessage = (event) => {
            const data = JSON.parse(event.data);
            if (data.action === "sync") {
                this.state = {
                    running: data.state === "running",
                    referenceUTC: data.reference_utc,
                    baseFrames: data.base_frames,
                    fps: data.fps,
                    isDF: data.is_df
                };
                if (this.onSync) this.onSync();
            }
        };
    }

    getCurrentFrame() {
        if (!this.state.running) return this.state.baseFrames;
        const now = Date.now();
        const diffMs = now - this.state.referenceUTC;
        const diffFrames = Math.floor(diffMs * (this.state.fps / 1000));
        return this.state.baseFrames + diffFrames;
    }

    formatTimecode(frames) {
        const fpsRound = Math.round(this.state.fps);
        let f = frames;

        if (this.state.isDF && (fpsRound === 30 || fpsRound === 60)) {
            const drop = fpsRound === 60 ? 4 : 2;
            const totalMins = Math.floor(f / (fpsRound * 60));
            f += drop * (totalMins - Math.floor(totalMins / 10));
        }

        f = f % (fpsRound * 3600 * 24);
        const pad = (n) => n.toString().padStart(2, '0');
        
        const ff = f % fpsRound;
        const s = Math.floor(f / fpsRound) % 60;
        const m = Math.floor(f / (fpsRound * 60)) % 60;
        const h = Math.floor(f / (fpsRound * 3600)) % 24;

        const sep = this.state.isDF ? ';' : ':';
        return `${pad(h)}:${pad(m)}:${pad(s)}${sep}${pad(ff)}`;
    }

    sendAction(stateName) {
        const payload = {
            action: "timer_action",
            state: stateName,
            reference_utc: Date.now(),
            base_frames: this.getCurrentFrame(),
            fps: this.state.fps,
            is_df: this.state.isDF
        };
        this.socket.send(JSON.stringify(payload));
    }
}