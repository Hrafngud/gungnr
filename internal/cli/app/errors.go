package app

import "fmt"

type FriendlyError struct {
	Message string
	Hint    string
	Err     error
}

func (e FriendlyError) Error() string {
	if e.Err == nil {
		return e.Message
	}
	return fmt.Sprintf("%s: %v", e.Message, e.Err)
}

func (e FriendlyError) Unwrap() error {
	return e.Err
}
