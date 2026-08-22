package engine

import "example.com/gflow/internal/model"

func Terminal(s string) bool {
	return s == model.Completed || s == model.Failed || s == model.Dead || s == model.Terminated
}
