package entity

import "errors"

var ErrMailDeliveryNotConfigured = errors.New("email delivery is not configured on this instance")

type Mail struct {
	To        string
	Subject   string
	PlainBody string
	HTMLBody  string
}
