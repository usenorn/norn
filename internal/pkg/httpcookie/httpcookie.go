package httpcookie

import (
	"context"
	"net/http"
)

type jarKey struct{}

type Jar struct {
	cookies []*http.Cookie
}

func (j *Jar) Add(cookie *http.Cookie) {
	if j == nil || cookie == nil {
		return
	}

	j.cookies = append(j.cookies, cookie)
}

func (j *Jar) Cookies() []*http.Cookie {
	if j == nil {
		return nil
	}

	return j.cookies
}

func Into(ctx context.Context) (context.Context, *Jar) {
	jar := &Jar{}

	return context.WithValue(ctx, jarKey{}, jar), jar
}

func Pending(ctx context.Context) *Jar {
	jar, _ := ctx.Value(jarKey{}).(*Jar)

	return jar
}
