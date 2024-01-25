package artifacthub

import "sort"

var RankingParameterWeightage = map[string]int{
	"official":          5,
	"verifiedPublisher": 10,
}

func getPackageScore(pkg AhPackage) int {
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
		return getPackageScore(pkgs[j]) < getPackageScore(pkgs[i])
	})
	return pkgs
}

func FilterPackageWithGivenSourceURL(pkgs []AhPackage, url string) []AhPackage {
	for _, pkg := range pkgs {
		if pkg.ChartUrl == url {
			return []AhPackage{pkg}
		}
	}
	return []AhPackage{}
}
