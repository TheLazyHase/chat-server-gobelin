package internal

import (
	"encoding/json"
)

//Stored metadata is a JSON object keyed by namespace
//Empty values give an empty map, never nil, so callers can add a namespace to it
func DecodeMetadata(raw []byte) (map[string]json.RawMessage, error) {
	var metadata map[string]json.RawMessage
	if len(raw) > 0 {
		err := json.Unmarshal(raw, &metadata)
		if err != nil {
			return nil, err
		}
	}
	if metadata == nil {
		metadata = make(map[string]json.RawMessage)
	}
	return metadata, nil
}
