package main

import "os"
import "http"

type NoAuth struct {
	baseURL  string
	consumer string
}

func (noauth NoAuth) Login(baseURL string) os.Error {
	return nil
}

func (noauth NoAuth) Sign(req *http.Request) os.Error {
	return nil
}
