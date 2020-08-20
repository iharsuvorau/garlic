package main

//func TestSessionStore_Get(t *testing.T) {
//	type fields struct {
//		filepath string
//		Sessions []*Session
//		mu       sync.RWMutex
//	}
//	type args struct {
//		id string
//	}
//	tests := []struct {
//		name    string
//		fields  fields
//		args    args
//		want    *Session
//		wantErr bool
//	}{
//		// TODO: Add test cases.
//	}
//	for _, tt := range tests {
//		t.Run(tt.name, func(t *testing.T) {
//			s := &SessionStore{
//				filepath: tt.fields.filepath,
//				Sessions: tt.fields.Sessions,
//				mu:       tt.fields.mu,
//			}
//			got, err := s.Get(tt.args.id)
//			if (err != nil) != tt.wantErr {
//				t.Errorf("Get() error = %v, wantErr %v", err, tt.wantErr)
//				return
//			}
//			if !reflect.DeepEqual(got, tt.want) {
//				t.Errorf("Get() got = %v, want %v", got, tt.want)
//			}
//		})
//	}
//}
