package utils

import (
	"sort"
	"strconv"
	"strings"
)

type dottedStrings []string

func (d dottedStrings) Less(i, j int) bool {
	si := cleanup(d[i])
	sj := cleanup(d[j])
	siarr := strings.Split(si, ".")
	sjarr := strings.Split(sj, ".")
	var n int
	if len(siarr) < len(sjarr) {
		n = len(siarr)
	} else {
		n = len(sjarr)
	}
	// While comparing two strings, the comparison is made upto the size of smaller string
	for i := 0; i < n; i++ {
		//We can be sure that siarr and sjarr are numeric string array, hence Atoi can be safely used
		p, _ := strconv.Atoi(siarr[i])
		q, _ := strconv.Atoi(sjarr[i])
		if p < q {
			return true
		}
		if q < p {
			return false
		}
	}

	//If both strings are equal to the len of smallest string, consider the smaller-length string to be greater in value.
	// This is to make sure that , while comparing strings like, 1.0.0 and 1.0.0-someprefix, 1.0.0 is considered greater
	if len(siarr) < len(sjarr) {
		return false
	}
	return true
}

func (d dottedStrings) Len() int {
	return len(d)
}
func (d dottedStrings) Swap(i, j int) {
	d[i], d[j] = d[j], d[i]
}
func cleanup(s string) string {
	s = strings.Replace(s, "alpha", ".0", -1)
	s = strings.Replace(s, "beta", ".1", -1)
	s = strings.Replace(s, "rc", ".2", -1)
	s = strings.Replace(s, "stable", ".3", -1)
	s1 := ""
	for _, s := range s {
		if (s >= 48 && s <= 57) || s == 46 {
			s1 += string(s)
		}
	}
	return s1
}

//SortDottedStringsByDigits takes version-like dot seperated digits in string format and returns them in sorted normalized form.
// Takes [v1.4.3,0.9.3,v0.0.0]=> returns [v0.0.0,0.9.3,v1.4.3]
// This function ignores all letters except for:
// - numeric digits
// - alpha, beta, rc, stable
func SortDottedStringsByDigits(s []string) []string {
	s1 := dottedStrings(s)
	sort.Sort(s1)
	return s1
}
