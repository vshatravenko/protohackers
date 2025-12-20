package main

import "log/slog"

func handlePayload(payload []byte) {
	msgType := uint8(payload[0])
	payload = payload[1:]

	switch msgType {
	case msgTypes["error"]:
		parseErrorMsg(payload)
	case msgTypes["plate"]:
		parsePlateMsg(payload)
	case msgTypes["ticket"]:
		slog.Debug("parsing ticket msg")
	case msgTypes["want_heartbeat"]:
		parseWantHeartbeatMsg(payload)
	case msgTypes["heartbeat"]:
		slog.Debug("parsing heartbeat msg")
	case msgTypes["camera"]:
		parseCameraMsg(payload)
	case msgTypes["dispatcher"]:
		parseDispatcherMsg(payload)
	}
}
