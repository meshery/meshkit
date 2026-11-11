package kubernetes

import (
	"fmt"
	"time"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/release"
)

const (
	// DefaultPendingReleaseTimeout is how long, by default, the latest revision of a release may
	// stay pending-install, pending-upgrade or pending-rollback before an INSTALL with
	// UpgradeIfInstalled treats the operation that left it there as abandoned and recovers the
	// release. It is twice Helm's default operation timeout (5m), so an operation that is still
	// running elsewhere with Helm's defaults is never mistaken for an abandoned one.
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
	// marked failed before the upgrade, which Helm then performs from the last deployed revision
	// or, when none was ever deployed, from the abandoned one.
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
//     operation that fails, and the release is upgraded. This neither deletes anything, as
//     uninstalling and reinstalling would, nor takes a second operation, as rolling back and then
//     upgrading would. The revision stays in the history with a description saying why: failed,
//     or superseded when no revision was ever deployed, since Helm then upgrades from it. A dry
//     run never changes the release record, so it returns ErrHelmReleasePending.
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
		if pendingFor < timeout || cfg.DryRun {
			return helmReleasePlan{}, ErrHelmReleasePending(cfg.ReleaseName, cfg.Namespace, status.String(), latest.Version, pendingFor, timeout, cfg.DryRun)
		}
		return helmReleasePlan{action: UPGRADE, abandoned: latest, pendingFor: pendingFor}, nil
	default:
		return helmReleasePlan{}, ErrHelmReleaseState(cfg.ReleaseName, cfg.Namespace, status.String(), latest.Version)
	}
}

// recoverAbandonedRelease marks the revision that an abandoned operation left pending as failed,
// so that the upgrade that follows can run: Helm refuses to upgrade a release whose latest
// revision is pending. Nothing happens when the plan has no abandoned revision.
//
// The write is unconditional, as all of Helm's release record updates are. The revision has been
// pending for at least the pending timeout, so another process finishing it at the same moment is
// not a realistic race, and the in-process release lock rules it out within one process.
func recoverAbandonedRelease(actionConfig *action.Configuration, cfg ApplyHelmChartConfig, plan helmReleasePlan) error {
	rel := plan.abandoned
	if rel == nil {
		return nil
	}
	status := rel.Info.Status
	pendingFor := plan.pendingFor.Round(time.Second)
	cfg.Logger("helm release %q in namespace %q has been %s at revision %d for %s, longer than the %s pending timeout: marking the revision failed and upgrading the release",
		cfg.ReleaseName, cfg.Namespace, status, rel.Version, pendingFor, pendingReleaseTimeout(cfg))
	rel.SetStatus(release.StatusFailed, fmt.Sprintf("Marked failed by MeshKit: the %s operation was abandoned, still pending after %s", status, pendingFor))
	if err := actionConfig.Releases.Update(rel); err != nil {
		return ErrHelmReleaseStorage(cfg.ReleaseName, cfg.Namespace, err)
	}
	return nil
}

// pendingReleaseTimeout returns the configured pending timeout, or DefaultPendingReleaseTimeout
// when none is set.
func pendingReleaseTimeout(cfg ApplyHelmChartConfig) time.Duration {
	if cfg.PendingReleaseTimeout > 0 {
		return cfg.PendingReleaseTimeout
	}
	return DefaultPendingReleaseTimeout
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
