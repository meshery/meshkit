package controllers

import (
	"github.com/meshery/meshkit/errors"
	mesherykube "github.com/meshery/meshkit/utils/kubernetes"
)

var (
	ErrGetControllerStatusCode          = "meshkit-11141"
	ErrDeployControllerCode             = "meshkit-11142"
	ErrGetControllerPublicEndpointCode  = "meshkit-11143"
	ErrGetControllerEndpointForPortCode = "meshkit-11319"

	// ErrUndeployControllerCode is the error raised when Helm could not uninstall the Meshery
	// Operator release, for a reason MeshKit's Helm client did not classify further.
	ErrUndeployControllerCode = "meshkit-11341"

	// ErrOperatorHelmReleaseStateCode is the error raised when the Meshery Operator Helm release
	// is pending or uninstalling, so that it cannot be installed or upgraded now.
	ErrOperatorHelmReleaseStateCode = "meshkit-11342"

	// ErrOperatorClusterAccessCode is the error raised when Meshery Server could not reach the
	// cluster, or read or write the Meshery Operator Helm release record in it.
	ErrOperatorClusterAccessCode = "meshkit-11343"

	// ErrOperatorChartFetchCode is the error raised when the Meshery Operator Helm chart could
	// not be located, downloaded or loaded.
	ErrOperatorChartFetchCode = "meshkit-11344"

	// ErrOperatorDeploymentInProgressCode is the error raised when a deployment of Meshery
	// Operator is refused because the operator's rollout has not finished.
	ErrOperatorDeploymentInProgressCode = "meshkit-11345"
)

// remInspectOperatorRelease answers every cause that Helm's own message explains best.
var remInspectOperatorRelease = errors.Remediation{
	ID:          "inspect-operator-release",
	Description: "Read Helm's message in the error details, and inspect the release with helm status meshery-operator -n meshery and helm history meshery-operator -n meshery.",
}

func ErrGetControllerStatus(err error) error {
	return errors.New(ErrGetControllerStatusCode, errors.Alert, []string{"Error getting the status of the meshery controller"}, []string{err.Error()}, []string{"Controller may not be healthy or not deployed"}, []string{"Make sure the controller is deployed and healthy"})
}

// ErrDeployController returns the error for a failed deployment of Meshery Operator.
//
// A failure that MeshKit's Helm client classified is reported as the error for that class,
// whose probable causes and remediations fit it: ErrOperatorHelmReleaseState for a release that
// is pending or uninstalling, ErrOperatorClusterAccess for a cluster that could not be reached or
// a release record that could not be read, and ErrOperatorChartFetch for a chart that could not
// be fetched. Anything else, typically Helm's install or upgrade itself failing, is reported with
// this error's own code.
func ErrDeployController(err error) error {
	if classified := classifyOperatorHelmError(err); classified != nil {
		return classified
	}
	e := errors.NewV3(ErrDeployControllerCode, errors.Alert,
		[]string{"Error deploying Meshery Operator"},
		[]string{},
		[]errors.Cause{
			{ID: "helm-install-or-upgrade-failed", Description: "Helm could not install or upgrade the meshery-operator release; the error details carry Helm's message.", Remediations: []errors.Remediation{
				remInspectOperatorRelease,
			}},
			{ID: "insufficient-cluster-permissions", Description: "The connection's identity is not allowed to create the resources of Meshery Operator, such as its namespace, CustomResourceDefinitions, ClusterRoles and Deployment.", Remediations: []errors.Remediation{
				{ID: "grant-operator-permissions", Description: "Connect the cluster with a kubeconfig whose identity can create namespaces, CustomResourceDefinitions, ClusterRoles, ClusterRoleBindings and Deployments, such as a cluster administrator."},
			}},
		})
	e.LongDescription = append(e.LongDescription, instanceDescription(err)...)
	return e
}

