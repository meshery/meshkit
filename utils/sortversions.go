package utils

import (
	"sort"
	"strconv"
	"strings"
)

type dottedStrings []string

func (d dottedStrings) Less(i, j int) bool {
	si := d[i]
	sj := d[j]
	si = cleanup(si)
	sj = cleanup(sj)
	siarr := strings.Split(si, ".")
	sjarr := strings.Split(sj, ".")
	var n int
	if len(siarr) < len(sjarr) {
		n = len(siarr)
		sjarr = sjarr[0:n]
	} else {
		n = len(sjarr)
		siarr = siarr[0:n]
	}
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
	return false
}

func (d dottedStrings) Len() int {
	return len(d)
}
func (d dottedStrings) Swap(i, j int) {
	d[i], d[j] = d[j], d[i]
}
func cleanup(s string) string {
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
func SortDottedStringsByDigits(s []string) []string {
	s1 := dottedStrings(s)
	sort.Sort(s1)
	return s1
}
