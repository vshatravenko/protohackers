package main

import (
	"encoding/binary"
	"fmt"
)

const (
	actionInsert = 'I'
	actionQuery  = 'Q'
	payloadLen   = 9
)

type payload struct {
	action   byte
	segment1 int32
	segment2 int32
}

func parseRawPayload(raw []byte) (*payload, error) {
	if len(raw) > payloadLen || len(raw) == 0 {
		return nil, fmt.Errorf("unexpected payload length %d, expecting %d", len(raw), payloadLen)
	}

	action := raw[0]
	if action != actionInsert && action != actionQuery {
		return nil, fmt.Errorf("invalid action %s, expecting %s or %s", string(action), string(actionInsert), string(actionQuery))
	}

	segment1 := int32(binary.BigEndian.Uint32(raw[1:5]))
	segment2 := int32(binary.BigEndian.Uint32(raw[5:]))

	res := &payload{
		action:   action,
		segment1: segment1,
		segment2: segment2,
	}

	return res, nil
}

func (p *payload) toBytes() []byte {
	res := make([]byte, 1)
	res[0] = p.action

	segment1, segment2 := make([]byte, 4), make([]byte, 4)
	binary.BigEndian.PutUint32(segment1, uint32(p.segment1))
	binary.BigEndian.PutUint32(segment2, uint32(p.segment2))

	res = append(res, segment1...)
	res = append(res, segment2...)

	return res
}
