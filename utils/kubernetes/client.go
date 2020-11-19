package kubernetes

func SetContext() error {
	clientConfig, err := clientcmd.Load(kubeconfig)
	if err != nil {
		return
	}
}
