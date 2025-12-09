package config

// StringValue represents a string configuration value with its source.
type StringValue struct {
	Value  string
	Source ConfigSource
}

// IntValue represents an int configuration value with its source.
type IntValue struct {
	Value  int
	Source ConfigSource
}

// BoolValue represents a bool configuration value with its source.
type BoolValue struct {
	Value  bool
	Source ConfigSource
}

// NewStringValue creates a new StringValue with default source.
func NewStringValue(value string) StringValue {
	return StringValue{Value: value, Source: SourceDefault}
}

// NewIntValue creates a new IntValue with default source.
func NewIntValue(value int) IntValue {
	return IntValue{Value: value, Source: SourceDefault}
}

// NewBoolValue creates a new BoolValue with default source.
func NewBoolValue(value bool) BoolValue {
	return BoolValue{Value: value, Source: SourceDefault}
}