// ErrUndeployController returns the error for a failed removal of Meshery Operator. Like
// ErrDeployController, a failure that MeshKit's Helm client classified is reported as the error
// for that class.
func ErrUndeployController(err error) error {
	if classified := classifyOperatorHelmError(err); classified != nil {
		return classified
	}
	e := errors.NewV3(ErrUndeployControllerCode, errors.Alert,
		[]string{"Error undeploying Meshery Operator"},
		[]string{},
		[]errors.Cause{
			{ID: "helm-uninstall-failed", Description: "Helm could not uninstall the meshery-operator release; the error details carry Helm's message.", Remediations: []errors.Remediation{
				remInspectOperatorRelease,
			}},
			{ID: "operator-release-not-found", Description: "There is no meshery-operator release in the meshery namespace, for example because it was already uninstalled or Meshery Operator was installed without Helm.", Remediations: []errors.Remediation{
				{ID: "find-operator-release", Description: "List the releases with helm list -n meshery, and remove an operator installed without Helm with kubectl."},
			}},
		})
	e.LongDescription = append(e.LongDescription, instanceDescription(err)...)
	return e
}

// ErrOperatorHelmReleaseState is the error for a Meshery Operator Helm release that cannot be
// installed or upgraded now because its latest revision is pending or uninstalling. err is the
// classified error from MeshKit's Helm client.
func ErrOperatorHelmReleaseState(err error) error {
	e := errors.NewV3(ErrOperatorHelmReleaseStateCode, errors.Alert,
		[]string{"Meshery Operator Helm release is busy or was left unfinished"},
		[]string{},
		[]errors.Cause{
			{ID: "operator-release-operation-in-progress", Description: "Another install, upgrade or rollback of the meshery-operator release is still running, for example from another Meshery Server, or another connection to the same cluster.", Remediations: []errors.Remediation{
				{ID: "wait-for-operator-release-operation", Description: "Wait for that operation to finish, then deploy Meshery Operator again."},
			}},
			{ID: "operator-release-operation-interrupted", Description: "An earlier install, upgrade, rollback or uninstall of the meshery-operator release was interrupted, for example by a restart of Meshery Server, and left the release pending or uninstalling.", Remediations: []errors.Remediation{
				{ID: "await-pending-operator-release-recovery", Description: "Deploy again once the release has been pending for longer than the pending timeout (10 minutes by default): the abandoned revision is then marked failed and the release upgraded."},
				{ID: "recover-operator-release-manually", Description: "To recover now, inspect the release with helm history meshery-operator -n meshery, then run helm rollback meshery-operator <last deployed revision> -n meshery, or helm uninstall meshery-operator -n meshery if no revision was ever deployed or the release is uninstalling, and deploy again."},
			}},
		})
	e.LongDescription = append(e.LongDescription, instanceDescription(err)...)
	return e
}

// ErrOperatorClusterAccess is the error for a Meshery Operator deployment or removal that failed
// because Meshery Server could not reach the cluster, or read or write the Helm release record
// in it. err is the classified error from MeshKit's Helm client.
func ErrOperatorClusterAccess(err error) error {
	e := errors.NewV3(ErrOperatorClusterAccessCode, errors.Alert,
		[]string{"Meshery Server could not reach the cluster to manage Meshery Operator"},
		[]string{},
		[]errors.Cause{
			{ID: "cluster-unreachable", Description: "The Kubernetes API server of the cluster could not be reached from Meshery Server.", Remediations: []errors.Remediation{
				{ID: "check-cluster-connectivity", Description: "Verify that the cluster is running and that its API server is reachable from Meshery Server (network, DNS and firewall)."},
			}},
			{ID: "credentials-rejected", Description: "The cluster rejected the connection's credentials, for example because a token expired.", Remediations: []errors.Remediation{
				{ID: "refresh-connection-credentials", Description: "Refresh the kubeconfig of the connection and connect the cluster again."},
			}},
			{ID: "operator-release-record-forbidden", Description: "The connection's identity is not allowed to read or write the Secrets in the meshery namespace in which Helm records the meshery-operator release.", Remediations: []errors.Remediation{
				{ID: "grant-operator-release-record-access", Description: "Grant the identity get, list, create, update and delete on Secrets in the meshery namespace."},
			}},
		})
	e.LongDescription = append(e.LongDescription, instanceDescription(err)...)
	return e
}

