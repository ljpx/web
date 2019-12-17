package web

// Purifiable defines the methods that any purifiable request model must
// implement.  The intention is to allow request methods to be able to validate
// themselves - keeping the validation of user provided information out of
// routes.
//
// The first return value is the name of the invalid field, the second is the
// error describing the problem.
type Purifiable interface {
	Purify() (string, error)
}
