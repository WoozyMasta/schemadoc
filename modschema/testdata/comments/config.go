package comments

// Config describes a comment normalization fixture. More type details follow.
type Config struct {
	// Text spans physically wrapped
	// source lines with [SomeType] and [SomeType.Method].
	//
	// See [the guide] and https://example.com/auto.
	//
	// # Allowed usage patterns
	//
	// Supported values:
	//
	//   - First item wrapped
	//     onto a second line.
	//   - Second item.
	//
	// Steps:
	//
	//  3. First step.
	//  4. Second step.
	//
	// Example:
	//
	//     first()
	//     second()
	//
	// [the guide]: https://example.com/guide
	Text string `json:"text"`
	// Tagged has a description supplied by a struct tag.
	Tagged string `json:"tagged" jsonschema_description:"Keep [SomeType] and  double spaces."`
}