// ErrOperatorChartFetch is the error for a Meshery Operator deployment that failed because the
// operator's Helm chart could not be located, downloaded or loaded. Nothing was changed in the
// cluster. err is the classified error from MeshKit's Helm client.
func ErrOperatorChartFetch(err error) error {
	e := errors.NewV3(ErrOperatorChartFetchCode, errors.Alert,
		[]string{"Could not fetch the Meshery Operator Helm chart"},
		[]string{},
		[]errors.Cause{
			{ID: "operator-chart-repository-unreachable", Description: "Meshery Server could not reach the Meshery Operator chart repository.", Remediations: []errors.Remediation{
				{ID: "check-operator-chart-repository-access", Description: "Verify that Meshery Server can reach the chart repository (network egress, proxy and DNS)."},
			}},
			{ID: "operator-chart-version-not-published", Description: "The meshery-operator chart version that Meshery Server asked for is not published in the chart repository.", Remediations: []errors.Remediation{
				{ID: "check-operator-chart-version", Description: "Verify that the requested meshery-operator chart version is listed in the repository's index.yaml, or configure a published version."},
			}},
		})
	e.LongDescription = append(e.LongDescription, instanceDescription(err)...)
	return e
}

// ErrOperatorDeploymentInProgress is the error for a deployment of Meshery Operator that was not
// started because the rollout of the operator's Deployment has not finished.
func ErrOperatorDeploymentInProgress() error {
	return errors.NewV3(ErrOperatorDeploymentInProgressCode, errors.Alert,
		[]string{"Meshery Operator is already being deployed"},
		[]string{"The meshery-operator Deployment in the meshery namespace is still rolling out, so a new deployment was not started."},
		[]errors.Cause{
			{ID: "operator-rollout-in-progress", Description: "The meshery-operator Deployment exists and its rollout has not completed yet.", Remediations: []errors.Remediation{
				{ID: "wait-for-operator-rollout", Description: "Wait for the rollout to finish, which kubectl rollout status deployment/meshery-operator -n meshery reports, then deploy again."},
			}},
			{ID: "operator-rollout-stuck", Description: "The rollout cannot complete, for example because a pod cannot pull its image or keeps crashing.", Remediations: []errors.Remediation{
				{ID: "inspect-operator-pods", Description: "Inspect the pods with kubectl describe pod -n meshery -l app=meshery-operator and kubectl logs deploy/meshery-operator -n meshery -c manager, and fix what they report."},
			}},
		})
}

func ErrGetControllerPublicEndpoint(err error) error {
	return errors.New(ErrGetControllerPublicEndpointCode, errors.Alert, []string{"Could not get the public endpoint of the controller"}, []string{err.Error()}, []string{"Client configuration may not be valid"}, []string{"Make sure the client configuration is valid"})
}

func ErrGetControllerEndpointForPort(err error) error {
	return errors.New(ErrGetControllerEndpointForPortCode, errors.Alert, []string{"Could not get endpoint for port of the controller"}, []string{err.Error()}, []string{"Client configuration may not be valid"}, []string{"Make sure the client configuration is valid"})
}

// classifyOperatorHelmError returns the error for the class of Helm failure that err is, or nil
// when MeshKit's Helm client did not classify it.
func classifyOperatorHelmError(err error) error {
	switch errors.GetCode(err) {
	case mesherykube.ErrHelmReleasePendingCode, mesherykube.ErrHelmReleaseStateCode:
		return ErrOperatorHelmReleaseState(err)
	case mesherykube.ErrHelmClusterUnreachableCode, mesherykube.ErrHelmReleaseStorageCode:
		return ErrOperatorClusterAccess(err)
	case mesherykube.ErrHelmChartFetchCode:
		return ErrOperatorChartFetch(err)
	}
	return nil
}

// instanceDescription returns the lines that describe this occurrence of err, for the long
// description of the error that reports it. For a MeshKit error that is its own long
// description, without the probable cause and remediation that its Error() appends, which
// belong to the inner error and would sit beside the reporting error's own; for any other error
// it is the message.
func instanceDescription(err error) []string {
	if err == nil {
		return nil
	}
	if e, ok := errors.AsErrorV2(err); ok {
		if lines := errors.NonEmptyLines(e.LongDescription); len(lines) > 0 {
			return lines
		}
	}
	return []string{err.Error()}
}
