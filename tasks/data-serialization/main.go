package main

import (
	"bytes"
	"encoding/gob"
	"time"
)

func encode(gl GameLog) ([]byte, error) {
	// ?
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)

	if err := enc.Encode(gl); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decode(data []byte) (GameLog, error) {
	// ?
	buf := bytes.NewBuffer(data)
	dec := gob.NewDecoder(buf)
	var gl GameLog
	if err := dec.Decode(&gl); err != nil {
		return GameLog{}, err
	}
	return gl, nil
}

// don't touch below this line

type GameLog struct {
	CurrentTime time.Time
	Message     string
	Username    string
}
