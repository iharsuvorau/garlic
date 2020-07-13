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
