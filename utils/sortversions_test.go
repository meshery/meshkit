package utils

import "testing"

type TestCases struct {
	ExpectedArray []string
	PassedArray   []string
}

var testcases = []TestCases{
	{
		PassedArray:   []string{"1.0.0", "0.5.6", "8.9.6"},
		ExpectedArray: []string{"0.5.6", "1.0.0", "8.9.6"},
	},
	{
		PassedArray:   []string{"v1.0.0", "0.5.6", "fasdfv8.9.6"},
		ExpectedArray: []string{"0.5.6", "v1.0.0", "fasdfv8.9.6"},
	},
	{
		PassedArray:   []string{"v1.0.0-alpha", "v1.0.0-alpha-beta", "v1.0.0-alpha-rc", "1.0.0-rc", "1.0.0"},
		ExpectedArray: []string{"v1.0.0-alpha-beta", "v1.0.0-alpha-rc", "v1.0.0-alpha", "1.0.0-rc", "1.0.0"},
	},
	{
		PassedArray:   []string{"v1.0.0.0", "1.0.0"},
		ExpectedArray: []string{"v1.0.0.0", "1.0.0"},
	},
	{
		PassedArray:   []string{"v1.0.0.0-alpha", "asffdsafgaga1.sag0.0######"},
		ExpectedArray: []string{"v1.0.0.0-alpha", "asffdsafgaga1.sag0.0######"},
	},
	{
		PassedArray:   []string{"1.0.0-alpha.beta", "1.0.0-alpha.1"},
		ExpectedArray: []string{"1.0.0-alpha.beta", "1.0.0-alpha.1"},
	},
	{
		PassedArray:   []string{"v1.0.0-stable", "0.9.8", "1.0.0-alpha.1", "v1.0.0-alpha"},
		ExpectedArray: []string{"0.9.8", "1.0.0-alpha.1", "v1.0.0-alpha", "v1.0.0-stable"},
	},
}

func TestSort(t *testing.T) {
	for _, tc := range testcases {
		SortDottedStringsByDigits(tc.PassedArray)
		if !isEqual(tc.PassedArray, tc.ExpectedArray) {
			t.Fatalf("Test Failed. Expected %+v Got %+v", tc.ExpectedArray, tc.PassedArray)
		}
	}
}

func isEqual(s1 []string, s2 []string) bool {
	for i, s1 := range s1 {
		if s1 != s2[i] {
			return false
		}
	}
	return true
}
