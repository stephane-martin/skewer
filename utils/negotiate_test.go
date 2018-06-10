package utils

import (
	"reflect"
	"testing"
)

func Test_parseOneAccept(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		expected []AcceptSpec
	}{
		{
			name:   "basic",
			header: "text/html",
			expected: []AcceptSpec{
				{
					Range:   "text/html",
					Quality: 1,
					Params:  []string{},
				},
			},
		},
		{
			name:   "star",
			header: "image/*",
			expected: []AcceptSpec{
				{
					Range:   "image/*",
					Quality: 1,
					Params:  []string{},
				},
			},
		},
		{
			name:   "typical",
			header: "text/html, application/xhtml+xml, application/xml;q=0.9, */*;q=0.8",
			expected: []AcceptSpec{
				{
					Range:   "text/html",
					Quality: 1,
					Params:  []string{},
				},
				{
					Range:   "application/xhtml+xml",
					Quality: 1,
					Params:  []string{},
				},
				{
					Range:   "application/xml",
					Quality: 0.9,
					Params:  []string{},
				},
				{
					Range:   "*/*",
					Quality: 0.8,
					Params:  []string{},
				},
			},
		},
		{
			name:   "fairly complex",
			header: "audio/*; q=0.2, audio/basic",
			expected: []AcceptSpec{
				{
					Range:   "audio/*",
					Quality: 0.2,
					Params:  []string{},
				},
				{
					Range:   "audio/basic",
					Quality: 1.0,
					Params:  []string{},
				},
			},
		},
		{
			name:   "newline",
			header: "text/plain,\ntext/x-dvi",
			expected: []AcceptSpec{
				{
					Range:   "text/plain",
					Quality: 1.0,
					Params:  []string{},
				},
				{
					Range:   "text/x-dvi",
					Quality: 1.0,
					Params:  []string{},
				},
			},
		},
		{
			name:   "attributes",
			header: "text/*;q=0.8, text/plain;q=0.9, text/plain;format=flowed;charset=utf-8",
			expected: []AcceptSpec{
				{
					Range:   "text/*",
					Quality: 0.8,
					Params:  []string{},
				},
				{
					Range:   "text/plain",
					Quality: 0.9,
					Params:  []string{},
				},
				{
					Range:   "text/plain",
					Quality: 1.0,
					Params:  []string{"format=flowed", "charset=utf-8"},
				},
			},
		},
		{
			name:   "full",
			header: "text/*;q=0.3, text/html;q=0.7, text/html;level=1, text/html;level=2;q=0.4, */*;q=0.5",
			expected: []AcceptSpec{
				{
					Range:   "text/*",
					Quality: 0.3,
					Params:  []string{},
				},
				{
					Range:   "text/html",
					Quality: 0.7,
					Params:  []string{},
				},
				{
					Range:   "text/html",
					Quality: 1.0,
					Params:  []string{"level=1"},
				},
				{
					Range:   "text/html",
					Quality: 0.4,
					Params:  []string{"level=2"},
				},
				{
					Range:   "*/*",
					Quality: 0.5,
					Params:  []string{},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := make([]AcceptSpec, 0)
			result = parseOneAccept(tt.header, result)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("parseOneAccept() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestParseAccept(t *testing.T) {
	tests := []struct {
		name     string
		headers  []string
		expected []AcceptSpec
	}{
		{
			name:    "multiple",
			headers: []string{"text/html", "text/plain"},
			expected: []AcceptSpec{
				{
					Range:   "text/html",
					Quality: 1,
					Params:  []string{},
				},
				{
					Range:   "text/plain",
					Quality: 1,
					Params:  []string{},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotSpecs := ParseAccept(tt.headers); !reflect.DeepEqual(gotSpecs, tt.expected) {
				t.Errorf("ParseAccept() = %v, want %v", gotSpecs, tt.expected)
			}
		})
	}
}

func Test_negotiateContentType(t *testing.T) {
	tests := []struct {
		name         string
		clientSpecs  []AcceptSpec
		serverOffers []string
		defaultOffer string
		expected     string
	}{
		{
			name: "fff",
			clientSpecs: []AcceptSpec{
				{
					Quality: 0.2,
					Range:   "audio/*",
				},
				{
					Quality: 1,
					Range:   "audio/basic",
				},
			},
			serverOffers: []string{"audio/basic", "text/html"},
			defaultOffer: "text/html",
			expected:     "audio/basic",
		},
		{
			name: "ggg",
			clientSpecs: []AcceptSpec{
				{
					Quality: 0.2,
					Range:   "audio/*",
				},
				{
					Quality: 1,
					Range:   "audio/basic",
				},
			},
			serverOffers: []string{"audio/mp3", "text/html"},
			defaultOffer: "text/html",
			expected:     "audio/mp3",
		},
		{
			name: "hhh",
			clientSpecs: []AcceptSpec{
				{
					Quality: 0.2,
					Range:   "audio/*",
				},
				{
					Quality: 1,
					Range:   "audio/basic",
				},
			},
			serverOffers: []string{"text/plain", "text/html"},
			defaultOffer: "text/html",
			expected:     "text/html",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := negotiateContentType(tt.clientSpecs, tt.serverOffers, tt.defaultOffer); got != tt.expected {
				t.Errorf("negotiateContentType() = %v, want %v", got, tt.expected)
			}
		})
	}
}
