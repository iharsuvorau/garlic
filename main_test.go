package main

import (
	"reflect"
	"testing"
)

func Test_collectMotions(t *testing.T) {
	type args struct {
		dataDir string
	}
	tests := []struct {
		name    string
		args    args
		want    []*MoveAction
		wantErr bool
	}{
		{
			name:    "A",
			args:    args{dataDir: "data/pepper-core-anims-master/"},
			want:    nil,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := collectMoves(tt.args.dataDir)
			if (err != nil) != tt.wantErr {
				t.Errorf("collectMoves() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("collectMoves() got = %v, want %v", got, tt.want)
			}
		})
	}
}

//func Test_PepperTaskMarshal(t *testing.T) {
//	var err error
//
//	moves, err = collectMoves("data/pepper-core-anims-master")
//	if err != nil {
//		t.Fatal(err)
//	}
//
//	sessions, err = collectSessions("data", &moves)
//	if err != nil {
//		t.Fatal(err)
//	}
//
//	content, err := sessions[0].Items[0].Question.MoveItem.Content()
//	if err != nil {
//		t.Fatalf("path: %v, err: %v", sessions[0].Items[0].Question.MoveItem.FilePath, err)
//	}
//
//	tt := PepperMessage{
//		Command: sessions[0].Items[0].Question.MoveItem.Command(),
//		Content: content,
//	}
//
//	b, err := json.Marshal(tt)
//	if err != nil {
//		t.Fatal(err)
//	}
//
//	if len(b) == 0 {
//		t.Fatal("nil bytes")
//	}
//}
