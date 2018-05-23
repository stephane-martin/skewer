package parser

import (
	"net"
	"net/url"
	"strconv"
)

func makeStr(s string) interface{} {
	if s == "-" {
		return ""
	}
	return s
}

func makeIP(s string) interface{} {
	if s == "-" || s == "" {
		return nil
	}
	ip := net.ParseIP(s)
	if ip == nil {
		// necessary to return untyped nil
		return nil
	}
	return ip
}

func makeInt(s string) interface{} {
	if s == "-" || s == "" {
		return nil
	}
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil
	}
	return i
}

func makeFloat(s string) interface{} {
	if s == "-" || s == "" {
		return nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return f
}

func decodeURI(s string) interface{} {
	if s == "-" || s == "" {
		return ""
	}
	uri, err := url.QueryUnescape(s)
	if err != nil {
		return s
	}
	return uri
}

