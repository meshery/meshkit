package artifacthub

var priorityRepos = map[string]bool{"prometheus-community": true, "grafana": true} //Append ahrepos here whose components should be respected and should be used when encountered duplicates

// returns pkgs with sorted pkgs at the front
func SortOnVerified(pkgs []AhPackage) (verified []AhPackage, official []AhPackage, cncf []AhPackage, priority []AhPackage, unverified []AhPackage) {
	for _, pkg := range pkgs {
		if priorityRepos[pkg.Repository] {
			priority = append(priority, pkg)
			continue
		}
		if pkg.CNCF {
			cncf = append(cncf, pkg)
		} else if pkg.Official {
			official = append(official, pkg)
		} else if pkg.VerifiedPublisher {
			verified = append(verified, pkg)
		} else {
			unverified = append(unverified, pkg)
		}
	}
	return
}