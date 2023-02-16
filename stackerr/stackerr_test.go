package stackerr

import "testing"

func TestStackErr_Error(t *testing.T) {
	err := New()
	t.Log(err.Error())
}
