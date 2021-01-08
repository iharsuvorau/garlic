package instruction

//
//func TestPepperMessage_MarshalJSON(t *testing.T) {
//	mustBytes := func(b []byte, err error) []byte {
//		if err != nil {
//			panic(err)
//		}
//		return b
//	}
//
//	type fields struct {
//		Command Command
//		Content string
//		Delay   int64
//	}
//	tests := []struct {
//		name    string
//		fields  fields
//		want    []byte
//		wantErr bool
//	}{
//		{
//			name: "A",
//			fields: fields{
//				Command: MoveCommand,
//				Content: "",
//				Delay:   2000,
//			},
//			want:    mustBytes(json.Marshal(map[string]interface{}{"command": "move", "content": "", "delay": 2000})),
//			wantErr: false,
//		},
//		{
//			name: "B",
//			fields: fields{
//				Command: ActionCommand,
//				Content: "",
//			},
//			want:    mustBytes(json.Marshal(map[string]interface{}{"command": "sayAndMove", "content": "", "delay": 0})),
//			wantErr: false,
//		},
//		{
//			name: "C",
//			fields: fields{
//				Command: SayCommand,
//				Content: "",
//			},
//			want:    mustBytes(json.Marshal(map[string]interface{}{"command": "say", "content": "", "delay": 0})),
//			wantErr: false,
//		},
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			pm := &PepperMessage{
//				Command: tt.fields.Command,
//				Content: tt.fields.Content,
//				Delay:   tt.fields.Delay,
//			}
//			got, err := pm.MarshalJSON()
//			if (err != nil) != tt.wantErr {
//				t.Errorf("MarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
//				return
//			}
//			if !reflect.DeepEqual(got, tt.want) {
//				t.Errorf("MarshalJSON() got = %s, want %s", got, tt.want)
//			}
//		})
//	}
//}
//
//func Test_castDelay(t *testing.T) {
//	type args struct {
//		delay interface{}
//	}
//	tests := []struct {
//		name             string
//		args             args
//		wantDelaySeconds int64
//		wantErr          bool
//	}{
//		{
//			name:             "A",
//			args:             args{delay: 5000000000},
//			wantDelaySeconds: 5000000000,
//			wantErr:          false,
//		},
//		{
//			name:             "B",
//			args:             args{delay: "300"},
//			wantDelaySeconds: 300,
//			wantErr:          false,
//		},
//		{
//			name:             "C",
//			args:             args{delay: "3s"},
//			wantDelaySeconds: 0,
//			wantErr:          true,
//		},
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			gotDelaySeconds, err := castDelay(tt.args.delay)
//			if (err != nil) != tt.wantErr {
//				t.Errorf("castDelay() error = %v, wantErr %v", err, tt.wantErr)
//				return
//			}
//			if gotDelaySeconds != tt.wantDelaySeconds {
//				t.Errorf("castDelay() gotDelaySeconds = %v, want %v", gotDelaySeconds, tt.wantDelaySeconds)
//			}
//		})
//	}
//}
