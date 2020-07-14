package main

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestPepperMessage_MarshalJSON(t *testing.T) {
	type fields struct {
		Command Command
		Content []byte
		Delay   int64
	}
	tests := []struct {
		name    string
		fields  fields
		want    []byte
		wantErr bool
	}{
		{
			name: "A",
			fields: fields{
				Command: MoveCommand,
				Content: []byte{},
				Delay:   2000,
			},
			want:    MustBytes(json.Marshal(map[string]interface{}{"command": "move", "content": []byte{}, "delay": 2000})),
			wantErr: false,
		},
		{
			name: "B",
			fields: fields{
				Command: SayAndMoveCommand,
				Content: []byte{},
			},
			want:    MustBytes(json.Marshal(map[string]interface{}{"command": "sayAndMove", "content": []byte{}, "delay": 0})),
			wantErr: false,
		},
		{
			name: "C",
			fields: fields{
				Command: SayCommand,
				Content: []byte{},
			},
			want:    MustBytes(json.Marshal(map[string]interface{}{"command": "say", "content": []byte{}, "delay": 0})),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pm := &PepperMessage{
				Command: tt.fields.Command,
				Content: tt.fields.Content,
				Delay:   tt.fields.Delay,
			}
			got, err := pm.MarshalJSON()
			if (err != nil) != tt.wantErr {
				t.Errorf("MarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MarshalJSON() got = %s, want %s", got, tt.want)
			}
		})
	}
}
