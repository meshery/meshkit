# The Utils Package 
 The Utils Package is the Central component of  Meshkit. This package encompases different packages that offers a simplified and tailored experience for developers working with Kubernetes and Helm. Developers can easily interact with Kubernetes and Helm without relying on the command line versions by utilizing higher-level functions from the utils package. 
 Here is a brief description of some of the packages embedded in Utils:
## [Broadcast](https://github.com/meshery/meshkit/blob/master/utils/broadcast)
  The BroadCast package provides a simple and concurrent way to implement a broadcast channel, where messages can be submitted and multiple subscribers can register to receive those messages. It allows for decoupling the provider and subscribers and facilitates pubsub communication between components in a system.
## [Component](https://github.com/meshery/meshkit/tree/master/utils/component) 
  The Component Package genarates a component definition struct  which may contain various fields that provide information about the component, such as the component kind, API version, display name, schema, and metadata based on a custom CRD. The Component package also Extracts the JSON schema of the CRD using the provided CUE path configuration. 
## [Kuberentes](https://github.com/meshery/meshkit/tree/master/utils/kubernetes)
  The kubernetes package provides functionality for working with Kubernetes clusters .The package defines a <code>Client</code> that encapsulates the necessary components for interacting with the Kubernetes API server.
  This package contains certain packages such as describe, expose, Kompose, manifests  and walker for interacting with the kubernetes Api Server.
  Below are descriptions of each package: 
  ##### [Describe](https://github.com/meshery/meshkit/tree/master/utils/kubernetes/describe) 
    Describe Package  is a comprehensive and user-friendly solution for describing Kubernetes objects through the Kubernetes API. With its rich set of functionalities, it contains components for creating and initiaizing a meshclient, allowing users to retrieve information about various Kubernetes resources such as pods, deployments, jobs, services, and more to interact with the kubernetes Api.
    Overall, the describe package provides a convenient way to retrieve detailed information about Kubernetes resources in a standardized format.  
  ##### [Expose](https://github.com/meshery/meshkit/tree/master/utils/kubernetes/expose) 
  The Expose Package provides functionality for exposing Kubernetes resources as services.
  - The package imports various packages from the standard library and external dependencies. 
  - It contains fields for specifying the service type, load balancer IP, cluster IP, namespace,  session affinity, name, annotations, and a logger. 
  - The <code>Expose()</code> is the main function of the package this takes a Kubernetes clientset, REST config, Config object, and a list of resources to expose.
  - It uses a Traverser to iterate over the resources and generate the corresponding services.
  Overall, this package provides a way to expose Kubernetes resources as services with customizable configurations.
  ##### [Kompose](https://github.com/meshery/meshkit/tree/master/utils/kubernetes/kompose)
  Kompose Package provides functionality for working with Docker Compose files and converting them to Kubernetes manifests.
  It provides the following features:
  - Validation: The package can validate a Docker Compose file against a provided JSON schema. It ensures that the file adheres to the specified structure and format.
  - Conversion: The package can convert a validated Docker Compose file into Kubernetes manifests. It transforms the services, volumes, and other components defined in the Compose file into their equivalent representations in the Kubernetes ecosystem.
  - Compatibility Check: The package checks the compatibility of the Docker Compose file version with the "kompose" tool. It verifies if the version exceeds a certain limit and throws an error if it does.
  - Formatting: The package performs formatting operations on the Docker Compose and converted Kubernetes manifest files to ensure compatibility and consistency.
  Overall, the kompose package aims to simplify the process of migrating from Docker Compose to Kubernetes by providing validation, conversion, and compatibility checking capabilities.