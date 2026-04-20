package controller

import "testing"

func TestShouldRetryChannelTestWithStream(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "explicit stream required",
			err:  assertErr("bad response status code 400, message: Stream must be set to true, body: {\"detail\":\"Stream must be set to true\"}"),
			want: true,
		},
		{
			name: "stream only provider",
			err:  assertErr("bad response status code 400, message: this endpoint is stream only"),
			want: true,
		},
		{
			name: "unrelated error",
			err:  assertErr("bad response status code 400, message: invalid model"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldRetryChannelTestWithStream(tt.err); got != tt.want {
				t.Fatalf("shouldRetryChannelTestWithStream() = %v, want %v", got, tt.want)
			}
		})
	}
}

func assertErr(message string) error {
	return &testErr{message: message}
}

type testErr struct {
	message string
}

func (e *testErr) Error() string {
	return e.message
}
