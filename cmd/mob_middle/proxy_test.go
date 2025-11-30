package main

import "testing"

var cases map[string][]string = map[string][]string{
	"valid address - start of message":            {"7iKDZEwPZSqIvDnHvVN2r0hUWXD5rHX - my address", "7YWHMfk9JZe0LM0g1ZauHuiSxhI - my address"},
	"valid address - middle of message":           {"So here's my address - 7iKDZEwPZSqIvDnHvVN2r0hUWXD5rHX - got it?", "So here's my address - 7YWHMfk9JZe0LM0g1ZauHuiSxhI - got it?"},
	"valid address - end of message":              {"So here's my address - 7iKDZEwPZSqIvDnHvVN2r0hUWXD5rHX", "So here's my address - 7YWHMfk9JZe0LM0g1ZauHuiSxhI"},
	"valid address - multiple":                    {"So here's my address - 7iKDZEwPZSqIvDnHvVN2r0hUWXD5rHX and 7iKDZEwPZSqIvDnHvVN2r0hUWXD5rHX", "So here's my address - 7YWHMfk9JZe0LM0g1ZauHuiSxhI and 7YWHMfk9JZe0LM0g1ZauHuiSxhI"},
	"valid address - multiple - different length": {"Please pay the ticket price of 15 Boguscoins to one of these addresses: 7PLLOryXl6gIvb98RRbErZ5BrsDN 7LjRDAYK2xPAEpTl2BuKheB7kU 7dzaKGyh0LRJoT13TGHgcg487ZrWckC", "Please pay the ticket price of 15 Boguscoins to one of these addresses: 7YWHMfk9JZe0LM0g1ZauHuiSxhI 7YWHMfk9JZe0LM0g1ZauHuiSxhI 7YWHMfk9JZe0LM0g1ZauHuiSxhI"},
	"invalid address - no space":                  {"So here's my address - 7iKDZEwPZSqIvDnHvVN2r0hUWXD5rHX- got it?", "So here's my address - 7iKDZEwPZSqIvDnHvVN2r0hUWXD5rHX- got it?"},
	"invalid address - too short":                 {"So here's my address - 7iKDZEwPZSqIvDnHvVN2r0abc", "So here's my address - 7iKDZEwPZSqIvDnHvVN2r0abc"},
	"invalid address - too long":                  {"So here's my address - 7iKDZEwPZSqIvDnHvVN2r0hUWXD5rHXabcde", "So here's my address - 7iKDZEwPZSqIvDnHvVN2r0hUWXD5rHXabcde"},
}

func TestBogusAddrReplace(t *testing.T) {
	for title, values := range cases {
		t.Logf("Testing %s\n", title)
		input, expected := values[0], values[1]
		actual := string(replaceBogusAddr([]byte(input)))

		if actual != expected {
			t.Errorf("%s case failed:\nexpected: %s\nactual:   %s", title, expected, actual)
		}
	}
}
