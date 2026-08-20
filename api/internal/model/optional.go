package model

import "encoding/json"

// Optional carries the three JSON states a PATCH field can have: absent, explicit
// null, and a value. Collapsing absent into null is what makes a partial update
// silently wipe fields the caller never sent.
type Optional[T any] struct {
	IsDefined bool
	Value     *T
}

func NewOptional[T any](value T) Optional[T] {
	return Optional[T]{IsDefined: true, Value: &value}
}

func NewOptionalPtr[T any](value *T) Optional[T] {
	return Optional[T]{IsDefined: true, Value: value}
}

func NewOptionalNull[T any]() Optional[T] {
	return Optional[T]{IsDefined: true}
}

func (o *Optional[T]) UnmarshalJSON(data []byte) error {
	o.IsDefined = true
	if string(data) == "null" {
		o.Value = nil
		return nil
	}
	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	o.Value = &value
	return nil
}

func (o Optional[T]) MarshalJSON() ([]byte, error) {
	if o.Value == nil {
		return []byte("null"), nil
	}
	return json.Marshal(o.Value)
}

// IsZero drives `json:",omitzero"`. Without it an undefined field marshals as an
// explicit null, which the receiver reads as "clear this field".
func (o Optional[T]) IsZero() bool { return !o.IsDefined }

// OrElse resolves a non-nullable column: undefined keeps current, defined null clears
// to the zero value.
func (o Optional[T]) OrElse(current T) T {
	if !o.IsDefined {
		return current
	}
	if o.Value == nil {
		var zero T
		return zero
	}
	return *o.Value
}

// PtrOrElse resolves a nullable column: undefined keeps current, defined null clears.
func (o Optional[T]) PtrOrElse(current *T) *T {
	if !o.IsDefined {
		return current
	}
	return o.Value
}
