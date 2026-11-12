package kubernetes

import (
	"errors"
	"fmt"
	"time"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"
)

const (
	// DefaultPendingReleaseTimeout is how long, by default, the latest revision of a release may
	// stay pending-install, pending-upgrade or pending-rollback before an INSTALL with
	// UpgradeIfInstalled treats the operation that left it there as abandoned and recovers the
	// release. It is twice Helm's default operation timeout (5m), so an operation that is still
	// running elsewhere with Helm's defaults is never mistaken for an abandoned one. MeshKit's own
	// operations do not wait for resources and finish well within it.
	DefaultPendingReleaseTimeout = 10 * time.Minute

	// DefaultHelmMaxHistory is the number of revisions an UPGRADE keeps for a release by default,
	// the same default as the Helm CLI's --history-max. Without a limit every upgrade adds a
	// Secret to the release's namespace forever.
	DefaultHelmMaxHistory = 10
)

// helmReleasePlan is what one ApplyHelmChart call does to a release.
type helmReleasePlan struct {
	action HelmChartAction
	// replace installs over a release that was uninstalled with its history kept.
	replace bool
	// abandoned is the latest revision when an operation that never finished left it pending. It is
	// marked failed before the action runs: an upgrade from the last deployed revision, or, when no
	// revision was ever deployed, the install again (with replace).
	abandoned *release.Release
	// pendingFor is how long abandoned had been pending when the plan was made.
	pendingFor time.Duration
}

// planInstallOrUpgrade decides how an INSTALL with UpgradeIfInstalled brings the release to the
// requested chart, from the latest revision of the release in the namespace the action
// configuration is bound to:
//
//   - none: install.
//   - deployed, failed or superseded: upgrade. Helm upgrades a failed or superseded release from
//     its last deployed revision, or from the failed one when none was ever deployed.
//   - uninstalled, with its history kept: install with Replace, which Helm allows over an
//     uninstalled release and which continues its revision numbers.
//   - pending-install, pending-upgrade or pending-rollback for less than the pending timeout:
//     ErrHelmReleasePending, because the operation that left it there may still be running.
//   - pending for at least the pending timeout: the operation is treated as abandoned (the process
//     running it was restarted, say). The revision is marked failed, exactly as Helm records an
//     operation that fails, and stays in the history with a description saying why. An
//     interrupted upgrade or rollback is then followed by an upgrade, and so is an interrupted
//     install over a release that has a deployed revision. An interrupted install with no deployed
//     revision (a first install, or one over an uninstalled release) runs again with Replace,
//     which Helm allows over a failed release: it adopts the resources the interrupted install
//     created, and it runs the chart's install hooks and renders .Release.IsInstall as an
//     install, which an upgrade would not. Neither path deletes anything, as uninstalling and
//     reinstalling would, or takes a second operation, as rolling back and then upgrading would.
//     A dry run never changes the release record, so it returns ErrHelmReleasePending.
//   - pending, with a negative PendingReleaseTimeout: ErrHelmReleasePending however old it is.
//     Helm records no owner or heartbeat for an operation, so elapsed time is the only sign that
//     one was abandoned; a caller whose releases are also operated by tools that run longer than
//     the timeout can raise it, or turn the recovery off this way.
//   - uninstalling, or a status this code does not know: ErrHelmReleaseState. Finishing an
//     uninstall deletes the release's resources, which is not something an install should do on
//     its own, so it is left to an operator.
//
// now is the current time, against which the age of a pending revision is measured.
func planInstallOrUpgrade(actionConfig *action.Configuration, cfg ApplyHelmChartConfig, now time.Time) (helmReleasePlan, error) {
	latest, err := latestRelease(actionConfig, cfg.ReleaseName)
	if err != nil {
		return helmReleasePlan{}, ErrHelmReleaseStorage(cfg.ReleaseName, cfg.Namespace, err)
	}
	if latest == nil {
		return helmReleasePlan{action: INSTALL}, nil
	}

	status := latest.Info.Status
	switch {
	case status == release.StatusDeployed, status == release.StatusFailed, status == release.StatusSuperseded:
		return helmReleasePlan{action: UPGRADE}, nil
	case status == release.StatusUninstalled:
		return helmReleasePlan{action: INSTALL, replace: true}, nil
	case status.IsPending():
		// LastDeployed is when the pending operation started. A zero value reads as long ago, and a
		// value in the future (clock skew between hosts) as just now, which errs towards waiting.
		pendingFor := now.Sub(latest.Info.LastDeployed.Time)
		timeout := pendingReleaseTimeout(cfg)
		if timeout < 0 || pendingFor < timeout || cfg.DryRun {
			return helmReleasePlan{}, ErrHelmReleasePending(cfg.ReleaseName, cfg.Namespace, status.String(), latest.Version, pendingFor, timeout, cfg.DryRun)
		}
		// Only an interrupted install is installed again. An interrupted upgrade or rollback stays
		// on the upgrade path even when no revision is deployed now: Helm supersedes the previous
		// revision before it records the new one as deployed, so a crash in between leaves
		// [superseded, pending-upgrade] for a release that was deployed.
		if status == release.StatusPendingInstall {
			deployed, err := hasDeployedRevision(actionConfig, cfg.ReleaseName)
			if err != nil {
				return helmReleasePlan{}, ErrHelmReleaseStorage(cfg.ReleaseName, cfg.Namespace, err)
			}
			if !deployed {
				return helmReleasePlan{action: INSTALL, replace: true, abandoned: latest, pendingFor: pendingFor}, nil
			}
		}
		return helmReleasePlan{action: UPGRADE, abandoned: latest, pendingFor: pendingFor}, nil
	default:
		return helmReleasePlan{}, ErrHelmReleaseState(cfg.ReleaseName, cfg.Namespace, status.String(), latest.Version)
	}
}

