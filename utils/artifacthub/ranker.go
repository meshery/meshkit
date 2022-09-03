package artifacthub

import "sort"

var RankingParameterWeightage = map[string]int{
	"official":          5,
	"verifiedPublisher": 10,
}

func GetPackageScore(pkg AhPackage) int {
	score := 1
	if pkg.VerifiedPublisher {
		score = score + RankingParameterWeightage["verifiedPublisher"]
	}
	if pkg.Official {
		score = score + RankingParameterWeightage["official"]
	}
	return score
}

func SortPackagesWithScore(pkgs []AhPackage) []AhPackage {
	sort.SliceStable(pkgs, func(i, j int) bool {
		return GetPackageScore(pkgs[j]) < GetPackageScore(pkgs[i])
	})
	return pkgs
}