// recoverAbandonedRelease marks the revision that an abandoned operation left pending as failed,
// so that the action that follows can run: Helm refuses to upgrade a release whose latest
// revision is pending, and to install over one unless it is failed or uninstalled. Nothing happens
// when the plan has no abandoned revision.
//
// The write is unconditional, as all of Helm's release record updates are. Within one process the
// release lock rules out a concurrent operation; across processes, the pending timeout is the only
// guard (see planInstallOrUpgrade).
func recoverAbandonedRelease(actionConfig *action.Configuration, cfg ApplyHelmChartConfig, plan helmReleasePlan) error {
	if plan.abandoned == nil {
		return nil
	}
	// Change a copy, so that a failed write leaves nothing changed, even with a storage driver that
	// hands out the stored record itself (the in-memory one does).
	rel := *plan.abandoned
	info := *rel.Info
	rel.Info = &info
	status := info.Status
	pendingFor := plan.pendingFor.Round(time.Second)
	next := "upgrading the release"
	if plan.action == INSTALL {
		next = "installing it again, since no revision was ever deployed"
	}
	cfg.Logger("helm release %q in namespace %q has been %s at revision %d for %s, longer than the %s pending timeout: marking the revision failed and %s",
		cfg.ReleaseName, cfg.Namespace, status, rel.Version, pendingFor, pendingReleaseTimeout(cfg), next)
	rel.SetStatus(release.StatusFailed, fmt.Sprintf("Marked failed by MeshKit: the %s operation was abandoned, still pending after %s", status, pendingFor))
	if err := actionConfig.Releases.Update(&rel); err != nil {
		return ErrHelmReleaseStorage(cfg.ReleaseName, cfg.Namespace, err)
	}
	return nil
}

// pendingReleaseTimeout returns the configured pending timeout, DefaultPendingReleaseTimeout when
// none is set, and a negative value when the recovery of abandoned operations is turned off.
func pendingReleaseTimeout(cfg ApplyHelmChartConfig) time.Duration {
	if cfg.PendingReleaseTimeout != 0 {
		return cfg.PendingReleaseTimeout
	}
	return DefaultPendingReleaseTimeout
}

// hasDeployedRevision reports whether any revision of the release is deployed.
func hasDeployedRevision(actionConfig *action.Configuration, name string) (bool, error) {
	if _, err := actionConfig.Releases.Deployed(name); err != nil {
		if errors.Is(err, driver.ErrNoDeployedReleases) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// helmMaxHistory returns the history limit for Helm's Upgrade action: DefaultHelmMaxHistory when
// none is configured, and 0 (Helm's "keep every revision") when the configured value is negative.
func helmMaxHistory(cfg ApplyHelmChartConfig) int {
	switch {
	case cfg.MaxHistory > 0:
		return cfg.MaxHistory
	case cfg.MaxHistory < 0:
		return 0
	default:
		return DefaultHelmMaxHistory
	}
}
